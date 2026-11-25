package keeper

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"cosmossdk.io/collections"
	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"

	"nira-chain/x/community/types"
)

// Block hooks (docs/spec/community.md §8).
//
// INV-COMM-8: the community layer is off the finality path, so these hooks
// never return an error. Each step runs in a cache context; a failing step is
// discarded, reported as a `comm_hook_error` event and logged, and the chain
// continues. Funds involved stay in the module account.

// safe runs fn in a cache context and commits only on success.
func (k Keeper) safe(ctx context.Context, step string, fn func(ctx context.Context) error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	cacheCtx, write := sdkCtx.CacheContext()
	if err := fn(cacheCtx); err != nil {
		sdkCtx.Logger().Error("community hook step failed", "step", step, "err", err)
		sdkCtx.EventManager().EmitEvent(sdk.NewEvent("comm_hook_error",
			sdk.NewAttribute("step", step), sdk.NewAttribute("error", err.Error())))
		return
	}
	write()
}

// BeginBlock: app-hash history → reward funding → epoch transition.
// Must run after x/mint and before x/distribution.
func (k Keeper) BeginBlock(ctx context.Context) error {
	params, err := k.Params.Get(ctx)
	if err != nil {
		sdk.UnwrapSDKContext(ctx).Logger().Error("community params missing", "err", err)
		return nil
	}
	k.safe(ctx, "apphash", func(c context.Context) error { return k.recordAppHash(c, params) })
	k.safe(ctx, "funding", func(c context.Context) error { return k.fundRewards(c, params) })
	k.safe(ctx, "epoch", func(c context.Context) error { return k.maybeAdvanceEpoch(c, params) })
	return nil
}

// EndBlock: release matured unbondings and expire alerts, each bounded.
func (k Keeper) EndBlock(ctx context.Context) error {
	params, err := k.Params.Get(ctx)
	if err != nil {
		return nil
	}
	k.safe(ctx, "unbonding", func(c context.Context) error { return k.releaseUnbonded(c, params) })
	k.safe(ctx, "alerts", func(c context.Context) error { return k.expireAlerts(c, params) })
	return nil
}

// recordAppHash stores AppHash[height-1] (the root that state proofs at
// version height-1 verify against) and prunes at most two entries older than
// apphash_window + epoch_blocks (INV-COMM-7: ring stays bounded and
// self-heals after parameter changes).
func (k Keeper) recordAppHash(ctx context.Context, p types.Params) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	h := sdkCtx.BlockHeight()
	if h <= 1 {
		return nil
	}
	appHash := sdkCtx.HeaderInfo().AppHash
	if len(appHash) == 0 {
		return nil
	}
	if err := k.AppHashes.Set(ctx, h-1, appHash); err != nil {
		return err
	}
	cutoff := h - 1 - int64(p.ApphashWindow) - int64(p.EpochBlocks)
	for i := 0; i < 2; i++ {
		it, err := k.AppHashes.Iterate(ctx, nil)
		if err != nil {
			return err
		}
		if !it.Valid() {
			it.Close()
			return nil
		}
		oldest, err := it.Key()
		it.Close()
		if err != nil {
			return err
		}
		if oldest >= cutoff {
			return nil
		}
		if err := k.AppHashes.Remove(ctx, oldest); err != nil {
			return err
		}
	}
	return nil
}

// fundRewards moves reward_share_bps of the fee collector's bond-denom balance
// (fees + this block's inflation) into the epoch reward pool.
func (k Keeper) fundRewards(ctx context.Context, p types.Params) error {
	if p.RewardShareBps == 0 {
		return nil
	}
	ep, err := k.Epoch.Get(ctx)
	if errors.Is(err, collections.ErrNotFound) {
		return nil // no epoch yet (first block): funded from the next block
	} else if err != nil {
		return err
	}
	fc := k.authKeeper.GetModuleAddress(authtypes.FeeCollectorName)
	bal := k.bankKeeper.GetBalance(ctx, fc, p.BondDenom).Amount
	amt := bal.MulRaw(int64(p.RewardShareBps)).QuoRaw(10_000) // floor
	if !amt.IsPositive() {
		return nil
	}
	if err := k.bankKeeper.SendCoinsFromModuleToModule(ctx, authtypes.FeeCollectorName, types.ModuleName,
		sdk.NewCoins(sdk.NewCoin(p.BondDenom, amt))); err != nil {
		return err
	}
	ep.Budget = ep.Budget.Add(amt)
	return k.Epoch.Set(ctx, ep)
}

// maybeAdvanceEpoch settles the current epoch and opens the next when due.
func (k Keeper) maybeAdvanceEpoch(ctx context.Context, p types.Params) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	h := sdkCtx.BlockHeight()
	ep, err := k.Epoch.Get(ctx)
	switch {
	case errors.Is(err, collections.ErrNotFound):
		ep = types.Epoch{Number: 0, Budget: math.ZeroInt(), EligibleBond: math.ZeroInt()}
	case err != nil:
		return err
	case h < ep.StartHeight+int64(p.EpochBlocks):
		return nil
	default:
		if err := k.settle(ctx, p, ep); err != nil {
			return err
		}
	}
	total, err := k.totalActiveBond(ctx)
	if err != nil {
		return err
	}
	next := types.Epoch{
		Number:       ep.Number + 1,
		StartHeight:  h,
		Seed:         types.NextSeed(ep.Seed, sdkCtx.HeaderInfo().Hash, ep.Number+1),
		EligibleBond: total,
		Budget:       math.ZeroInt(),
	}
	if err := k.Epoch.Set(ctx, next); err != nil {
		return err
	}
	sdkCtx.EventManager().EmitEvent(sdk.NewEvent("epoch_started",
		sdk.NewAttribute("epoch", strconv.FormatUint(next.Number, 10)),
		sdk.NewAttribute("start_height", strconv.FormatInt(h, 10)),
		sdk.NewAttribute("eligible_bond", total.String())))
	return nil
}

// settle pays epoch e's budget pro rata to proven units (floor), sends the
// remainder to the community pool (INV-COMM-1) and deletes epoch-scoped data.
// Work is bounded by max_proofs_per_epoch entries.
func (k Keeper) settle(ctx context.Context, p types.Params, ep types.Epoch) error {
	type work struct {
		op    []byte
		units uint32
	}
	var items []work
	total := uint64(0)
	rng := collections.NewPrefixedPairRange[uint64, []byte](ep.Number)
	err := k.EpochWork.Walk(ctx, rng, func(key collections.Pair[uint64, []byte], units uint32) (bool, error) {
		items = append(items, work{op: key.K2(), units: units})
		total += uint64(units)
		return false, nil
	})
	if err != nil {
		return err
	}
	paid := math.ZeroInt()
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	for _, w := range items {
		amt := ep.Budget.Mul(math.NewIntFromUint64(uint64(w.units))).Quo(math.NewIntFromUint64(total))
		if amt.IsPositive() {
			if err := k.bankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, w.op,
				sdk.NewCoins(sdk.NewCoin(p.BondDenom, amt))); err != nil {
				return err
			}
			paid = paid.Add(amt)
			if node, err := k.Nodes.Get(ctx, w.op); err == nil { // G-COMM-3 reward tracking
				node.RewardsTotal = orZero(node.RewardsTotal).Add(amt)
				if err := k.Nodes.Set(ctx, w.op, node); err != nil {
					return err
				}
			}
		}
		opStr, _ := k.addressCodec.BytesToString(w.op)
		sdkCtx.EventManager().EmitEvent(sdk.NewEvent("community_reward",
			sdk.NewAttribute("epoch", strconv.FormatUint(ep.Number, 10)),
			sdk.NewAttribute("operator", opStr),
			sdk.NewAttribute("units", strconv.FormatUint(uint64(w.units), 10)),
			sdk.NewAttribute("amount", amt.String())))
		if err := k.EpochWork.Remove(ctx, collections.Join(ep.Number, w.op)); err != nil {
			return err
		}
	}
	if paid.GT(ep.Budget) {
		return fmt.Errorf("INV-COMM-1 violated: paid %s > budget %s", paid, ep.Budget)
	}
	if rest := ep.Budget.Sub(paid); rest.IsPositive() {
		if err := k.distrKeeper.FundCommunityPool(ctx, sdk.NewCoins(sdk.NewCoin(p.BondDenom, rest)), k.ModuleAddress()); err != nil {
			return err
		}
	}
	// TaskDone records of the epoch (≤ max_proofs_per_epoch)
	trng := collections.NewPrefixedTripleRange[uint64, []byte, uint32](ep.Number)
	if err := k.TaskDone.Clear(ctx, trng); err != nil {
		return err
	}
	sdkCtx.EventManager().EmitEvent(sdk.NewEvent("epoch_settled",
		sdk.NewAttribute("epoch", strconv.FormatUint(ep.Number, 10)),
		sdk.NewAttribute("budget", ep.Budget.String()),
		sdk.NewAttribute("paid", paid.String()),
		sdk.NewAttribute("units", strconv.FormatUint(total, 10))))
	return nil
}

func (k Keeper) releaseUnbonded(ctx context.Context, p types.Params) error {
	h := sdk.UnwrapSDKContext(ctx).BlockHeight()
	var due []collections.Pair[int64, []byte]
	// Queue is ordered by completion height: stop at the first future entry.
	err := k.UnbondingQueue.Walk(ctx, nil, func(key collections.Pair[int64, []byte]) (bool, error) {
		if key.K1() > h {
			return true, nil
		}
		due = append(due, key)
		return len(due) >= int(p.MaxUnbondPerBlock), nil
	})
	if err != nil {
		return err
	}
	for _, key := range due {
		op := key.K2()
		node, err := k.Nodes.Get(ctx, op)
		if err == nil {
			if err := k.bankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, op,
				sdk.NewCoins(sdk.NewCoin(p.BondDenom, node.Bond))); err != nil {
				return err
			}
			if err := k.Nodes.Remove(ctx, op); err != nil {
				return err
			}
			sdk.UnwrapSDKContext(ctx).EventManager().EmitEvent(sdk.NewEvent("node_unbonded",
				sdk.NewAttribute("operator", node.Operator), sdk.NewAttribute("amount", node.Bond.String())))
		}
		if err := k.UnbondingQueue.Remove(ctx, key); err != nil {
			return err
		}
	}
	return nil
}

func (k Keeper) expireAlerts(ctx context.Context, p types.Params) error {
	h := sdk.UnwrapSDKContext(ctx).BlockHeight()
	var due []collections.Pair[int64, uint64]
	err := k.OpenAlerts.Walk(ctx, nil, func(key collections.Pair[int64, uint64]) (bool, error) {
		if key.K1() > h {
			return true, nil
		}
		due = append(due, key)
		return len(due) >= int(p.MaxUnbondPerBlock), nil
	})
	if err != nil {
		return err
	}
	for _, key := range due {
		a, err := k.Alerts.Get(ctx, key.K2())
		if err != nil {
			return err
		}
		reporter, err := k.addressCodec.StringToBytes(a.Reporter)
		if err != nil {
			return err
		}
		if err := k.bankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, reporter,
			sdk.NewCoins(sdk.NewCoin(p.BondDenom, a.Deposit))); err != nil {
			return err
		}
		a.Status = types.ALERT_STATUS_EXPIRED
		if err := k.closeAlert(ctx, a); err != nil {
			return err
		}
	}
	return nil
}
