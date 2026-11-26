package community

import (
	"context"
	"testing"

	"cosmossdk.io/math"
	storetypes "cosmossdk.io/store/types"
	addresscodec "github.com/cosmos/cosmos-sdk/codec/address"
	"github.com/cosmos/cosmos-sdk/testutil"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"

	communitytypes "nira-chain/x/community/types"
)

// G-SEC-3: fuzz the precompile from raw calldata (selector + ABI decoding +
// every handler + event/output encoding). Stub keepers stand in for the module,
// so the target isolates the precompile's own decoding and encoding.

type stubMsg struct {
	communitytypes.MsgServer
	calls int
}

func (s *stubMsg) RegisterNode(_ context.Context, m *communitytypes.MsgRegisterNode) (*communitytypes.MsgRegisterNodeResponse, error) {
	s.calls++
	if m.Operator == "" || m.Bond.Amount.IsNil() {
		panic("precompile passed an incomplete MsgRegisterNode")
	}
	return &communitytypes.MsgRegisterNodeResponse{BaseEpoch: 1}, nil
}
func (s *stubMsg) RotateOnion(context.Context, *communitytypes.MsgRotateOnion) (*communitytypes.MsgRotateOnionResponse, error) {
	s.calls++
	return &communitytypes.MsgRotateOnionResponse{BaseEpoch: 2}, nil
}
func (s *stubMsg) SubmitTaskProof(context.Context, *communitytypes.MsgSubmitTaskProof) (*communitytypes.MsgSubmitTaskProofResponse, error) {
	s.calls++
	return &communitytypes.MsgSubmitTaskProofResponse{}, nil
}
func (s *stubMsg) DeregisterNode(context.Context, *communitytypes.MsgDeregisterNode) (*communitytypes.MsgDeregisterNodeResponse, error) {
	s.calls++
	return &communitytypes.MsgDeregisterNodeResponse{UnbondCompleteHeight: 9}, nil
}
func (s *stubMsg) ReportDivergence(context.Context, *communitytypes.MsgReportDivergence) (*communitytypes.MsgReportDivergenceResponse, error) {
	s.calls++
	return &communitytypes.MsgReportDivergenceResponse{AlertId: 3}, nil
}

type stubQuery struct{ communitytypes.QueryServer }

func (stubQuery) Params(context.Context, *communitytypes.QueryParamsRequest) (*communitytypes.QueryParamsResponse, error) {
	return &communitytypes.QueryParamsResponse{Params: communitytypes.DefaultParams()}, nil
}
func (stubQuery) Node(context.Context, *communitytypes.QueryNodeRequest) (*communitytypes.QueryNodeResponse, error) {
	return &communitytypes.QueryNodeResponse{Node: communitytypes.Node{Bond: math.NewInt(5)}}, nil
}
func (stubQuery) CurrentEpoch(context.Context, *communitytypes.QueryCurrentEpochRequest) (*communitytypes.QueryCurrentEpochResponse, error) {
	return &communitytypes.QueryCurrentEpochResponse{Epoch: communitytypes.Epoch{EligibleBond: math.OneInt(), Budget: math.ZeroInt(), Seed: make([]byte, 32)}, TotalActiveBond: math.OneInt()}, nil
}
func (stubQuery) Tasks(context.Context, *communitytypes.QueryTasksRequest) (*communitytypes.QueryTasksResponse, error) {
	return &communitytypes.QueryTasksResponse{Tasks: []communitytypes.Task{{Store: "acc", Key: make([]byte, 32)}}}, nil
}

// fakeDB implements only AddLog; any other StateDB use by a handler panics,
// which the fuzzer would report.
type fakeDB struct {
	vm.StateDB
	logs int
}

func (f *fakeDB) AddLog(*ethtypes.Log) { f.logs++ }

func FuzzPrecompileCalldata(f *testing.F) {
	// seeds: one well-formed call per method
	addr := [20]byte{1}
	for _, m := range ABI.Methods {
		var args []interface{}
		switch m.Name {
		case RegisterNodeMethod:
			args = []interface{}{"node", math.NewInt(1000).BigInt(), [32]byte{1}, uint32(10)}
		case RotateOnionMethod:
			args = []interface{}{[32]byte{2}, uint32(5)}
		case SubmitTaskProofMethod:
			args = []interface{}{uint64(1), [32]byte{3}, uint32(0), "acc", int64(5), []byte{1}, []byte{2}}
		case ReportDivergenceMethod:
			args = []interface{}{int64(4), [32]byte{4}}
		case GetNodeMethod, GetTasksMethod:
			args = []interface{}{addr}
		}
		packed, err := m.Inputs.Pack(args...)
		if err != nil {
			f.Fatalf("seed %s: %v", m.Name, err)
		}
		f.Add(append(append([]byte{}, m.ID...), packed...))
	}
	f.Add([]byte{0, 1, 2})

	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) < 4 {
			return
		}
		method, err := ABI.MethodById(data[:4])
		if err != nil {
			return
		}
		args, err := method.Inputs.Unpack(data[4:])
		if err != nil {
			return // malformed calldata is rejected by the ABI layer
		}
		key := storetypes.NewKVStoreKey("fuzz")
		ctx := testutil.DefaultContext(key, storetypes.NewTransientStoreKey("fuzz_t"))
		msg := &stubMsg{}
		p := NewPrecompile(msg, stubQuery{}, nil, addresscodec.NewBech32Codec("nira"))
		db := &fakeDB{}
		contract := vm.NewContract([20]byte{9}, [20]byte{}, nil, 1_000_000, nil)
		var out []byte
		switch method.Name {
		case RegisterNodeMethod:
			out, err = p.RegisterNode(ctx, method, db, contract, args)
		case RotateOnionMethod:
			out, err = p.RotateOnion(ctx, method, db, contract, args)
		case SubmitTaskProofMethod:
			out, err = p.SubmitTaskProof(ctx, method, db, contract, args)
		case DeregisterNodeMethod:
			out, err = p.DeregisterNode(ctx, method, db, contract, args)
		case ReportDivergenceMethod:
			out, err = p.ReportDivergence(ctx, method, db, contract, args)
		case GetNodeMethod:
			out, err = p.GetNode(ctx, method, args)
		case CurrentEpochMethod:
			out, err = p.CurrentEpoch(ctx, method)
		case GetTasksMethod:
			out, err = p.GetTasks(ctx, method, args)
		default:
			t.Fatalf("method %s has no handler", method.Name)
		}
		if err != nil {
			return
		}
		// a successful call must return outputs that decode with the ABI
		if _, derr := method.Outputs.Unpack(out); derr != nil {
			t.Fatalf("%s returned undecodable output: %v", method.Name, derr)
		}
		if p.IsTransaction(method) && (msg.calls != 1 || db.logs != 1) {
			t.Fatalf("%s: module calls %d, logs %d (want 1, 1)", method.Name, msg.calls, db.logs)
		}
	})
}
