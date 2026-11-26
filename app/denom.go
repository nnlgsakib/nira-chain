package app

// Native token constants (ADR-0005). The token has 18 decimals so that the
// same base unit is used by Cosmos modules and the EVM (1 nira = 10^18 anira).
const (
	// BaseDenom is the on-chain base unit (atto-nira).
	BaseDenom = "anira"
	// DisplayDenom is the human-readable unit.
	DisplayDenom = "nira"
	// EVMChainID is the EIP-155 chain ID (proposed, CHAIN.md Q2 — confirm
	// against chainlist.org before any public network).
	EVMChainID uint64 = 47201
)
