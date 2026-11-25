package types

// DONTCOVER

import "cosmossdk.io/errors"

// x/community sentinel errors (docs/spec/community.md §6). Codes are stable.
var (
	ErrInvalidSigner    = errors.Register(ModuleName, 1100, "invalid signer")
	ErrNodeExists       = errors.Register(ModuleName, 1101, "community node already registered")
	ErrInsufficientBond = errors.Register(ModuleName, 1102, "bond below minimum or wrong denom")
	ErrInvalidOnion     = errors.Register(ModuleName, 1103, "invalid onion commitment")
	ErrInvalidMoniker   = errors.Register(ModuleName, 1104, "invalid moniker")
	ErrNodeNotFound     = errors.Register(ModuleName, 1105, "community node not found")
	ErrNodeNotActive    = errors.Register(ModuleName, 1106, "community node not active")
	ErrNodeNotEligible  = errors.Register(ModuleName, 1107, "community node not eligible in this epoch")
	ErrWrongEpoch       = errors.Register(ModuleName, 1108, "wrong epoch")
	ErrInvalidReveal    = errors.Register(ModuleName, 1109, "invalid onion reveal")
	ErrNotSelected      = errors.Register(ModuleName, 1110, "node not selected in this epoch")
	ErrTaskDone         = errors.Register(ModuleName, 1111, "task index invalid or already proven")
	ErrEpochFull        = errors.Register(ModuleName, 1112, "epoch proof capacity reached")
	ErrWrongTask        = errors.Register(ModuleName, 1113, "proof does not match assigned task")
	ErrProofTooLarge    = errors.Register(ModuleName, 1114, "proof or value too large")
	ErrInvalidProof     = errors.Register(ModuleName, 1115, "state proof verification failed")
	ErrNoDivergence     = errors.Register(ModuleName, 1116, "claimed app hash equals committed app hash or height unknown")
	ErrDuplicateAlert   = errors.Register(ModuleName, 1117, "alert already reported for this height")
	ErrTooManyAlerts    = errors.Register(ModuleName, 1118, "too many open alerts")
	ErrAlertNotOpen     = errors.Register(ModuleName, 1119, "alert not found or not open")
)
