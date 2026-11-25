package keeper_test

import (
	"bytes"
	"math/rand"
	"testing"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	"github.com/stretchr/testify/require"

	"nira-chain/x/community/types"
)

func feeCollector() sdk.AccAddress { return modAddr(authtypes.FeeCollectorName) }

func hasEvent(ctx sdk.Context, typ string) bool {
	for _, e := range ctx.EventManager().Events() {
		if e.Type == typ {
			return true
		}
	}
	return false
}

// startEligible: register ops in epoch 1, advance to epoch 2 (start height 11).
func startEligible(t *testing.T, f *fixture, ops ...*operator) {
	f.advance(t, 1)
	for _, o := range ops {
		f.register(t, o, nira(1000))
	}
	f.advance(t, 11)
	ep, _ := f.keeper.Epoch.Get(f.ctx)
	require.Equal(t, uint64(2), ep.Number)
}

func TestRegisterNode(t *testing.T) {
	f := initFixture(t)
	f.advance(t, 1)
	o := f.newOperator(t, 1, nira(10_000))
	res := f.register(t, o, nira(1000))
	require.Equal(t, uint64(2), res.BaseEpoch)
	require.True(t, f.moduleBalance().Equal(nira(1000)))

	_, err := f.msg.RegisterNode(f.ctx, &types.MsgRegisterNode{Operator: o.bech, Bond: sdk.NewCoin(denom, nira(1000)), OnionTip: o.tip(), OnionLength: 10})
	require.ErrorIs(t, err, types.ErrNodeExists) // C2

	o2 := f.newOperator(t, 2, nira(10_000))
	bad := []struct {
		msg types.MsgRegisterNode
		err error
	}{
		{types.MsgRegisterNode{Operator: o2.bech, Bond: sdk.NewCoin(denom, nira(999)), OnionTip: o2.tip(), OnionLength: 10}, types.ErrInsufficientBond},
		{types.MsgRegisterNode{Operator: o2.bech, Bond: sdk.NewCoin("other", nira(1000)), OnionTip: o2.tip(), OnionLength: 10}, types.ErrInsufficientBond},
		{types.MsgRegisterNode{Operator: o2.bech, Bond: sdk.NewCoin(denom, nira(1000)), OnionTip: make([]byte, 32), OnionLength: 10}, types.ErrInvalidOnion},
		{types.MsgRegisterNode{Operator: o2.bech, Bond: sdk.NewCoin(denom, nira(1000)), OnionTip: o2.tip()[:31], OnionLength: 10}, types.ErrInvalidOnion},
		{types.MsgRegisterNode{Operator: o2.bech, Bond: sdk.NewCoin(denom, nira(1000)), OnionTip: o2.tip(), OnionLength: 0}, types.ErrInvalidOnion},
		{types.MsgRegisterNode{Operator: o2.bech, Bond: sdk.NewCoin(denom, nira(1000)), OnionTip: o2.tip(), OnionLength: 10, Moniker: string(make([]byte, 65))}, types.ErrInvalidMoniker},
		{types.MsgRegisterNode{Operator: "bad", Bond: sdk.NewCoin(denom, nira(1000)), OnionTip: o2.tip(), OnionLength: 10}, types.ErrInvalidSigner},
	}
	for i, c := range bad {
		_, err := f.msg.RegisterNode(f.ctx, &c.msg)
		require.ErrorIs(t, err, c.err, "case %d", i)
	}
	// insufficient funds
	poor := f.newOperator(t, 3, nira(10))
	_, err = f.msg.RegisterNode(f.ctx, &types.MsgRegisterNode{Operator: poor.bech, Bond: sdk.NewCoin(denom, nira(1000)), OnionTip: poor.tip(), OnionLength: 10})
	require.Error(t, err)
	f.checkInvariants(t)
}

// Full flow: proofs verified against real IAVL app hashes, settlement pro rata (INV-COMM-1, -2).
func TestProofAndSettlement(t *testing.T) {
	f := initFixture(t)
	a := f.newOperator(t, 1, nira(10_000))
	b := f.newOperator(t, 2, nira(10_000))
	startEligible(t, f, a, b)

	// A proves 2 tasks, B proves 1
	for _, m := range []*types.MsgSubmitTaskProof{f.proofMsg(t, a, 0), f.proofMsg(t, a, 1), f.proofMsg(t, b, 0)} {
		_, err := f.msg.SubmitTaskProof(f.ctx, m)
		require.NoError(t, err)
	}
	require.True(t, hasEvent(f.ctx, "task_proven"))
	nr, err := f.query.Node(f.ctx, &types.QueryNodeRequest{Operator: a.bech})
	require.NoError(t, err)
	require.Equal(t, uint32(2), nr.UnitsThisEpoch)
	f.checkInvariants(t)

	// fund: fee collector holds 1000 NIRA at each block → 15 % moved per block
	aBefore := f.bank.bal[string(a.addr)].AmountOf(denom)
	bBefore := f.bank.bal[string(b.addr)].AmountOf(denom)
	for h := int64(12); h <= 20; h++ {
		f.bank.bal[string(feeCollector())] = sdk.NewCoins(sdk.NewCoin(denom, nira(1000)))
		f.advance(t, h)
	}
	ep, _ := f.keeper.Epoch.Get(f.ctx)
	budget := ep.Budget
	require.True(t, budget.Equal(nira(150).MulRaw(9)), "budget %s", budget)
	poolBefore := f.communityPool()

	// x/distribution empties the fee collector every block on a real chain
	f.bank.bal[string(feeCollector())] = sdk.NewCoins()
	f.advance(t, 21) // settle epoch 2
	paidA := f.bank.bal[string(a.addr)].AmountOf(denom).Sub(aBefore)
	paidB := f.bank.bal[string(b.addr)].AmountOf(denom).Sub(bBefore)
	require.True(t, paidA.Equal(budget.MulRaw(2).QuoRaw(3)), "A paid %s", paidA)
	require.True(t, paidB.Equal(budget.QuoRaw(3)), "B paid %s", paidB)
	na, _ := f.keeper.Nodes.Get(f.ctx, a.addr)
	require.True(t, na.RewardsTotal.Equal(paidA), "G-COMM-3: rewards_total %s != paid %s", na.RewardsTotal, paidA)
	dust := f.communityPool().Sub(poolBefore)
	require.True(t, paidA.Add(paidB).Add(dust).Equal(budget), "INV-COMM-1")
	require.True(t, hasEvent(f.ctx, "epoch_settled"))

	// epoch-scoped records cleared
	n := 0
	require.NoError(t, f.keeper.TaskDone.Walk(f.ctx, nil, func(_ collectionsTriple) (bool, error) { n++; return false, nil }))
	require.Zero(t, n)
	f.checkInvariants(t)
}

// Every rejection path of SubmitTaskProof (C2–C12).
func TestSubmitTaskProofRejections(t *testing.T) {
	f := initFixture(t)
	a := f.newOperator(t, 1, nira(10_000))
	b := f.newOperator(t, 2, nira(10_000))
	startEligible(t, f, a, b)

	// C2: registered in the current epoch → not eligible yet
	late := f.newOperator(t, 3, nira(10_000))
	f.register(t, late, nira(1000))
	m := f.proofMsg(t, a, 0)
	m.Operator = late.bech
	_, err := f.msg.SubmitTaskProof(f.ctx, m)
	require.ErrorIs(t, err, types.ErrNodeNotEligible)

	// C3: wrong epoch
	m = f.proofMsg(t, a, 0)
	m.Epoch = 1
	_, err = f.msg.SubmitTaskProof(f.ctx, m)
	require.ErrorIs(t, err, types.ErrWrongEpoch)

	// C5: grinding — using a different onion layer than the one bound to this epoch
	m = f.proofMsg(t, a, 0)
	m.Reveal = a.layer(3, 2)
	_, err = f.msg.SubmitTaskProof(f.ctx, m)
	require.ErrorIs(t, err, types.ErrInvalidReveal)

	// C9: copying another node's task (tasks are per-operator)
	m = f.proofMsg(t, a, 0)
	m.Operator = b.bech
	nb, _ := f.keeper.Nodes.Get(f.ctx, b.addr)
	m.Reveal = b.layer(2, nb.OnionBaseEpoch)
	_, err = f.msg.SubmitTaskProof(f.ctx, m)
	require.ErrorIs(t, err, types.ErrWrongTask)

	// C12: tampered value / proof
	m = f.proofMsg(t, a, 0)
	if len(m.Value) > 0 {
		m.Value = []byte("forged")
	} else {
		m.Value = []byte("claim membership")
	}
	_, err = f.msg.SubmitTaskProof(f.ctx, m)
	require.ErrorIs(t, err, types.ErrInvalidProof)
	m = f.proofMsg(t, a, 0)
	m.Proof = bytes.Repeat([]byte{1}, 100)
	_, err = f.msg.SubmitTaskProof(f.ctx, m)
	require.ErrorIs(t, err, types.ErrInvalidProof)

	// C10: too large
	m = f.proofMsg(t, a, 0)
	m.Proof = make([]byte, 20_000)
	_, err = f.msg.SubmitTaskProof(f.ctx, m)
	require.ErrorIs(t, err, types.ErrProofTooLarge)

	// failed attempts above did not consume the reveal: valid proof works now
	_, err = f.msg.SubmitTaskProof(f.ctx, f.proofMsg(t, a, 0))
	require.NoError(t, err)

	// C7: duplicate task, index out of range
	_, err = f.msg.SubmitTaskProof(f.ctx, f.proofMsg(t, a, 0))
	require.ErrorIs(t, err, types.ErrTaskDone)
	m = f.proofMsg(t, a, 1)
	m.TaskIndex = 5
	_, err = f.msg.SubmitTaskProof(f.ctx, m)
	require.ErrorIs(t, err, types.ErrTaskDone)

	// C8: epoch capacity
	p, _ := f.keeper.Params.Get(f.ctx)
	p.MaxProofsPerEpoch = 1
	require.NoError(t, f.keeper.Params.Set(f.ctx, p))
	_, err = f.msg.SubmitTaskProof(f.ctx, f.proofMsg(t, b, 0))
	require.ErrorIs(t, err, types.ErrEpochFull)
	f.checkInvariants(t)
}

// C6: selection is enforced. With target 1, a tiny bond next to a huge one is not selected.
func TestNotSelected(t *testing.T) {
	f := initFixture(t)
	p := testParams()
	p.TargetCommittee = 1
	p.TasksPerSelection = 1
	require.NoError(t, f.keeper.Params.Set(f.ctx, p))
	small := f.newOperator(t, 1, nira(10_000))
	whale := f.newOperator(t, 2, nira(10_000_000))
	f.advance(t, 1)
	f.register(t, small, nira(1000))
	f.register(t, whale, nira(9_000_000))
	f.advance(t, 11)
	_, err := f.msg.SubmitTaskProof(f.ctx, f.proofMsg(t, small, 0))
	require.ErrorIs(t, err, types.ErrNotSelected)
	// the whale (p ≈ 99.99 %) is selected
	_, err = f.msg.SubmitTaskProof(f.ctx, f.proofMsg(t, whale, 0))
	require.NoError(t, err)
}

// Offline home node: no penalty; after a long gap it rotates its onion and resumes.
func TestOfflineNodeNoPenaltyAndRejoin(t *testing.T) {
	f := initFixture(t)
	p := testParams()
	p.MaxRevealDistance = 2
	require.NoError(t, f.keeper.Params.Set(f.ctx, p))
	a := f.newOperator(t, 1, nira(10_000))
	startEligible(t, f, a)
	f.advance(t, 61) // offline through epochs 2..6, now epoch 7
	n, _ := f.keeper.Nodes.Get(f.ctx, a.addr)
	require.True(t, n.Bond.Equal(nira(1000)), "never slashed for absence")
	_, err := f.msg.SubmitTaskProof(f.ctx, f.proofMsg(t, a, 0))
	require.ErrorIs(t, err, types.ErrInvalidReveal) // gap 6 > max distance 2

	a.secret = []byte("new secret")
	_, err = f.msg.RotateOnion(f.ctx, &types.MsgRotateOnion{Operator: a.bech, OnionTip: a.tip(), OnionLength: a.length})
	require.NoError(t, err)
	_, err = f.msg.SubmitTaskProof(f.ctx, f.proofMsg(t, a, 0))
	require.ErrorIs(t, err, types.ErrNodeNotEligible) // new onion valid from next epoch only
	f.advance(t, 71)
	_, err = f.msg.SubmitTaskProof(f.ctx, f.proofMsg(t, a, 0))
	require.NoError(t, err)
	f.checkInvariants(t)
}

func TestDeregisterAndUnbond(t *testing.T) {
	f := initFixture(t)
	a := f.newOperator(t, 1, nira(10_000))
	startEligible(t, f, a)
	_, err := f.msg.SubmitTaskProof(f.ctx, f.proofMsg(t, a, 0))
	require.NoError(t, err)
	before := f.bank.bal[string(a.addr)].AmountOf(denom)

	res, err := f.msg.DeregisterNode(f.ctx, &types.MsgDeregisterNode{Operator: a.bech})
	require.NoError(t, err)
	require.Equal(t, int64(11+5), res.UnbondCompleteHeight)
	f.checkInvariants(t)
	_, err = f.msg.DeregisterNode(f.ctx, &types.MsgDeregisterNode{Operator: a.bech})
	require.ErrorIs(t, err, types.ErrNodeNotActive)
	_, err = f.msg.SubmitTaskProof(f.ctx, f.proofMsg(t, a, 1))
	require.ErrorIs(t, err, types.ErrNodeNotEligible)

	f.advance(t, 16)
	_, err = f.keeper.Nodes.Get(f.ctx, a.addr)
	require.Error(t, err, "node removed after unbonding")
	require.True(t, f.bank.bal[string(a.addr)].AmountOf(denom).Sub(before).Equal(nira(1000)), "bond refunded")
	f.checkInvariants(t)
}

func TestDivergenceAlerts(t *testing.T) {
	f := initFixture(t)
	a := f.newOperator(t, 1, nira(10_000))
	startEligible(t, f, a)
	h := int64(5)
	committed, err := f.keeper.AppHashes.Get(f.ctx, h)
	require.NoError(t, err)

	_, err = f.msg.ReportDivergence(f.ctx, &types.MsgReportDivergence{Operator: a.bech, Height: h, ClaimedAppHash: committed})
	require.ErrorIs(t, err, types.ErrNoDivergence)
	wrong := bytes.Repeat([]byte{7}, 32)
	res, err := f.msg.ReportDivergence(f.ctx, &types.MsgReportDivergence{Operator: a.bech, Height: h, ClaimedAppHash: wrong})
	require.NoError(t, err)
	require.Equal(t, uint64(1), res.AlertId)
	require.True(t, hasEvent(f.ctx, "divergence_threshold"), "sole node holds 100 % of bond")
	_, err = f.msg.ReportDivergence(f.ctx, &types.MsgReportDivergence{Operator: a.bech, Height: h, ClaimedAppHash: wrong})
	require.ErrorIs(t, err, types.ErrDuplicateAlert)
	f.checkInvariants(t)

	// only governance resolves
	_, err = f.msg.ResolveAlert(f.ctx, &types.MsgResolveAlert{Authority: a.bech, AlertId: 1})
	require.ErrorIs(t, err, types.ErrInvalidSigner)
	pool := f.communityPool()
	_, err = f.msg.ResolveAlert(f.ctx, &types.MsgResolveAlert{Authority: f.gov, AlertId: 1, Valid: false})
	require.NoError(t, err)
	// G-COMM-1: false alert = provable misconduct → deposit (100) + 10 % of the 1000 NIRA bond (100) to the pool
	require.True(t, f.communityPool().Sub(pool).Equal(nira(200)), "pool +%s", f.communityPool().Sub(pool))
	n, _ := f.keeper.Nodes.Get(f.ctx, a.addr)
	require.True(t, n.Bond.Equal(nira(900)), "bond %s", n.Bond)
	require.True(t, n.SlashedTotal.Equal(nira(100)))
	require.True(t, hasEvent(f.ctx, "community_node_slashed"))
	f.checkInvariants(t)
	_, err = f.msg.ResolveAlert(f.ctx, &types.MsgResolveAlert{Authority: f.gov, AlertId: 1, Valid: true})
	require.ErrorIs(t, err, types.ErrAlertNotOpen)

	// expiry refunds
	_, err = f.msg.ReportDivergence(f.ctx, &types.MsgReportDivergence{Operator: a.bech, Height: h + 1, ClaimedAppHash: wrong})
	require.NoError(t, err)
	bal := f.bank.bal[string(a.addr)].AmountOf(denom)
	f.advance(t, 11+100)
	al, err := f.query.Alert(f.ctx, &types.QueryAlertRequest{Id: 2})
	require.NoError(t, err)
	require.Equal(t, types.ALERT_STATUS_EXPIRED, al.Alert.Status)
	require.True(t, f.bank.bal[string(a.addr)].AmountOf(denom).Sub(bal).Equal(nira(100)))
	f.checkInvariants(t)
}

// INV-COMM-8: a failing hook step never halts the chain; its state change is discarded.
func TestHookFailureIsolated(t *testing.T) {
	f := initFixture(t)
	a := f.newOperator(t, 1, nira(10_000))
	startEligible(t, f, a)
	_, err := f.msg.SubmitTaskProof(f.ctx, f.proofMsg(t, a, 0))
	require.NoError(t, err)
	f.bank.bal[string(feeCollector())] = sdk.NewCoins(sdk.NewCoin(denom, nira(1000)))
	f.advance(t, 20)

	f.bank.failSends = true
	f.advance(t, 21) // settlement fails → BeginBlock still returns nil
	require.True(t, hasEvent(f.ctx, "comm_hook_error"))
	ep, _ := f.keeper.Epoch.Get(f.ctx)
	require.Equal(t, uint64(2), ep.Number, "epoch not advanced, state discarded")

	f.bank.failSends = false
	f.advance(t, 22) // retried next block
	ep, _ = f.keeper.Epoch.Get(f.ctx)
	require.Equal(t, uint64(3), ep.Number)
	f.checkInvariants(t)
}

func TestGenesisRoundTrip(t *testing.T) {
	f := initFixture(t)
	a := f.newOperator(t, 1, nira(10_000))
	b := f.newOperator(t, 2, nira(10_000))
	startEligible(t, f, a, b)
	_, err := f.msg.DeregisterNode(f.ctx, &types.MsgDeregisterNode{Operator: b.bech})
	require.NoError(t, err)
	_, err = f.msg.ReportDivergence(f.ctx, &types.MsgReportDivergence{Operator: a.bech, Height: 5, ClaimedAppHash: bytes.Repeat([]byte{9}, 32)})
	require.NoError(t, err)

	gs, err := f.keeper.ExportGenesis(f.ctx)
	require.NoError(t, err)
	require.NoError(t, gs.Validate())

	g := initFixture(t)
	require.NoError(t, g.keeper.InitGenesis(g.ctx, *gs))
	gs2, err := g.keeper.ExportGenesis(g.ctx)
	require.NoError(t, err)
	require.Equal(t, gs, gs2)
	total, _ := g.keeper.TotalActiveBond.Get(g.ctx)
	require.True(t, total.Equal(nira(1000)))
	res, err := g.msg.ReportDivergence(g.ctx, &types.MsgReportDivergence{Operator: a.bech, Height: 6, ClaimedAppHash: bytes.Repeat([]byte{9}, 32)})
	_ = res
	require.Error(t, err) // app-hash ring is not exported → no divergence claim possible yet
}

// Randomised operations over many epochs; invariants checked every block.
func TestINV_COMM_Randomised(t *testing.T) {
	f := initFixture(t)
	r := rand.New(rand.NewSource(42))
	var ops []*operator
	for i := 0; i < 8; i++ {
		ops = append(ops, f.newOperator(t, byte(i+1), nira(100_000)))
	}
	f.advance(t, 1)
	proven := 0
	for h := int64(2); h <= 120; h++ {
		f.bank.bal[string(feeCollector())] = sdk.NewCoins(sdk.NewCoin(denom, nira(int64(r.Intn(1000)))))
		f.advance(t, h)
		for _, o := range ops {
			n, err := f.keeper.Nodes.Get(f.ctx, o.addr)
			switch {
			case err != nil:
				if r.Intn(4) == 0 {
					o.secret = []byte{byte(h), byte(r.Intn(255))}
					f.register(t, o, nira(int64(1000+r.Intn(5000))))
				}
			case n.Status == types.NODE_STATUS_ACTIVE && r.Intn(30) == 0:
				_, err := f.msg.DeregisterNode(f.ctx, &types.MsgDeregisterNode{Operator: o.bech})
				require.NoError(t, err)
			case n.Status == types.NODE_STATUS_ACTIVE && r.Intn(3) == 0:
				ep, _ := f.keeper.Epoch.Get(f.ctx)
				if ep.Number < n.OnionBaseEpoch || n.RegisteredEpoch >= ep.Number {
					continue
				}
				if _, err := f.msg.SubmitTaskProof(f.ctx, f.proofMsg(t, o, uint32(r.Intn(2)))); err == nil {
					proven++
				} else {
					require.ErrorIs(t, err, types.ErrTaskDone)
				}
			}
		}
		f.checkInvariants(t)
	}
	require.Greater(t, proven, 20)
}

var _ = math.ZeroInt

// G-SEC-1: the onion hash chain is gas-metered before it is computed.
// Identical failing txs at reveal distance 3 and 13 differ by ≥ 10·RevealGasPerHash.
func TestRevealIsGasMetered(t *testing.T) {
	gasAt := func(height int64) uint64 {
		f := initFixture(t)
		a := f.newOperator(t, 1, nira(10_000))
		startEligible(t, f, a)
		f.advance(t, height)
		m := f.proofMsg(t, a, 0)
		m.Proof = []byte{0xff} // reveal passes, proof decoding fails
		before := f.ctx.GasMeter().GasConsumed()
		_, err := f.msg.SubmitTaskProof(f.ctx, m)
		require.ErrorIs(t, err, types.ErrInvalidProof)
		return f.ctx.GasMeter().GasConsumed() - before
	}
	near := gasAt(41) // epoch 4, anchor epoch 1 → d = 3
	far := gasAt(141) // epoch 14 → d = 13
	require.GreaterOrEqual(t, far-near, 10*types.RevealGasPerHash, "near %d far %d", near, far)

	// beyond max_reveal_distance: rejected before hashing
	f := initFixture(t)
	a := f.newOperator(t, 1, nira(10_000))
	startEligible(t, f, a)
	f.advance(t, 141)
	p, _ := f.keeper.Params.Get(f.ctx)
	p.MaxRevealDistance = 2
	require.NoError(t, f.keeper.Params.Set(f.ctx, p))
	_, err := f.msg.SubmitTaskProof(f.ctx, f.proofMsg(t, a, 0))
	require.ErrorIs(t, err, types.ErrInvalidReveal)
}
