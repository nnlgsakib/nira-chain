package types

// DONTCOVER

import (
	"cosmossdk.io/errors"
)

// x/policy module sentinel errors. Codes are stable: never renumber.
var (
	ErrInvalidSigner = errors.Register(ModuleName, 1100, "expected gov account as only signer for proposal message")
	// ErrValidatorPowerCap: a delegation would push a validator above P-STAKE-max_validator_power_bps (ADR-0004).
	ErrValidatorPowerCap = errors.Register(ModuleName, 1101, "delegation exceeds validator voting-power cap")
	// ErrMinSelfBond: a new validator's min_self_delegation is below P-STAKE-min_self_bond (G-STAKE-1).
	ErrMinSelfBond = errors.Register(ModuleName, 1102, "validator min_self_delegation below chain minimum self-bond")
)
