package cmd

import (
	cmtcfg "github.com/cometbft/cometbft/config"
	evmconfig "github.com/cosmos/evm/config"

	"nira-chain/app"
)

// initCometBFTConfig overrides default CometBFT config values.
func initCometBFTConfig() *cmtcfg.Config {
	cfg := cmtcfg.DefaultConfig()
	return cfg
}

// initAppConfig returns the app.toml template and defaults, including the
// [evm], [json-rpc] and [tls] sections from cosmos/evm. The EVM chain ID
// is written as 0: the node derives it from the genesis chain-id (nira_<id>-<rev>).
//
// Min gas price defaults to 0anira because EVM fees are enforced by the
// x/feemarket base fee; operators may raise it (docs/ops, production TODO).
func initAppConfig() (string, interface{}) {
	// 0 = derive the EVM chain ID from the chain-id in genesis (app.GetEVMChainID)
	return evmconfig.InitAppConfig(app.BaseDenom, 0)
}
