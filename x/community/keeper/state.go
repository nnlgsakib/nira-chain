package keeper

import (
	"context"
	"errors"

	"cosmossdk.io/collections"
	"cosmossdk.io/math"

	"nira-chain/x/community/types"
)

func (k Keeper) totalActiveBond(ctx context.Context) (math.Int, error) {
	v, err := k.TotalActiveBond.Get(ctx)
	if errors.Is(err, collections.ErrNotFound) {
		return math.ZeroInt(), nil
	}
	return v, err
}

func (k Keeper) openAlertCount(ctx context.Context) (uint64, error) {
	v, err := k.OpenAlertCount.Get(ctx)
	if errors.Is(err, collections.ErrNotFound) {
		return 0, nil
	}
	return v, err
}

// currentEpoch returns the open epoch; before the first BeginBlock it is epoch 0.
func (k Keeper) currentEpoch(ctx context.Context) (types.Epoch, error) {
	e, err := k.Epoch.Get(ctx)
	if errors.Is(err, collections.ErrNotFound) {
		return types.Epoch{EligibleBond: math.ZeroInt(), Budget: math.ZeroInt()}, nil
	}
	return e, err
}

// node fetches a node by operator bytes.
func (k Keeper) node(ctx context.Context, op []byte) (types.Node, error) {
	n, err := k.Nodes.Get(ctx, op)
	if errors.Is(err, collections.ErrNotFound) {
		return n, types.ErrNodeNotFound
	}
	return n, err
}

// Tasks derives the task list of an operator for the given epoch.
func (k Keeper) Tasks(epoch types.Epoch, params types.Params, op []byte) []types.TaskSpec {
	out := make([]types.TaskSpec, 0, params.TasksPerSelection)
	for i := uint32(0); i < params.TasksPerSelection; i++ {
		t, ok := types.DeriveTask(epoch.Seed, op, i, epoch.StartHeight, params.ApphashWindow, params.SampleStores)
		if ok {
			out = append(out, t)
		}
	}
	return out
}
