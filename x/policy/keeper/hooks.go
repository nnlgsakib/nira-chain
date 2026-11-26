package keeper

import (
	"context"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

var _ stakingtypes.StakingHooks = Hooks{}

// Hooks implements x/staking hooks for the voting-power cap (ADR-0004).
type Hooks struct{ k Keeper }

// Hooks returns the staking hooks of the module.
func (k Keeper) Hooks() Hooks { return Hooks{k} }

func (h Hooks) BeforeDelegationCreated(ctx context.Context, _ sdk.AccAddress, valAddr sdk.ValAddress) error {
	return h.k.recordPreTokens(ctx, valAddr)
}

func (h Hooks) BeforeDelegationSharesModified(ctx context.Context, _ sdk.AccAddress, valAddr sdk.ValAddress) error {
	return h.k.recordPreTokens(ctx, valAddr)
}

func (h Hooks) AfterDelegationModified(ctx context.Context, _ sdk.AccAddress, valAddr sdk.ValAddress) error {
	pre, ok, err := h.k.popPreTokens(ctx, valAddr)
	if err != nil {
		return err
	}
	val, err := h.k.stakingKeeper.GetValidator(ctx, valAddr)
	if err != nil {
		return err
	}
	// Only stake increases are capped; unbonding must never be blocked.
	if ok && !val.Tokens.GT(pre) {
		return nil
	}
	return h.k.CheckValidatorPowerCap(ctx, val)
}

func (h Hooks) BeforeDelegationRemoved(ctx context.Context, _ sdk.AccAddress, valAddr sdk.ValAddress) error {
	_, _, err := h.k.popPreTokens(ctx, valAddr)
	return err
}

// AfterValidatorCreated enforces the chain-wide minimum self-bond (G-STAKE-1).
// x/staking calls it after storing the validator (with its
// min_self_delegation) and before the initial self-delegation, for every
// creation path: Cosmos tx, authz, gentx and the EVM staking precompile.
func (h Hooks) AfterValidatorCreated(ctx context.Context, valAddr sdk.ValAddress) error {
	return h.k.CheckMinSelfBond(ctx, valAddr)
}
func (Hooks) BeforeValidatorModified(context.Context, sdk.ValAddress) error { return nil }
func (Hooks) AfterValidatorRemoved(context.Context, sdk.ConsAddress, sdk.ValAddress) error {
	return nil
}
func (Hooks) AfterValidatorBonded(context.Context, sdk.ConsAddress, sdk.ValAddress) error {
	return nil
}
func (Hooks) AfterValidatorBeginUnbonding(context.Context, sdk.ConsAddress, sdk.ValAddress) error {
	return nil
}
func (Hooks) BeforeValidatorSlashed(context.Context, sdk.ValAddress, math.LegacyDec) error {
	return nil
}
func (Hooks) AfterUnbondingInitiated(context.Context, uint64) error { return nil }
