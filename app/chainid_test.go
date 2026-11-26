package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/cosmos/cosmos-sdk/client/flags"
	simtestutil "github.com/cosmos/cosmos-sdk/testutil/sims"
	srvflags "github.com/cosmos/evm/server/flags"
	"github.com/stretchr/testify/require"
)

func TestParseEVMChainID(t *testing.T) {
	for id, want := range map[string]uint64{
		"nira_91000-1": MainnetEVMChainID, "nira_92000-1": TestnetEVMChainID, "nira_93000-1": LocalEVMChainID, "nira_92000-17": 92000,
	} {
		got, err := ParseEVMChainID(id)
		require.NoError(t, err, id)
		require.Equal(t, want, got, id)
	}
	for _, bad := range []string{"", "nira", "nira-1", "nira_47201", "nira_0-1", "nira_92000-0", "evmos_9001-1", "nira_092000-1", "nira_92000-1x"} {
		_, err := ParseEVMChainID(bad)
		require.Error(t, err, bad)
	}
	require.Equal(t, "mainnet", NetworkName(91000))
	require.Equal(t, "testnet", NetworkName(92000))
	require.Equal(t, "local", NetworkName(93000))
}

func TestGetEVMChainID(t *testing.T) {
	home := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(home, "config"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(home, "config", "genesis.json"), []byte(`{"chain_id":"nira_91000-1"}`), 0o644))

	// derived from genesis when app.toml leaves it at 0
	id, err := GetEVMChainID(simtestutil.AppOptionsMap{flags.FlagHome: home})
	require.NoError(t, err)
	require.Equal(t, MainnetEVMChainID, id)

	// an explicit matching value is accepted, a conflicting one refuses to start
	_, err = GetEVMChainID(simtestutil.AppOptionsMap{flags.FlagHome: home, srvflags.EVMChainID: uint64(91000)})
	require.NoError(t, err)
	_, err = GetEVMChainID(simtestutil.AppOptionsMap{flags.FlagHome: home, srvflags.EVMChainID: uint64(92000)})
	require.ErrorContains(t, err, "means EVM chain ID 91000")

	// --chain-id flag wins over genesis lookup
	id, err = GetEVMChainID(simtestutil.AppOptionsMap{flags.FlagHome: home, flags.FlagChainID: "nira_92000-1"})
	require.NoError(t, err)
	require.Equal(t, TestnetEVMChainID, id)

	// a chain-id without an EVM chain ID is rejected
	_, err = GetEVMChainID(simtestutil.AppOptionsMap{flags.FlagChainID: "nira-1"})
	require.Error(t, err)

	// no genesis (unit tests, simulation): local default
	id, err = GetEVMChainID(simtestutil.AppOptionsMap{flags.FlagHome: t.TempDir()})
	require.NoError(t, err)
	require.Equal(t, LocalEVMChainID, id)
}
