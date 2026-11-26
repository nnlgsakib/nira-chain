package app

import (
	"github.com/cosmos/cosmos-sdk/types/module"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
)

// simGenesisAccounts is the simulation's genesis-account generator. The SDK
// default also creates random vesting accounts and converts the stake with
// Int64(), which overflows for 18-decimal tokens once accounts hold more than
// ~9.2 NIRA. Plain base accounts let simulations use realistic stakes; vesting
// genesis accounts are covered by the genesis builder tests (tests/genesis).
func simGenesisAccounts(simState *module.SimulationState) authtypes.GenesisAccounts {
	accs := make(authtypes.GenesisAccounts, len(simState.Accounts))
	for i, acc := range simState.Accounts {
		accs[i] = authtypes.NewBaseAccountWithAddress(acc.Address)
	}
	return accs
}
