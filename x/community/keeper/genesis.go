package keeper

import (
	"context"

	"cosmossdk.io/collections"
	"cosmossdk.io/math"

	"nira-chain/x/community/types"
)

// InitGenesis imports params, nodes, alerts, the epoch and the in-progress
// epoch state (work, proven tasks, app-hash window), so an export → import
// upgrade settles the epoch exactly as if it had never been interrupted.
func (k Keeper) InitGenesis(ctx context.Context, gs types.GenesisState) error {
	if err := k.Params.Set(ctx, gs.Params); err != nil {
		return err
	}
	total := math.ZeroInt()
	for _, n := range gs.Nodes {
		op, err := k.addressCodec.StringToBytes(n.Operator)
		if err != nil {
			return err
		}
		n.RewardsTotal, n.SlashedTotal = orZero(n.RewardsTotal), orZero(n.SlashedTotal)
		if err := k.Nodes.Set(ctx, op, n); err != nil {
			return err
		}
		switch n.Status {
		case types.NODE_STATUS_ACTIVE:
			total = total.Add(n.Bond)
		case types.NODE_STATUS_UNBONDING:
			if err := k.UnbondingQueue.Set(ctx, collections.Join(n.UnbondCompleteHeight, op)); err != nil {
				return err
			}
		}
	}
	if err := k.TotalActiveBond.Set(ctx, total); err != nil { // INV-COMM-5
		return err
	}
	open := uint64(0)
	for _, a := range gs.Alerts {
		if err := k.Alerts.Set(ctx, a.Id, a); err != nil {
			return err
		}
		reporter, err := k.addressCodec.StringToBytes(a.Reporter)
		if err != nil {
			return err
		}
		if err := k.AlertIndex.Set(ctx, collections.Join(a.Height, reporter), a.Id); err != nil {
			return err
		}
		if a.Status == types.ALERT_STATUS_OPEN {
			open++
			if err := k.OpenAlerts.Set(ctx, collections.Join(a.ExpiresHeight, a.Id)); err != nil {
				return err
			}
		}
	}
	if err := k.OpenAlertCount.Set(ctx, open); err != nil {
		return err
	}
	if gs.NextAlertId > 1 {
		if err := k.NextAlertID.Set(ctx, gs.NextAlertId-1); err != nil {
			return err
		}
	}
	if gs.Epoch != nil {
		if err := k.Epoch.Set(ctx, *gs.Epoch); err != nil {
			return err
		}
	}
	for _, w := range gs.EpochWork {
		op, err := k.addressCodec.StringToBytes(w.Operator)
		if err != nil {
			return err
		}
		if err := k.EpochWork.Set(ctx, collections.Join(w.Epoch, op), w.Units); err != nil {
			return err
		}
	}
	for _, t := range gs.TaskDone {
		op, err := k.addressCodec.StringToBytes(t.Operator)
		if err != nil {
			return err
		}
		if err := k.TaskDone.Set(ctx, collections.Join3(t.Epoch, op, t.TaskIndex)); err != nil {
			return err
		}
	}
	for _, a := range gs.AppHashes {
		if err := k.AppHashes.Set(ctx, a.Height, a.AppHash); err != nil {
			return err
		}
	}
	return nil
}

// ExportGenesis exports params, nodes, alerts and the current epoch.
func (k Keeper) ExportGenesis(ctx context.Context) (*types.GenesisState, error) {
	gs := types.DefaultGenesis()
	var err error
	if gs.Params, err = k.Params.Get(ctx); err != nil {
		return nil, err
	}
	if err := k.Nodes.Walk(ctx, nil, func(_ []byte, n types.Node) (bool, error) {
		gs.Nodes = append(gs.Nodes, n)
		return false, nil
	}); err != nil {
		return nil, err
	}
	if err := k.Alerts.Walk(ctx, nil, func(_ uint64, a types.Alert) (bool, error) {
		gs.Alerts = append(gs.Alerts, a)
		return false, nil
	}); err != nil {
		return nil, err
	}
	seq, err := k.NextAlertID.Peek(ctx)
	if err != nil {
		return nil, err
	}
	gs.NextAlertId = seq + 1
	if ep, err := k.Epoch.Get(ctx); err == nil {
		gs.Epoch = &ep
	}
	if err := k.EpochWork.Walk(ctx, nil, func(key collections.Pair[uint64, []byte], units uint32) (bool, error) {
		op, err := k.addressCodec.BytesToString(key.K2())
		gs.EpochWork = append(gs.EpochWork, types.EpochWork{Epoch: key.K1(), Operator: op, Units: units})
		return false, err
	}); err != nil {
		return nil, err
	}
	if err := k.TaskDone.Walk(ctx, nil, func(key collections.Triple[uint64, []byte, uint32]) (bool, error) {
		op, err := k.addressCodec.BytesToString(key.K2())
		gs.TaskDone = append(gs.TaskDone, types.TaskDone{Epoch: key.K1(), Operator: op, TaskIndex: key.K3()})
		return false, err
	}); err != nil {
		return nil, err
	}
	if err := k.AppHashes.Walk(ctx, nil, func(h int64, hash []byte) (bool, error) {
		gs.AppHashes = append(gs.AppHashes, types.AppHashEntry{Height: h, AppHash: hash})
		return false, nil
	}); err != nil {
		return nil, err
	}
	return gs, nil
}
