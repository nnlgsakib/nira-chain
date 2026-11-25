package keeper

import (
	"context"

	"cosmossdk.io/collections"
	"cosmossdk.io/math"

	"nira-chain/x/community/types"
)

// InitGenesis imports params, nodes, alerts and the epoch. Epoch-scoped work
// and the app-hash ring are not exported: an imported chain resumes with the
// current epoch's work empty (documented in genesis.proto).
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
	return gs, nil
}
