package keeper_test

import (
	"math/rand"
	"testing"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	"github.com/stretchr/testify/require"

	"nira-chain/x/nirachain/types"
)

var nira = func(n int64) math.Int { return math.NewIntWithDecimal(n, 18) }

func valAddr(i int) sdk.ValAddress {
	b := make([]byte, 20)
	b[19] = byte(i + 1)
	return sdk.ValAddress(b)
}

// setupValidators bonds n validators with `each` NIRA and sets params so the
// cap is active (10 %, activation 1 NIRA, min validators 10).
func setupValidators(t *testing.T, f *fixture, n int, each int64) {
	t.Helper()
	for i := 0; i < n; i++ {
		f.staking.set(valAddr(i), nira(each), true)
	}
	require.NoError(t, f.keeper.Params.Set(f.ctx, types.NewParams(1000, nira(1), 10)))
}

// delegate simulates x/staking Delegate: Before hook, tokens grow, After hook.
func delegate(f *fixture, v sdk.ValAddress, amt math.Int) error {
	h := f.keeper.Hooks()
	if err := h.BeforeDelegationSharesModified(f.ctx, nil, v); err != nil {
		return err
	}
	val, _ := f.staking.GetValidator(f.ctx, v)
	f.staking.set(v, val.Tokens.Add(amt), val.IsBonded())
	return h.AfterDelegationModified(f.ctx, nil, v)
}

// undelegate simulates x/staking Unbond: Before hook, After hook fires while
// tokens are unchanged, then tokens shrink.
func undelegate(f *fixture, v sdk.ValAddress, amt math.Int) error {
	h := f.keeper.Hooks()
	if err := h.BeforeDelegationSharesModified(f.ctx, nil, v); err != nil {
		return err
	}
	if err := h.AfterDelegationModified(f.ctx, nil, v); err != nil {
		return err
	}
	val, _ := f.staking.GetValidator(f.ctx, v)
	f.staking.set(v, val.Tokens.Sub(amt), val.IsBonded())
	return nil
}

func share(f *fixture, v sdk.ValAddress) math.LegacyDec {
	val, _ := f.staking.GetValidator(f.ctx, v)
	total, _ := f.staking.TotalBondedTokens(f.ctx)
	return math.LegacyNewDecFromInt(val.Tokens).QuoInt(total)
}

// INV-STAKE-CAP-1: an accepted delegation never leaves the validator above the cap.
func TestINV_STAKE_CAP_1_DelegationAboveCapRejected(t *testing.T) {
	f := initFixture(t)
	setupValidators(t, f, 10, 100) // each 10 % exactly

	// any increase pushes v0 above 10 %
	err := delegate(f, valAddr(0), nira(1))
	require.ErrorIs(t, err, types.ErrValidatorPowerCap)

	// adding 10 validators halves everyone's share → delegation fits
	for i := 10; i < 20; i++ {
		f.staking.set(valAddr(i), nira(100), true)
	}
	require.NoError(t, delegate(f, valAddr(0), nira(100))) // 200 / 2100 ≈ 9.5 %
	require.True(t, share(f, valAddr(0)).LTE(math.LegacyNewDecWithPrec(10, 2)))
}

// Exact boundary: share == cap is allowed, share > cap by 1 base unit is not.
func TestPowerCapBoundaryExact(t *testing.T) {
	f := initFixture(t)
	setupValidators(t, f, 10, 0)
	for i := 1; i < 10; i++ {
		f.staking.set(valAddr(i), nira(100), true)
	}
	// v0 with x tokens: x*10000 <= 1000*(900+x) ⇔ x <= 100 NIRA
	require.NoError(t, delegate(f, valAddr(0), nira(100)))
	require.ErrorIs(t, delegate(f, valAddr(0), math.OneInt()), types.ErrValidatorPowerCap)
}

// INV-STAKE-CAP-2: undelegation is never blocked, even above the cap.
func TestINV_STAKE_CAP_2_UndelegationNeverBlocked(t *testing.T) {
	f := initFixture(t)
	setupValidators(t, f, 10, 100)
	f.staking.set(valAddr(0), nira(500), true) // 500/1400 ≈ 36 % — far above cap
	require.NoError(t, undelegate(f, valAddr(0), nira(10)))
	require.NoError(t, undelegate(f, valAddr(0), nira(400)))
}

// Unbonded (not yet active) validators are counted as if they join the set.
func TestPowerCapUnbondedValidatorCounted(t *testing.T) {
	f := initFixture(t)
	setupValidators(t, f, 10, 100)
	f.staking.set(valAddr(20), nira(0), false)
	// x <= 0.1 * (1000 + x)  ⇔  x <= 111.11 NIRA
	require.NoError(t, delegate(f, valAddr(20), nira(111)))
	require.ErrorIs(t, delegate(f, valAddr(20), nira(1)), types.ErrValidatorPowerCap)
}

// Cap inactive below activation thresholds (genesis bootstrapping).
func TestPowerCapInactiveBelowActivation(t *testing.T) {
	f := initFixture(t)
	f.staking.set(valAddr(0), nira(100), true)
	// default params: activation 5M NIRA and 15 validators
	require.NoError(t, delegate(f, valAddr(0), nira(1_000_000)))

	// enough stake but too few validators
	require.NoError(t, f.keeper.Params.Set(f.ctx, types.NewParams(1000, nira(1), 15)))
	for i := 1; i < 14; i++ {
		f.staking.set(valAddr(i), nira(1), true)
	}
	require.NoError(t, delegate(f, valAddr(0), nira(1)))
}

// Randomised sequences: after every accepted delegation the validator's
// share is ≤ cap; undelegations always succeed.
func TestINV_STAKE_CAP_Randomised(t *testing.T) {
	f := initFixture(t)
	setupValidators(t, f, 20, 50)
	r := rand.New(rand.NewSource(7))
	cap := math.LegacyNewDecWithPrec(10, 2)
	accepted, rejected := 0, 0
	for i := 0; i < 2000; i++ {
		v := valAddr(r.Intn(20))
		amt := nira(int64(r.Intn(80) + 1))
		if r.Intn(3) == 0 {
			val, _ := f.staking.GetValidator(f.ctx, v)
			if val.Tokens.GT(amt) {
				require.NoError(t, undelegate(f, v, amt))
			}
			continue
		}
		before, _ := f.staking.GetValidator(f.ctx, v)
		if err := delegate(f, v, amt); err != nil {
			require.ErrorIs(t, err, types.ErrValidatorPowerCap)
			// tx would revert: restore state as the SDK's cache-ctx would
			f.staking.set(v, before.Tokens, true)
			rejected++
			continue
		}
		accepted++
		require.True(t, share(f, v).LTE(cap), "share %s > cap", share(f, v))
	}
	require.Positive(t, accepted)
	require.Positive(t, rejected)
}

func TestParamsValidate(t *testing.T) {
	require.NoError(t, types.DefaultParams().Validate())
	cases := []types.Params{
		types.NewParams(299, nira(1), 34),
		types.NewParams(3334, nira(1), 4),
		types.NewParams(1000, nira(1), 9), // 10 % needs ≥ 10 validators
		types.NewParams(1000, math.NewInt(-1), 10),
		types.NewParams(1000, nira(1), 1001),
	}
	for i, p := range cases {
		require.Error(t, p.Validate(), "case %d", i)
	}
}

// INV-STAKE-CAP-3: no scratch entry survives a delegate / undelegate / full removal.
func TestINV_STAKE_CAP_3_NoResidue(t *testing.T) {
	f := initFixture(t)
	setupValidators(t, f, 20, 100)
	v := valAddr(3)
	require.NoError(t, delegate(f, v, nira(5)))
	require.NoError(t, undelegate(f, v, nira(5)))
	// full removal path: Before shares modified, then BeforeDelegationRemoved
	h := f.keeper.Hooks()
	require.NoError(t, h.BeforeDelegationSharesModified(f.ctx, nil, v))
	require.NoError(t, h.BeforeDelegationRemoved(f.ctx, nil, v))
	// rejected delegation: the tx reverts in the SDK, but the hook itself must still clean up
	f.staking.vals = map[string]stakingtypes.Validator{}
	setupValidators(t, f, 10, 100)
	require.Error(t, delegate(f, valAddr(0), nira(1)))

	n, err := f.keeper.PreDelegationEntries(f.ctx)
	require.NoError(t, err)
	require.Zero(t, n)
}
