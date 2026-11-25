package keeper

import (
	"context"

	errorsmod "cosmossdk.io/errors"
	"cosmossdk.io/math"
	storetypes "cosmossdk.io/store/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"

	"nira-chain/x/policy/types"
)

// Voting-power cap (ADR-0004).
//
// x/staking calls BeforeDelegationCreated / BeforeDelegationSharesModified
// before a delegation changes, and AfterDelegationModified afterwards. In
// Delegate the validator's tokens have already increased when the After hook
// runs; in Unbond they have not decreased yet. We record the validator's tokens
// in the Before hook and only enforce
// the cap when the tokens grew. Undelegation is therefore never blocked
// (INV-STAKE-CAP-2), and every path that increases stake — Cosmos tx, authz,
// redelegation, EVM staking precompile — goes through Delegate and is checked
// (INV-STAKE-CAP-1).
//
// The scratch value lives in the module's regular KV store, not a transient
// store: the EVM StateDB runs precompiles on a cache built only from KV store
// keys, so a transient store is unreachable from the staking precompile. Each
// entry is written in a Before hook and deleted in the matching After /
// BeforeDelegationRemoved hook of the same staking call, so nothing persists
// past the transaction (INV-STAKE-CAP-3).

func preTokensKey(valAddr sdk.ValAddress) []byte {
	return append(append([]byte{}, types.PreDelegationTokensPrefix...), valAddr...)
}

func (k Keeper) recordPreTokens(ctx context.Context, valAddr sdk.ValAddress) error {
	val, err := k.stakingKeeper.GetValidator(ctx, valAddr)
	if err != nil {
		return err
	}
	bz, err := val.Tokens.Marshal()
	if err != nil {
		return err
	}
	return k.storeService.OpenKVStore(ctx).Set(preTokensKey(valAddr), bz)
}

func (k Keeper) popPreTokens(ctx context.Context, valAddr sdk.ValAddress) (math.Int, bool, error) {
	store := k.storeService.OpenKVStore(ctx)
	key := preTokensKey(valAddr)
	bz, err := store.Get(key)
	if err != nil || bz == nil {
		return math.Int{}, false, err
	}
	if err := store.Delete(key); err != nil {
		return math.Int{}, false, err
	}
	var tokens math.Int
	if err := tokens.Unmarshal(bz); err != nil {
		return math.Int{}, false, err
	}
	return tokens, true, nil
}

// CheckValidatorPowerCap returns ErrValidatorPowerCap if the validator's share
// of total bonded stake exceeds the cap and the cap is active.
func (k Keeper) CheckValidatorPowerCap(ctx context.Context, val stakingtypes.Validator) error {
	params, err := k.Params.Get(ctx)
	if err != nil {
		return err
	}
	bonded, err := k.LastBondedState(ctx)
	if err != nil {
		return err
	}
	total := bonded.TotalBonded
	if !val.IsBonded() {
		// The validator's stake is not in the bonded pool yet; count it as if
		// it joins the active set.
		total = total.Add(val.Tokens)
	}
	if total.LT(params.CapActivationBonded) ||
		bonded.NumValidators < uint64(params.CapActivationMinValidators) {
		return nil
	}
	// tokens * 10000 > bps * total  ⇔  share > cap (exact integer arithmetic)
	lhs := val.Tokens.MulRaw(int64(types.BpsDenominator))
	rhs := total.MulRaw(int64(params.MaxValidatorPowerBps))
	if lhs.GT(rhs) {
		return errorsmod.Wrapf(types.ErrValidatorPowerCap,
			"validator %s would hold %s of %s bonded tokens (cap %d bps)",
			val.OperatorAddress, val.Tokens, total, params.MaxValidatorPowerBps)
	}
	return nil
}

// BondedState is a snapshot of the bonded validator set.
type BondedState struct {
	TotalBonded   math.Int
	NumValidators uint64
}

// LastBondedState reads total bonded tokens and the number of validators in
// the last committed active set (bounded by max_validators).
func (k Keeper) LastBondedState(ctx context.Context) (BondedState, error) {
	total, err := k.stakingKeeper.TotalBondedTokens(ctx)
	if err != nil {
		return BondedState{}, err
	}
	vals, err := k.stakingKeeper.GetLastValidators(ctx)
	if err != nil {
		return BondedState{}, err
	}
	return BondedState{TotalBonded: total, NumValidators: uint64(len(vals))}, nil
}

// PreDelegationEntries counts scratch entries (must be 0 between transactions;
// used by tests and the module invariant).
func (k Keeper) PreDelegationEntries(ctx context.Context) (int, error) {
	it, err := k.storeService.OpenKVStore(ctx).Iterator(
		types.PreDelegationTokensPrefix, storetypes.PrefixEndBytes(types.PreDelegationTokensPrefix))
	if err != nil {
		return 0, err
	}
	defer it.Close()
	n := 0
	for ; it.Valid(); it.Next() {
		n++
	}
	return n, nil
}

// CheckMinSelfBond rejects a validator whose min_self_delegation is below
// P-STAKE-min_self_bond. The SDK guarantees the initial self-bond ≥
// min_self_delegation, that min_self_delegation can only increase, and jails a
// validator whose self-delegation falls below it — so the floor holds for the
// validator's whole life (INV-STAKE-SELF-1).
func (k Keeper) CheckMinSelfBond(ctx context.Context, valAddr sdk.ValAddress) error {
	params, err := k.Params.Get(ctx)
	if err != nil {
		return err
	}
	val, err := k.stakingKeeper.GetValidator(ctx, valAddr)
	if err != nil {
		return err
	}
	if val.MinSelfDelegation.LT(params.MinSelfBond) {
		return errorsmod.Wrapf(types.ErrMinSelfBond, "min_self_delegation %s < required %s (base units)",
			val.MinSelfDelegation, params.MinSelfBond)
	}
	return nil
}
