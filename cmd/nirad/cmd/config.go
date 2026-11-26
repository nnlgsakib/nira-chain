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
// default is the protocol constant app.EVMChainID (ADR-0005).
//
// Min gas price defaults to 0anira because EVM fees are enforced by the
// x/feemarket base fee; operators may raise it (docs/ops, production TODO).
func initAppConfig() (string, interface{}) {
	return evmconfig.InitAppConfig(app.BaseDenom, app.EVMChainID)
}
