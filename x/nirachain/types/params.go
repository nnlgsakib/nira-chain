package types

import (
	"fmt"

	"cosmossdk.io/math"
)

// Parameter bounds (ADR-0004, docs/spec/parameters.md).
const (
	MinValidatorPowerBps uint32 = 300  // 3 %
	MaxValidatorPowerBps uint32 = 3333 // just under 1/3: one validator can never halt the chain alone
	BpsDenominator       uint32 = 10_000

	MaxCapActivationMinValidators uint32 = 1000

	DefaultMaxValidatorPowerBps       uint32 = 1000 // 10 %
	DefaultCapActivationMinValidators uint32 = 15
)

// DefaultCapActivationBonded is 5,000,000 NIRA (18 decimals). (proposed)
var DefaultCapActivationBonded = math.NewIntWithDecimal(5_000_000, 18)

// NewParams creates a new Params instance.
func NewParams(maxBps uint32, activationBonded math.Int, activationMinVals uint32) Params {
	return Params{
		MaxValidatorPowerBps:       maxBps,
		CapActivationBonded:        activationBonded,
		CapActivationMinValidators: activationMinVals,
	}
}

// DefaultParams returns a default set of parameters.
func DefaultParams() Params {
	return NewParams(DefaultMaxValidatorPowerBps, DefaultCapActivationBonded, DefaultCapActivationMinValidators)
}

// MinValidatorsForCap is the smallest number of validators for which every
// validator can stay at or below the cap: ceil(10000 / bps).
func MinValidatorsForCap(bps uint32) uint32 {
	return (BpsDenominator + bps - 1) / bps
}

// Validate validates the set of params.
func (p Params) Validate() error {
	if p.MaxValidatorPowerBps < MinValidatorPowerBps || p.MaxValidatorPowerBps > MaxValidatorPowerBps {
		return fmt.Errorf("max_validator_power_bps must be in [%d, %d], got %d",
			MinValidatorPowerBps, MaxValidatorPowerBps, p.MaxValidatorPowerBps)
	}
	if p.CapActivationBonded.IsNil() || p.CapActivationBonded.IsNegative() {
		return fmt.Errorf("cap_activation_bonded must be non-negative")
	}
	minVals := MinValidatorsForCap(p.MaxValidatorPowerBps)
	if p.CapActivationMinValidators < minVals || p.CapActivationMinValidators > MaxCapActivationMinValidators {
		return fmt.Errorf("cap_activation_min_validators must be in [%d, %d] for bps %d, got %d",
			minVals, MaxCapActivationMinValidators, p.MaxValidatorPowerBps, p.CapActivationMinValidators)
	}
	return nil
}
