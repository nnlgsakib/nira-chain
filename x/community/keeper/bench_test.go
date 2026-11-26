package keeper_test

import (
	"testing"

	"nira-chain/x/community/types"
)

// BenchmarkVerifyStateProof measures on-chain verification of one real ICS-23
// state proof (the per-task cost of community verification, G-TEST-4).
func BenchmarkVerifyStateProof(b *testing.B) {
	t := &testing.T{}
	f := initFixture(t)
	a := f.newOperator(t, 1, nira(10_000))
	startEligible(t, f, a)
	m := f.proofMsg(t, a, 0)
	appHash, _ := f.keeper.AppHashes.Get(f.ctx, m.Height)
	ep, _ := f.keeper.Epoch.Get(f.ctx)
	p, _ := f.keeper.Params.Get(f.ctx)
	task := f.keeper.Tasks(ep, p, a.addr)[0]
	b.ReportMetric(float64(len(m.Proof)), "proof_bytes")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := types.VerifyStateProof(appHash, task.Store, task.Key, m.Value, m.Proof); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkSubmitTaskProof measures the full message handler (checks C1–C12 + writes).
func BenchmarkSubmitTaskProof(b *testing.B) {
	t := &testing.T{}
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		f := initFixture(t)
		a := f.newOperator(t, 1, nira(10_000))
		startEligible(t, f, a)
		m := f.proofMsg(t, a, 0)
		gas0 := f.ctx.GasMeter().GasConsumed()
		b.StartTimer()
		if _, err := f.msg.SubmitTaskProof(f.ctx, m); err != nil {
			b.Fatal(err)
		}
		b.StopTimer()
		b.ReportMetric(float64(f.ctx.GasMeter().GasConsumed()-gas0), "gas/op")
	}
}
