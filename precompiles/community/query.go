package community

import (
	"fmt"
	"math/big"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"

	communitytypes "nira-chain/x/community/types"
)

// ABI output tuples (field names match the Solidity structs).
type NodeInfo struct {
	Moniker              string
	Bond                 *big.Int
	Status               uint8
	RegisteredEpoch      uint64
	OnionBaseEpoch       uint64
	UnitsTotal           uint64
	UnitsThisEpoch       uint32
	UnbondCompleteHeight int64
}

type EpochInfo struct {
	Number          uint64
	StartHeight     int64
	NextEpochHeight int64
	Seed            [32]byte
	EligibleBond    *big.Int
	Budget          *big.Int
	TotalActiveBond *big.Int
}

type TaskInfo struct {
	Index  uint32
	Store  string
	Height int64
	Key    [32]byte
}

func to32(b []byte) (out [32]byte) { copy(out[:], b); return }

func (p Precompile) GetNode(ctx sdk.Context, method *abi.Method, args []interface{}) ([]byte, error) {
	if err := argN(args, 1); err != nil {
		return nil, err
	}
	addr, ok := args[0].(common.Address)
	if !ok {
		return nil, fmt.Errorf("invalid address")
	}
	op, err := p.addrCodec.BytesToString(addr.Bytes())
	if err != nil {
		return nil, err
	}
	res, err := p.queryServer.Node(ctx, &communitytypes.QueryNodeRequest{Operator: op})
	if err != nil {
		return nil, err
	}
	n := res.Node
	return method.Outputs.Pack(NodeInfo{
		Moniker: n.Moniker, Bond: n.Bond.BigInt(), Status: uint8(n.Status), //nolint:gosec
		RegisteredEpoch: n.RegisteredEpoch, OnionBaseEpoch: n.OnionBaseEpoch, UnitsTotal: n.UnitsTotal,
		UnitsThisEpoch: res.UnitsThisEpoch, UnbondCompleteHeight: n.UnbondCompleteHeight,
	})
}

func (p Precompile) CurrentEpoch(ctx sdk.Context, method *abi.Method) ([]byte, error) {
	res, err := p.queryServer.CurrentEpoch(ctx, &communitytypes.QueryCurrentEpochRequest{})
	if err != nil {
		return nil, err
	}
	e := res.Epoch
	return method.Outputs.Pack(EpochInfo{
		Number: e.Number, StartHeight: e.StartHeight, NextEpochHeight: res.NextEpochHeight, Seed: to32(e.Seed),
		EligibleBond: e.EligibleBond.BigInt(), Budget: e.Budget.BigInt(), TotalActiveBond: res.TotalActiveBond.BigInt(),
	})
}

func (p Precompile) GetTasks(ctx sdk.Context, method *abi.Method, args []interface{}) ([]byte, error) {
	if err := argN(args, 1); err != nil {
		return nil, err
	}
	addr, ok := args[0].(common.Address)
	if !ok {
		return nil, fmt.Errorf("invalid address")
	}
	op, err := p.addrCodec.BytesToString(addr.Bytes())
	if err != nil {
		return nil, err
	}
	res, err := p.queryServer.Tasks(ctx, &communitytypes.QueryTasksRequest{Operator: op})
	if err != nil {
		return nil, err
	}
	out := make([]TaskInfo, len(res.Tasks))
	for i, t := range res.Tasks {
		out[i] = TaskInfo{Index: t.Index, Store: t.Store, Height: t.Height, Key: to32(t.Key)}
	}
	return method.Outputs.Pack(out)
}
