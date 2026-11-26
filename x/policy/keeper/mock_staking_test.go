package keeper_test

import (
	"context"
	"sort"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

// mockStakingKeeper is a minimal in-memory staking view: validators with
// tokens and bonded status. TotalBondedTokens = Σ tokens of bonded validators.
type mockStakingKeeper struct {
	vals map[string]stakingtypes.Validator
}

func newMockStakingKeeper() *mockStakingKeeper {
	return &mockStakingKeeper{vals: map[string]stakingtypes.Validator{}}
}

func (m *mockStakingKeeper) set(addr sdk.ValAddress, tokens math.Int, bonded bool) {
	status := stakingtypes.Unbonded
	if bonded {
		status = stakingtypes.Bonded
	}
	m.vals[string(addr)] = stakingtypes.Validator{OperatorAddress: addr.String(), Tokens: tokens, Status: status}
}

// setMin registers a just-created validator (no tokens yet) with the given
// min_self_delegation, as x/staking stores it before AfterValidatorCreated.
func (m *mockStakingKeeper) setMin(addr sdk.ValAddress, minSelf math.Int) {
	m.vals[string(addr)] = stakingtypes.Validator{
		OperatorAddress: addr.String(), Tokens: math.ZeroInt(), Status: stakingtypes.Unbonded, MinSelfDelegation: minSelf,
	}
}

func (m *mockStakingKeeper) GetValidator(_ context.Context, addr sdk.ValAddress) (stakingtypes.Validator, error) {
	v, ok := m.vals[string(addr)]
	if !ok {
		return v, stakingtypes.ErrNoValidatorFound
	}
	return v, nil
}

func (m *mockStakingKeeper) TotalBondedTokens(context.Context) (math.Int, error) {
	t := math.ZeroInt()
	for _, v := range m.vals {
		if v.IsBonded() {
			t = t.Add(v.Tokens)
		}
	}
	return t, nil
}

func (m *mockStakingKeeper) GetLastValidators(context.Context) ([]stakingtypes.Validator, error) {
	var out []stakingtypes.Validator
	for _, v := range m.vals {
		if v.IsBonded() {
			out = append(out, v)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].OperatorAddress < out[j].OperatorAddress })
	return out, nil
}
