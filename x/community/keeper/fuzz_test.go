package keeper_test

import (
	"testing"

	"nira-chain/x/community/types"
)

// G-SEC-3: arbitrary SubmitTaskProof / ReportDivergence fields from an
// eligible, selected operator must never panic, never pay a unit for an
// invalid proof, and never break the module invariants.
func FuzzSubmitTaskProof(f *testing.F) {
	f.Add(uint64(2), []byte{1, 2}, uint32(0), "acc", int64(5), []byte{}, []byte{0x0a, 0x00}, int64(3), []byte{7})
	f.Fuzz(func(t *testing.T, epoch uint64, reveal []byte, idx uint32, store string, height int64, value, proof []byte,
		alertHeight int64, claimed []byte) {
		fx := initFixture(t)
		a := fx.newOperator(t, 1, nira(10_000))
		startEligible(t, fx, a)
		before, _ := fx.keeper.Epoch.Get(fx.ctx)
		_, err := fx.msg.SubmitTaskProof(fx.ctx, &types.MsgSubmitTaskProof{
			Operator: a.bech, Epoch: epoch, Reveal: reveal, TaskIndex: idx, Store: store, Height: height, Value: value, Proof: proof,
		})
		after, _ := fx.keeper.Epoch.Get(fx.ctx)
		if err == nil {
			t.Fatalf("fuzzed proof accepted (store=%q height=%d proof=%x)", store, height, proof)
		}
		if after.Proofs != before.Proofs {
			t.Fatalf("failed proof changed the epoch proof count")
		}
		_, _ = fx.msg.ReportDivergence(fx.ctx, &types.MsgReportDivergence{Operator: a.bech, Height: alertHeight, ClaimedAppHash: claimed})
		fx.checkInvariants(t)
	})
}
