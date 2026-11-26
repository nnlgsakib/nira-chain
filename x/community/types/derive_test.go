package types_test

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"cosmossdk.io/math"
	"github.com/stretchr/testify/require"

	"nira-chain/x/community/types"
)

func mustHex(s string) []byte {
	b, err := hex.DecodeString(s)
	if err != nil {
		panic(err)
	}
	return b
}
func hn(v []byte, n int) []byte {
	for i := 0; i < n; i++ {
		s := sha256.Sum256(v)
		v = s[:]
	}
	return v
}

// Test vectors from docs/spec/community.md §13.
func TestSpecVectors(t *testing.T) {
	s := make([]byte, 32)
	tip := hn(s, 5)
	require.Equal(t, "376da11fe3ab3d0eaaddb418ccb49b5426d5c2504f526f7766580f6e45984e3b", hex.EncodeToString(tip))
	vb := hn(s, 4)
	require.Equal(t, "fe15c0d3ebe314fad720a08b839a004c2e6386f5aecc19ec74807d1920cb6aeb", hex.EncodeToString(vb))

	hdr := make([]byte, 32)
	for i := range hdr {
		hdr[i] = 0x11
	}
	seed := types.NextSeed(nil, hdr, 7)
	require.Equal(t, "7921fca6768af1463d28998ff0c9229468f530a5edbca62880b0fb4eb9718c96", hex.EncodeToString(seed))

	op := mustHex("963ebdf2e1f8db8707d05fc75bfeffba1b5bac17")
	tk := types.Ticket(seed, vb, op)
	require.Equal(t, "6476c593104efe73168ef013a1c62c283c395cc17a72255de2c1e43c99ac216b", hex.EncodeToString(tk))
	require.False(t, types.IsSelected(tk, math.NewIntWithDecimal(1, 24), math.NewIntWithDecimal(64, 24), 8))
	require.True(t, types.IsSelected(tk, math.NewIntWithDecimal(64, 24), math.NewIntWithDecimal(64, 24), 1))

	stores := []string{"s0", "s1", "s2", "s3"}
	t0, ok := types.DeriveTask(seed, op, 0, 1000, 100, stores)
	require.True(t, ok)
	require.Equal(t, int64(1000-1-20), t0.Height)
	require.Equal(t, "s1", t0.Store)
	require.Equal(t, "57bf8f4838ffa79921a2b8306ae1755fcc543beb7295377f7784bb6128ec76d6", hex.EncodeToString(t0.Key))
	t1, _ := types.DeriveTask(seed, op, 1, 1000, 100, stores)
	require.Equal(t, int64(1000-1-41), t1.Height)
	require.Equal(t, "s3", t1.Store)
}

// INV-COMM-9: reveals must hash forward to the anchor, distance-bounded, epoch-bound.
func TestINV_COMM_9_Reveal(t *testing.T) {
	s := make([]byte, 32)
	tip := hn(s, 10)
	base := uint64(5) // anchor epoch = 4
	// epoch 5 → v = H^9(s); epoch 7 (skipping 5,6) → v = H^7(s)
	require.NoError(t, types.VerifyReveal(hn(s, 9), tip, 4, 5, base, 10, 100))
	require.NoError(t, types.VerifyReveal(hn(s, 7), tip, 4, 7, base, 10, 100))
	// wrong layer for epoch (grinding attempt: reuse layer for a later epoch)
	require.Error(t, types.VerifyReveal(hn(s, 9), tip, 4, 6, base, 10, 100))
	// distance bound
	require.Error(t, types.VerifyReveal(hn(s, 7), tip, 4, 7, base, 10, 2))
	// onion exhausted: epoch base+10 needs H^-1
	require.Error(t, types.VerifyReveal(s, tip, 4, 15, base, 10, 100))
	// before base / not after anchor
	require.Error(t, types.VerifyReveal(hn(s, 9), tip, 4, 4, base, 10, 100))
	require.Error(t, types.VerifyReveal(hn(s, 9)[:31], tip, 4, 5, base, 10, 100))
}

// Selection frequency ≈ target · bond / eligible; splitting a bond doesn't help (sybil neutrality).
func TestSelectionProportionalToBond(t *testing.T) {
	eligible := math.NewInt(1_000_000)
	count := func(bond math.Int, n int, salt byte) int {
		c := 0
		for i := 0; i < n; i++ {
			tk := types.Ticket([]byte{salt, byte(i), byte(i >> 8)}, []byte{byte(i >> 16)}, []byte("op"))
			if types.IsSelected(tk, bond, eligible, 100) {
				c++
			}
		}
		return c
	}
	// p = 100 · 1000 / 1e6 = 10 %
	got := count(math.NewInt(1000), 20000, 1)
	require.InDelta(t, 2000, got, 200)
	// two identities with 500 each: 5 % each → same total expectation
	a := count(math.NewInt(500), 20000, 2)
	b := count(math.NewInt(500), 20000, 3)
	require.InDelta(t, 2000, a+b, 250)
}

func TestParams(t *testing.T) {
	require.NoError(t, types.DefaultParams().Validate())
	p := types.DefaultParams()
	p.TasksPerSelection = 16
	p.TargetCommittee = 100
	require.Error(t, p.Validate())
	p = types.DefaultParams()
	p.SampleStores = []string{"bank", "bank"}
	require.Error(t, p.Validate())
	p = types.DefaultParams()
	p.RewardShareBps = 3001
	require.Error(t, p.Validate())
	require.Error(t, types.Params{}.Validate())
}
