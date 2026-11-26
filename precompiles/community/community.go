// Package community is the EVM precompile for x/community (ADR-0006) at
// 0x0000000000000000000000000000000000000900. It is a thin ABI adapter over
// the module's MsgServer / QueryServer, so Cosmos and EVM callers share one
// code path and one state. The operator is always msg.sender.
package community

import (
	"embed"
	"fmt"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"

	cmn "github.com/cosmos/evm/precompiles/common"

	"cosmossdk.io/core/address"
	"cosmossdk.io/log"
	storetypes "cosmossdk.io/store/types"
	sdk "github.com/cosmos/cosmos-sdk/types"

	communitytypes "nira-chain/x/community/types"
)

// Address of the precompile (static, must be listed in evm params
// active_static_precompiles to be callable).
const Address = "0x0000000000000000000000000000000000000900"

var _ vm.PrecompiledContract = &Precompile{}

var (
	//go:embed abi.json
	f   embed.FS
	ABI abi.ABI
)

func init() {
	var err error
	ABI, err = cmn.LoadABI(f, "abi.json")
	if err != nil {
		panic(err)
	}
}

const (
	RegisterNodeMethod     = "registerNode"
	RotateOnionMethod      = "rotateOnion"
	SubmitTaskProofMethod  = "submitTaskProof"
	DeregisterNodeMethod   = "deregisterNode"
	ReportDivergenceMethod = "reportDivergence"
	GetNodeMethod          = "getNode"
	CurrentEpochMethod     = "currentEpoch"
	GetTasksMethod         = "getTasks"
)

// Precompile is the x/community EVM extension.
type Precompile struct {
	cmn.Precompile
	abi.ABI
	msgServer   communitytypes.MsgServer
	queryServer communitytypes.QueryServer
	addrCodec   address.Codec
	bondDenom   func(ctx sdk.Context) (string, error)
}

// NewPrecompile builds the precompile. bankKeeper feeds the balance handler
// that mirrors native coin movements (bond escrow, payouts) into the StateDB.
func NewPrecompile(
	msgServer communitytypes.MsgServer,
	queryServer communitytypes.QueryServer,
	bankKeeper cmn.BankKeeper,
	addrCodec address.Codec,
) *Precompile {
	p := &Precompile{
		Precompile: cmn.Precompile{
			KvGasConfig:           storetypes.KVGasConfig(),
			TransientKVGasConfig:  storetypes.TransientGasConfig(),
			ContractAddress:       common.HexToAddress(Address),
			BalanceHandlerFactory: cmn.NewBalanceHandlerFactory(bankKeeper),
		},
		ABI:         ABI,
		msgServer:   msgServer,
		queryServer: queryServer,
		addrCodec:   addrCodec,
	}
	p.bondDenom = func(ctx sdk.Context) (string, error) {
		res, err := queryServer.Params(ctx, &communitytypes.QueryParamsRequest{})
		if err != nil {
			return "", err
		}
		return res.Params.BondDenom, nil
	}
	return p
}

// RequiredGas is the base cost; proof verification additionally consumes
// P-COMM-proof_gas_per_byte through the SDK gas meter inside the msg server.
func (p Precompile) RequiredGas(input []byte) uint64 {
	if len(input) < 4 {
		return 0
	}
	method, err := p.MethodById(input[:4])
	if err != nil {
		return 0
	}
	return p.Precompile.RequiredGas(input, p.IsTransaction(method))
}

func (p Precompile) Run(evm *vm.EVM, contract *vm.Contract, readonly bool) ([]byte, error) {
	return p.RunNativeAction(evm, contract, func(ctx sdk.Context) ([]byte, error) {
		return p.Execute(ctx, evm.StateDB, contract, readonly)
	})
}

func (p Precompile) Execute(ctx sdk.Context, stateDB vm.StateDB, contract *vm.Contract, readOnly bool) ([]byte, error) {
	method, args, err := cmn.SetupABI(p.ABI, contract, readOnly, p.IsTransaction)
	if err != nil {
		return nil, err
	}
	switch method.Name {
	case RegisterNodeMethod:
		return p.RegisterNode(ctx, method, stateDB, contract, args)
	case RotateOnionMethod:
		return p.RotateOnion(ctx, method, stateDB, contract, args)
	case SubmitTaskProofMethod:
		return p.SubmitTaskProof(ctx, method, stateDB, contract, args)
	case DeregisterNodeMethod:
		return p.DeregisterNode(ctx, method, stateDB, contract, args)
	case ReportDivergenceMethod:
		return p.ReportDivergence(ctx, method, stateDB, contract, args)
	case GetNodeMethod:
		return p.GetNode(ctx, method, args)
	case CurrentEpochMethod:
		return p.CurrentEpoch(ctx, method)
	case GetTasksMethod:
		return p.GetTasks(ctx, method, args)
	default:
		return nil, fmt.Errorf(cmn.ErrUnknownMethod, method.Name)
	}
}

// IsTransaction reports whether a method writes state.
func (Precompile) IsTransaction(method *abi.Method) bool {
	switch method.Name {
	case RegisterNodeMethod, RotateOnionMethod, SubmitTaskProofMethod, DeregisterNodeMethod, ReportDivergenceMethod:
		return true
	default:
		return false
	}
}

func (p Precompile) Logger(ctx sdk.Context) log.Logger {
	return ctx.Logger().With("evm extension", "community")
}

// operator converts msg.sender into the module's address string.
func (p Precompile) operator(contract *vm.Contract) (string, error) {
	return p.addrCodec.BytesToString(contract.Caller().Bytes())
}
