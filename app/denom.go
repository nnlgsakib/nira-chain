package app

import (
	"fmt"
	"regexp"
	"strconv"
)

// Native token constants (ADR-0005). The token has 18 decimals so that the
// same base unit is used by Cosmos modules and the EVM (1 nira = 10^18 anira).
const (
	// BaseDenom is the on-chain base unit (atto-nira).
	BaseDenom = "anira"
	// DisplayDenom is the human-readable unit.
	DisplayDenom = "nira"
)

// EIP-155 chain IDs per network (owner decision 2026-11-26). The Cosmos chain-id
// carries the EVM chain ID: nira_<evm-chain-id>-<revision>, e.g. nira_91000-1.
const (
	MainnetEVMChainID uint64 = 91000
	TestnetEVMChainID uint64 = 92000
	LocalEVMChainID   uint64 = 93000

	// EVMChainID is the default used before a chain-id is known (CLI config
	// templates, unit tests): the local development network.
	EVMChainID = LocalEVMChainID
)

var chainIDRe = regexp.MustCompile(`^nira_([1-9][0-9]{0,18})-([1-9][0-9]*)$`)

// ParseEVMChainID extracts the EIP-155 chain ID from a Cosmos chain-id of the
// form nira_<evm-chain-id>-<revision>.
func ParseEVMChainID(cosmosChainID string) (uint64, error) {
	m := chainIDRe.FindStringSubmatch(cosmosChainID)
	if m == nil {
		return 0, fmt.Errorf("chain-id %q must look like nira_<evm-chain-id>-<revision> (mainnet nira_%d-1, testnet nira_%d-1, local nira_%d-1)",
			cosmosChainID, MainnetEVMChainID, TestnetEVMChainID, LocalEVMChainID)
	}
	return strconv.ParseUint(m[1], 10, 64)
}

// NetworkName names the network an EVM chain ID belongs to.
func NetworkName(evmChainID uint64) string {
	switch evmChainID {
	case MainnetEVMChainID:
		return "mainnet"
	case TestnetEVMChainID:
		return "testnet"
	case LocalEVMChainID:
		return "local"
	}
	return "custom"
}
