package community

import (
	"fmt"
	"math/big"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"

	cmn "github.com/cosmos/evm/precompiles/common"

	communitytypes "nira-chain/x/community/types"
)

func argN(args []interface{}, n int) error {
	if len(args) != n {
		return fmt.Errorf(cmn.ErrInvalidNumberOfArgs, n, len(args))
	}
	return nil
}

// emit appends an EVM log with indexed topics (address/uint) and packed data.
func (p Precompile) emit(ctx sdk.Context, stateDB vm.StateDB, name string, indexed []interface{}, data ...interface{}) error {
	ev, ok := p.Events[name]
	if !ok {
		return fmt.Errorf("unknown event %s", name)
	}
	topics := []common.Hash{ev.ID}
	for _, v := range indexed {
		t, err := cmn.MakeTopic(v)
		if err != nil {
			return err
		}
		topics = append(topics, t)
	}
	var nonIndexed abi.Arguments
	for _, in := range ev.Inputs {
		if !in.Indexed {
			nonIndexed = append(nonIndexed, in)
		}
	}
	packed, err := nonIndexed.Pack(data...)
	if err != nil {
		return err
	}
	stateDB.AddLog(&ethtypes.Log{
		Address:     p.Address(),
		Topics:      topics,
		Data:        packed,
		BlockNumber: uint64(ctx.BlockHeight()), //nolint:gosec
	})
	return nil
}

func (p Precompile) RegisterNode(ctx sdk.Context, method *abi.Method, stateDB vm.StateDB, contract *vm.Contract, args []interface{}) ([]byte, error) {
	if err := argN(args, 4); err != nil {
		return nil, err
	}
	moniker, ok1 := args[0].(string)
	bond, ok2 := args[1].(*big.Int)
	tip, ok3 := args[2].([32]byte)
	length, ok4 := args[3].(uint32)
	if !ok1 || !ok2 || !ok3 || !ok4 || bond == nil || bond.Sign() < 0 {
		return nil, fmt.Errorf("invalid registerNode arguments")
	}
	op, err := p.operator(contract)
	if err != nil {
		return nil, err
	}
	denom, err := p.bondDenom(ctx)
	if err != nil {
		return nil, err
	}
	res, err := p.msgServer.RegisterNode(ctx, &communitytypes.MsgRegisterNode{
		Operator: op, Moniker: moniker, Bond: sdk.NewCoin(denom, math.NewIntFromBigInt(bond)),
		OnionTip: tip[:], OnionLength: length,
	})
	if err != nil {
		return nil, err
	}
	if err := p.emit(ctx, stateDB, "NodeRegistered", []interface{}{contract.Caller()}, bond, res.BaseEpoch); err != nil {
		return nil, err
	}
	return method.Outputs.Pack(res.BaseEpoch)
}

func (p Precompile) RotateOnion(ctx sdk.Context, method *abi.Method, stateDB vm.StateDB, contract *vm.Contract, args []interface{}) ([]byte, error) {
	if err := argN(args, 2); err != nil {
		return nil, err
	}
	tip, ok1 := args[0].([32]byte)
	length, ok2 := args[1].(uint32)
	if !ok1 || !ok2 {
		return nil, fmt.Errorf("invalid rotateOnion arguments")
	}
	op, err := p.operator(contract)
	if err != nil {
		return nil, err
	}
	res, err := p.msgServer.RotateOnion(ctx, &communitytypes.MsgRotateOnion{Operator: op, OnionTip: tip[:], OnionLength: length})
	if err != nil {
		return nil, err
	}
	if err := p.emit(ctx, stateDB, "OnionRotated", []interface{}{contract.Caller()}, res.BaseEpoch); err != nil {
		return nil, err
	}
	return method.Outputs.Pack(res.BaseEpoch)
}

func (p Precompile) SubmitTaskProof(ctx sdk.Context, method *abi.Method, stateDB vm.StateDB, contract *vm.Contract, args []interface{}) ([]byte, error) {
	if err := argN(args, 7); err != nil {
		return nil, err
	}
	epoch, ok1 := args[0].(uint64)
	reveal, ok2 := args[1].([32]byte)
	idx, ok3 := args[2].(uint32)
	store, ok4 := args[3].(string)
	height, ok5 := args[4].(int64)
	value, ok6 := args[5].([]byte)
	proof, ok7 := args[6].([]byte)
	if !ok1 || !ok2 || !ok3 || !ok4 || !ok5 || !ok6 || !ok7 {
		return nil, fmt.Errorf("invalid submitTaskProof arguments")
	}
	op, err := p.operator(contract)
	if err != nil {
		return nil, err
	}
	if _, err := p.msgServer.SubmitTaskProof(ctx, &communitytypes.MsgSubmitTaskProof{
		Operator: op, Epoch: epoch, Reveal: reveal[:], TaskIndex: idx, Store: store, Height: height, Value: value, Proof: proof,
	}); err != nil {
		return nil, err
	}
	if err := p.emit(ctx, stateDB, "TaskProven", []interface{}{contract.Caller(), epoch}, idx); err != nil {
		return nil, err
	}
	return method.Outputs.Pack(true)
}

func (p Precompile) DeregisterNode(ctx sdk.Context, method *abi.Method, stateDB vm.StateDB, contract *vm.Contract, args []interface{}) ([]byte, error) {
	if err := argN(args, 0); err != nil {
		return nil, err
	}
	op, err := p.operator(contract)
	if err != nil {
		return nil, err
	}
	res, err := p.msgServer.DeregisterNode(ctx, &communitytypes.MsgDeregisterNode{Operator: op})
	if err != nil {
		return nil, err
	}
	if err := p.emit(ctx, stateDB, "NodeDeregistered", []interface{}{contract.Caller()}, res.UnbondCompleteHeight); err != nil {
		return nil, err
	}
	return method.Outputs.Pack(res.UnbondCompleteHeight)
}

func (p Precompile) ReportDivergence(ctx sdk.Context, method *abi.Method, stateDB vm.StateDB, contract *vm.Contract, args []interface{}) ([]byte, error) {
	if err := argN(args, 2); err != nil {
		return nil, err
	}
	height, ok1 := args[0].(int64)
	claimed, ok2 := args[1].([32]byte)
	if !ok1 || !ok2 {
		return nil, fmt.Errorf("invalid reportDivergence arguments")
	}
	op, err := p.operator(contract)
	if err != nil {
		return nil, err
	}
	res, err := p.msgServer.ReportDivergence(ctx, &communitytypes.MsgReportDivergence{Operator: op, Height: height, ClaimedAppHash: claimed[:]})
	if err != nil {
		return nil, err
	}
	if err := p.emit(ctx, stateDB, "DivergenceReported", []interface{}{contract.Caller(), res.AlertId}, height); err != nil {
		return nil, err
	}
	return method.Outputs.Pack(res.AlertId)
}
