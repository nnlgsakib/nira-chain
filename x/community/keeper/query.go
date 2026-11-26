package keeper

import (
	"context"
	"errors"

	"cosmossdk.io/collections"
	"github.com/cosmos/cosmos-sdk/types/query"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"nira-chain/x/community/types"
)

var _ types.QueryServer = queryServer{}

// NewQueryServerImpl returns an implementation of the QueryServer interface.
func NewQueryServerImpl(k Keeper) types.QueryServer { return queryServer{k} }

type queryServer struct{ k Keeper }

func (q queryServer) operator(addr string) ([]byte, error) {
	op, err := q.k.addressCodec.StringToBytes(addr)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid operator address")
	}
	return op, nil
}

func (q queryServer) Node(ctx context.Context, req *types.QueryNodeRequest) (*types.QueryNodeResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}
	op, err := q.operator(req.Operator)
	if err != nil {
		return nil, err
	}
	n, err := q.k.Nodes.Get(ctx, op)
	if errors.Is(err, collections.ErrNotFound) {
		return nil, status.Error(codes.NotFound, "node not found")
	} else if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	ep, err := q.k.currentEpoch(ctx)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	units, err := q.k.EpochWork.Get(ctx, collections.Join(ep.Number, op))
	if err != nil && !errors.Is(err, collections.ErrNotFound) {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &types.QueryNodeResponse{Node: n, UnitsThisEpoch: units}, nil
}

func (q queryServer) Nodes(ctx context.Context, req *types.QueryNodesRequest) (*types.QueryNodesResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}
	nodes, pageRes, err := query.CollectionPaginate(ctx, q.k.Nodes, req.Pagination,
		func(_ []byte, n types.Node) (types.Node, error) { return n, nil })
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &types.QueryNodesResponse{Nodes: nodes, Pagination: pageRes}, nil
}

func (q queryServer) CurrentEpoch(ctx context.Context, req *types.QueryCurrentEpochRequest) (*types.QueryCurrentEpochResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}
	p, err := q.k.Params.Get(ctx)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	ep, err := q.k.currentEpoch(ctx)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	total, err := q.k.totalActiveBond(ctx)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &types.QueryCurrentEpochResponse{
		Epoch:           ep,
		NextEpochHeight: ep.StartHeight + int64(p.EpochBlocks),
		TotalActiveBond: total,
	}, nil
}

func (q queryServer) Tasks(ctx context.Context, req *types.QueryTasksRequest) (*types.QueryTasksResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}
	op, err := q.operator(req.Operator)
	if err != nil {
		return nil, err
	}
	p, err := q.k.Params.Get(ctx)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	ep, err := q.k.currentEpoch(ctx)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	specs := q.k.Tasks(ep, p, op)
	out := make([]types.Task, len(specs))
	for i, t := range specs {
		out[i] = types.Task{Index: t.Index, Store: t.Store, Height: t.Height, Key: t.Key}
	}
	return &types.QueryTasksResponse{Epoch: ep.Number, Tasks: out}, nil
}

func (q queryServer) Alert(ctx context.Context, req *types.QueryAlertRequest) (*types.QueryAlertResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}
	a, err := q.k.Alerts.Get(ctx, req.Id)
	if errors.Is(err, collections.ErrNotFound) {
		return nil, status.Error(codes.NotFound, "alert not found")
	} else if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &types.QueryAlertResponse{Alert: a}, nil
}
