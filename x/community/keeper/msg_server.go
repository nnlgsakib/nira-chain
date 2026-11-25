package keeper

import (
	"bytes"
	"context"
	"errors"
	"strconv"

	"cosmossdk.io/collections"
	errorsmod "cosmossdk.io/errors"
	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"nira-chain/x/community/types"
)

type msgServer struct {
	Keeper
}

// NewMsgServerImpl returns the MsgServer used by both the Cosmos router and
// the EVM precompile (ADR-0006), so both interfaces share one code path.
func NewMsgServerImpl(keeper Keeper) types.MsgServer {
	return &msgServer{Keeper: keeper}
}

var _ types.MsgServer = msgServer{}

func (k msgServer) operator(addr string) ([]byte, error) {
	op, err := k.addressCodec.StringToBytes(addr)
	if err != nil {
		return nil, errorsmod.Wrapf(types.ErrInvalidSigner, "invalid operator address: %s", err)
	}
	return op, nil
}

func validOnion(tip []byte, length uint32, p types.Params) error {
	if len(tip) != types.HashLen || bytes.Equal(tip, make([]byte, types.HashLen)) {
		return errorsmod.Wrap(types.ErrInvalidOnion, "onion tip must be 32 non-zero bytes")
	}
	if length < 1 || length > p.MaxOnionLength {
		return errorsmod.Wrapf(types.ErrInvalidOnion, "onion length must be in [1, %d]", p.MaxOnionLength)
	}
	return nil
}

// RegisterNode — spec §6.1.
func (k msgServer) RegisterNode(ctx context.Context, msg *types.MsgRegisterNode) (*types.MsgRegisterNodeResponse, error) {
	op, err := k.operator(msg.Operator) // C1
	if err != nil {
		return nil, err
	}
	p, err := k.Params.Get(ctx)
	if err != nil {
		return nil, err
	}
	if has, err := k.Nodes.Has(ctx, op); err != nil { // C2
		return nil, err
	} else if has {
		return nil, types.ErrNodeExists
	}
	if msg.Bond.Denom != p.BondDenom || msg.Bond.Amount.IsNil() || msg.Bond.Amount.LT(p.MinBond) { // C3
		return nil, errorsmod.Wrapf(types.ErrInsufficientBond, "need ≥ %s%s", p.MinBond, p.BondDenom)
	}
	if err := validOnion(msg.OnionTip, msg.OnionLength, p); err != nil { // C4
		return nil, err
	}
	if len(msg.Moniker) > 64 { // C5
		return nil, types.ErrInvalidMoniker
	}
	ep, err := k.currentEpoch(ctx)
	if err != nil {
		return nil, err
	}
	if err := k.bankKeeper.SendCoinsFromAccountToModule(ctx, op, types.ModuleName, sdk.NewCoins(msg.Bond)); err != nil { // E1
		return nil, err
	}
	node := types.Node{ // E2
		Operator:         msg.Operator,
		Moniker:          msg.Moniker,
		Bond:             msg.Bond.Amount,
		Status:           types.NODE_STATUS_ACTIVE,
		RegisteredEpoch:  ep.Number,
		OnionBaseEpoch:   ep.Number + 1,
		OnionAnchor:      msg.OnionTip,
		OnionAnchorEpoch: ep.Number,
		OnionLength:      msg.OnionLength,
		RewardsTotal:     math.ZeroInt(),
		SlashedTotal:     math.ZeroInt(),
	}
	if err := k.Nodes.Set(ctx, op, node); err != nil {
		return nil, err
	}
	total, err := k.totalActiveBond(ctx)
	if err != nil {
		return nil, err
	}
	if err := k.TotalActiveBond.Set(ctx, total.Add(msg.Bond.Amount)); err != nil { // E3
		return nil, err
	}
	sdk.UnwrapSDKContext(ctx).EventManager().EmitEvent(sdk.NewEvent("node_registered",
		sdk.NewAttribute("operator", msg.Operator),
		sdk.NewAttribute("bond", msg.Bond.String()),
		sdk.NewAttribute("base_epoch", strconv.FormatUint(node.OnionBaseEpoch, 10))))
	return &types.MsgRegisterNodeResponse{BaseEpoch: node.OnionBaseEpoch}, nil
}

// RotateOnion — spec §6.2.
func (k msgServer) RotateOnion(ctx context.Context, msg *types.MsgRotateOnion) (*types.MsgRotateOnionResponse, error) {
	op, err := k.operator(msg.Operator)
	if err != nil {
		return nil, err
	}
	p, err := k.Params.Get(ctx)
	if err != nil {
		return nil, err
	}
	node, err := k.node(ctx, op)
	if err != nil {
		return nil, err
	}
	if node.Status != types.NODE_STATUS_ACTIVE {
		return nil, types.ErrNodeNotActive
	}
	if err := validOnion(msg.OnionTip, msg.OnionLength, p); err != nil {
		return nil, err
	}
	ep, err := k.currentEpoch(ctx)
	if err != nil {
		return nil, err
	}
	node.OnionBaseEpoch = ep.Number + 1
	node.OnionAnchor = msg.OnionTip
	node.OnionAnchorEpoch = ep.Number
	node.OnionLength = msg.OnionLength
	if err := k.Nodes.Set(ctx, op, node); err != nil {
		return nil, err
	}
	sdk.UnwrapSDKContext(ctx).EventManager().EmitEvent(sdk.NewEvent("onion_rotated",
		sdk.NewAttribute("operator", msg.Operator),
		sdk.NewAttribute("base_epoch", strconv.FormatUint(node.OnionBaseEpoch, 10))))
	return &types.MsgRotateOnionResponse{BaseEpoch: node.OnionBaseEpoch}, nil
}

// SubmitTaskProof — spec §6.3, checks C1–C12 in order.
func (k msgServer) SubmitTaskProof(ctx context.Context, msg *types.MsgSubmitTaskProof) (*types.MsgSubmitTaskProofResponse, error) {
	op, err := k.operator(msg.Operator) // C1
	if err != nil {
		return nil, err
	}
	p, err := k.Params.Get(ctx)
	if err != nil {
		return nil, err
	}
	ep, err := k.currentEpoch(ctx)
	if err != nil {
		return nil, err
	}
	node, err := k.node(ctx, op)
	if err != nil {
		return nil, err
	}
	if node.Status != types.NODE_STATUS_ACTIVE || node.RegisteredEpoch >= ep.Number { // C2 (INV-COMM-3)
		return nil, types.ErrNodeNotEligible
	}
	if msg.Epoch != ep.Number { // C3
		return nil, errorsmod.Wrapf(types.ErrWrongEpoch, "current epoch %d", ep.Number)
	}
	if ep.Number < node.OnionBaseEpoch { // C4
		return nil, errorsmod.Wrap(types.ErrNodeNotEligible, "onion not yet valid")
	}
	revealed := false // C5
	if node.OnionAnchorEpoch == ep.Number {
		if !bytes.Equal(msg.Reveal, node.OnionAnchor) {
			return nil, errorsmod.Wrap(types.ErrInvalidReveal, "reveal differs from this epoch's revealed layer")
		}
	} else {
		d := types.RevealDistance(node.OnionAnchorEpoch, ep.Number)
		if d > uint64(p.MaxRevealDistance) {
			return nil, errorsmod.Wrapf(types.ErrInvalidReveal, "reveal distance %d > max %d: rotate the onion", d, p.MaxRevealDistance)
		}
		// Meter the hash chain before computing it (unmetered CPU otherwise).
		sdk.UnwrapSDKContext(ctx).GasMeter().ConsumeGas(types.RevealGasPerHash*d, "community onion reveal")
		if err := types.VerifyReveal(msg.Reveal, node.OnionAnchor, node.OnionAnchorEpoch, ep.Number,
			node.OnionBaseEpoch, node.OnionLength, p.MaxRevealDistance); err != nil {
			return nil, errorsmod.Wrap(types.ErrInvalidReveal, err.Error())
		}
		revealed = true
	}
	if !types.IsSelected(types.Ticket(ep.Seed, msg.Reveal, op), node.Bond, ep.EligibleBond, p.TargetCommittee) { // C6
		return nil, types.ErrNotSelected
	}
	taskKey := collections.Join3(ep.Number, op, msg.TaskIndex) // C7
	if msg.TaskIndex >= p.TasksPerSelection {
		return nil, types.ErrTaskDone
	}
	if done, err := k.TaskDone.Has(ctx, taskKey); err != nil {
		return nil, err
	} else if done {
		return nil, types.ErrTaskDone
	}
	if ep.Proofs >= p.MaxProofsPerEpoch { // C8
		return nil, types.ErrEpochFull
	}
	task, ok := types.DeriveTask(ep.Seed, op, msg.TaskIndex, ep.StartHeight, p.ApphashWindow, p.SampleStores) // C9
	if !ok || task.Store != msg.Store || task.Height != msg.Height {
		return nil, errorsmod.Wrapf(types.ErrWrongTask, "assigned store=%s height=%d", task.Store, task.Height)
	}
	appHash, err := k.AppHashes.Get(ctx, task.Height)
	if err != nil {
		return nil, errorsmod.Wrapf(types.ErrWrongTask, "no app hash for height %d", task.Height)
	}
	if len(msg.Proof) > int(p.MaxProofBytes) || len(msg.Value) > 4096 { // C10
		return nil, types.ErrProofTooLarge
	}
	sdk.UnwrapSDKContext(ctx).GasMeter().ConsumeGas(p.ProofGasPerByte*uint64(len(msg.Proof)), "community state proof") // C11
	if err := types.VerifyStateProof(appHash, task.Store, task.Key, msg.Value, msg.Proof); err != nil {                // C12
		return nil, errorsmod.Wrap(types.ErrInvalidProof, err.Error())
	}

	// Effects
	if revealed { // E1
		node.OnionAnchor = msg.Reveal
		node.OnionAnchorEpoch = ep.Number
	}
	node.UnitsTotal++
	if err := k.Nodes.Set(ctx, op, node); err != nil {
		return nil, err
	}
	if err := k.TaskDone.Set(ctx, taskKey); err != nil { // E2
		return nil, err
	}
	workKey := collections.Join(ep.Number, op)
	units, err := k.EpochWork.Get(ctx, workKey)
	if err != nil && !errors.Is(err, collections.ErrNotFound) {
		return nil, err
	}
	if err := k.EpochWork.Set(ctx, workKey, units+1); err != nil {
		return nil, err
	}
	ep.Proofs++
	if err := k.Epoch.Set(ctx, ep); err != nil {
		return nil, err
	}
	sdk.UnwrapSDKContext(ctx).EventManager().EmitEvent(sdk.NewEvent("task_proven",
		sdk.NewAttribute("operator", msg.Operator),
		sdk.NewAttribute("epoch", strconv.FormatUint(ep.Number, 10)),
		sdk.NewAttribute("task_index", strconv.FormatUint(uint64(msg.TaskIndex), 10)),
		sdk.NewAttribute("store", task.Store),
		sdk.NewAttribute("height", strconv.FormatInt(task.Height, 10))))
	return &types.MsgSubmitTaskProofResponse{}, nil
}

// DeregisterNode — spec §6.4.
func (k msgServer) DeregisterNode(ctx context.Context, msg *types.MsgDeregisterNode) (*types.MsgDeregisterNodeResponse, error) {
	op, err := k.operator(msg.Operator)
	if err != nil {
		return nil, err
	}
	p, err := k.Params.Get(ctx)
	if err != nil {
		return nil, err
	}
	node, err := k.node(ctx, op)
	if err != nil {
		return nil, err
	}
	if node.Status != types.NODE_STATUS_ACTIVE {
		return nil, types.ErrNodeNotActive
	}
	height := sdk.UnwrapSDKContext(ctx).BlockHeight()
	node.Status = types.NODE_STATUS_UNBONDING
	node.UnbondCompleteHeight = height + int64(p.UnbondingBlocks)
	if err := k.Nodes.Set(ctx, op, node); err != nil {
		return nil, err
	}
	total, err := k.totalActiveBond(ctx)
	if err != nil {
		return nil, err
	}
	if err := k.TotalActiveBond.Set(ctx, total.Sub(node.Bond)); err != nil {
		return nil, err
	}
	if err := k.UnbondingQueue.Set(ctx, collections.Join(node.UnbondCompleteHeight, op)); err != nil {
		return nil, err
	}
	sdk.UnwrapSDKContext(ctx).EventManager().EmitEvent(sdk.NewEvent("node_deregistered",
		sdk.NewAttribute("operator", msg.Operator),
		sdk.NewAttribute("unbond_complete_height", strconv.FormatInt(node.UnbondCompleteHeight, 10))))
	return &types.MsgDeregisterNodeResponse{UnbondCompleteHeight: node.UnbondCompleteHeight}, nil
}

// ReportDivergence — spec §6.5. Advisory only: never touches consensus.
func (k msgServer) ReportDivergence(ctx context.Context, msg *types.MsgReportDivergence) (*types.MsgReportDivergenceResponse, error) {
	op, err := k.operator(msg.Operator)
	if err != nil {
		return nil, err
	}
	p, err := k.Params.Get(ctx)
	if err != nil {
		return nil, err
	}
	ep, err := k.currentEpoch(ctx)
	if err != nil {
		return nil, err
	}
	node, err := k.node(ctx, op)
	if err != nil {
		return nil, err
	}
	if node.Status != types.NODE_STATUS_ACTIVE || node.RegisteredEpoch >= ep.Number {
		return nil, types.ErrNodeNotEligible
	}
	committed, err := k.AppHashes.Get(ctx, msg.Height)
	if err != nil || len(msg.ClaimedAppHash) != types.HashLen || bytes.Equal(committed, msg.ClaimedAppHash) {
		return nil, types.ErrNoDivergence
	}
	idxKey := collections.Join(msg.Height, op)
	if has, err := k.AlertIndex.Has(ctx, idxKey); err != nil {
		return nil, err
	} else if has {
		return nil, types.ErrDuplicateAlert
	}
	open, err := k.openAlertCount(ctx)
	if err != nil {
		return nil, err
	}
	if open >= uint64(p.MaxOpenAlerts) {
		return nil, types.ErrTooManyAlerts
	}
	if err := k.bankKeeper.SendCoinsFromAccountToModule(ctx, op, types.ModuleName,
		sdk.NewCoins(sdk.NewCoin(p.BondDenom, p.AlertDeposit))); err != nil {
		return nil, err
	}
	id, err := k.NextAlertID.Next(ctx)
	if err != nil {
		return nil, err
	}
	id++ // ids start at 1
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	alert := types.Alert{
		Id: id, Reporter: msg.Operator, Height: msg.Height, ClaimedAppHash: msg.ClaimedAppHash,
		Deposit: p.AlertDeposit, Status: types.ALERT_STATUS_OPEN,
		ExpiresHeight: sdkCtx.BlockHeight() + int64(p.AlertTtlBlocks),
	}
	if err := k.Alerts.Set(ctx, id, alert); err != nil {
		return nil, err
	}
	if err := k.AlertIndex.Set(ctx, idxKey, id); err != nil {
		return nil, err
	}
	if err := k.OpenAlerts.Set(ctx, collections.Join(alert.ExpiresHeight, id)); err != nil {
		return nil, err
	}
	if err := k.OpenAlertCount.Set(ctx, open+1); err != nil {
		return nil, err
	}
	sdkCtx.EventManager().EmitEvent(sdk.NewEvent("divergence_reported",
		sdk.NewAttribute("alert_id", strconv.FormatUint(id, 10)),
		sdk.NewAttribute("reporter", msg.Operator),
		sdk.NewAttribute("height", strconv.FormatInt(msg.Height, 10))))

	// Threshold: Σ bond of distinct reporters with OPEN alerts at this height (≤ max_open_alerts entries).
	reported := math.ZeroInt()
	rng := collections.NewPrefixedPairRange[int64, []byte](msg.Height)
	err = k.AlertIndex.Walk(ctx, rng, func(key collections.Pair[int64, []byte], aid uint64) (bool, error) {
		a, err := k.Alerts.Get(ctx, aid)
		if err != nil || a.Status != types.ALERT_STATUS_OPEN {
			return false, err
		}
		if n, err := k.Nodes.Get(ctx, key.K2()); err == nil && n.Status == types.NODE_STATUS_ACTIVE {
			reported = reported.Add(n.Bond)
		}
		return false, nil
	})
	if err != nil {
		return nil, err
	}
	total, err := k.totalActiveBond(ctx)
	if err != nil {
		return nil, err
	}
	if total.IsPositive() && reported.MulRaw(10_000).GTE(total.MulRaw(int64(p.AlertThresholdBps))) {
		sdkCtx.EventManager().EmitEvent(sdk.NewEvent("divergence_threshold",
			sdk.NewAttribute("height", strconv.FormatInt(msg.Height, 10)),
			sdk.NewAttribute("reported_bond", reported.String()),
			sdk.NewAttribute("total_active_bond", total.String())))
	}
	return &types.MsgReportDivergenceResponse{AlertId: id}, nil
}

// ResolveAlert — spec §6.6, governance only.
func (k msgServer) ResolveAlert(ctx context.Context, msg *types.MsgResolveAlert) (*types.MsgResolveAlertResponse, error) {
	authority, err := k.addressCodec.StringToBytes(msg.Authority)
	if err != nil || !bytes.Equal(authority, k.GetAuthority()) {
		return nil, errorsmod.Wrap(types.ErrInvalidSigner, "only governance can resolve alerts")
	}
	a, err := k.Alerts.Get(ctx, msg.AlertId)
	if err != nil || a.Status != types.ALERT_STATUS_OPEN {
		return nil, types.ErrAlertNotOpen
	}
	p, err := k.Params.Get(ctx)
	if err != nil {
		return nil, err
	}
	deposit := sdk.NewCoins(sdk.NewCoin(p.BondDenom, a.Deposit))
	if msg.Valid {
		reporter, err := k.addressCodec.StringToBytes(a.Reporter)
		if err != nil {
			return nil, err
		}
		if err := k.bankKeeper.SendCoinsFromModuleToAccount(ctx, types.ModuleName, reporter, deposit); err != nil {
			return nil, err
		}
		a.Status = types.ALERT_STATUS_ACCEPTED
	} else {
		// Provable misconduct (G-COMM-1): governance judged the alert false.
		// Forfeit the deposit and slash false_alert_slash_bps of the bond.
		slashed, err := k.slashReporter(ctx, a.Reporter, p)
		if err != nil {
			return nil, err
		}
		if err := k.distrKeeper.FundCommunityPool(ctx, deposit.Add(sdk.NewCoin(p.BondDenom, slashed)), k.ModuleAddress()); err != nil {
			return nil, err
		}
		a.Status = types.ALERT_STATUS_REJECTED
	}
	if err := k.closeAlert(ctx, a); err != nil {
		return nil, err
	}
	sdk.UnwrapSDKContext(ctx).EventManager().EmitEvent(sdk.NewEvent("alert_resolved",
		sdk.NewAttribute("alert_id", strconv.FormatUint(a.Id, 10)),
		sdk.NewAttribute("status", a.Status.String())))
	return &types.MsgResolveAlertResponse{}, nil
}

// slashReporter reduces the reporter's bond by false_alert_slash_bps (floor)
// and returns the slashed amount; the caller moves it out of the module.
// A reporter whose node no longer exists (fully unbonded) has nothing left to slash.
func (k Keeper) slashReporter(ctx context.Context, reporter string, p types.Params) (math.Int, error) {
	op, err := k.addressCodec.StringToBytes(reporter)
	if err != nil {
		return math.Int{}, err
	}
	node, err := k.Nodes.Get(ctx, op)
	if errors.Is(err, collections.ErrNotFound) {
		return math.ZeroInt(), nil
	} else if err != nil {
		return math.Int{}, err
	}
	slash := node.Bond.MulRaw(int64(p.FalseAlertSlashBps)).QuoRaw(10_000)
	if !slash.IsPositive() {
		return math.ZeroInt(), nil
	}
	node.Bond = node.Bond.Sub(slash)
	node.SlashedTotal = orZero(node.SlashedTotal).Add(slash)
	if err := k.Nodes.Set(ctx, op, node); err != nil {
		return math.Int{}, err
	}
	if node.Status == types.NODE_STATUS_ACTIVE {
		total, err := k.totalActiveBond(ctx)
		if err != nil {
			return math.Int{}, err
		}
		if err := k.TotalActiveBond.Set(ctx, total.Sub(slash)); err != nil {
			return math.Int{}, err
		}
	}
	sdk.UnwrapSDKContext(ctx).EventManager().EmitEvent(sdk.NewEvent("community_node_slashed",
		sdk.NewAttribute("operator", reporter), sdk.NewAttribute("amount", slash.String()),
		sdk.NewAttribute("reason", "false_divergence_alert")))
	return slash, nil
}

// orZero treats an unset (nil) Int as zero (nodes created before the field existed).
func orZero(i math.Int) math.Int {
	if i.IsNil() {
		return math.ZeroInt()
	}
	return i
}

func (k Keeper) closeAlert(ctx context.Context, a types.Alert) error {
	if err := k.Alerts.Set(ctx, a.Id, a); err != nil {
		return err
	}
	if err := k.OpenAlerts.Remove(ctx, collections.Join(a.ExpiresHeight, a.Id)); err != nil {
		return err
	}
	open, err := k.openAlertCount(ctx)
	if err != nil {
		return err
	}
	if open > 0 {
		open--
	}
	return k.OpenAlertCount.Set(ctx, open)
}
