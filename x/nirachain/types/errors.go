package types

// DONTCOVER

import (
	"cosmossdk.io/errors"
)

// x/nirachain module sentinel errors. Codes are stable: never renumber.
var (
	ErrInvalidSigner = errors.Register(ModuleName, 1100, "expected gov account as only signer for proposal message")
	// ErrValidatorPowerCap: a delegation would push a validator above P-STAKE-max_validator_power_bps (ADR-0004).
	ErrValidatorPowerCap = errors.Register(ModuleName, 1101, "delegation exceeds validator voting-power cap")
)
