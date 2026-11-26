package app

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"cosmossdk.io/log"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/cosmos/evm/mempool/txpool"
)

func TestMempoolLoggerDowngradesOnlyAlreadyKnown(t *testing.T) {
	var buf bytes.Buffer
	l := newMempoolLogger(log.NewLogger(&buf, log.LevelOption(zerolog.DebugLevel), log.ColorOption(false)))

	// duplicate gossip → DEBUG (not ERR)
	l.Error("failed to insert EVM transaction", "error", txpool.ErrAlreadyKnown, "tx_hash", "0xaa")
	require.NotContains(t, buf.String(), "ERR")

	// wrapped duplicate is also recognised
	buf.Reset()
	l.With("module", "ExperimentalEVMMempool").Error("failed to insert EVM transaction",
		"error", errors.Join(errors.New("ctx"), txpool.ErrAlreadyKnown))
	require.NotContains(t, buf.String(), "ERR")

	// any other failure stays ERROR
	buf.Reset()
	l.Error("failed to insert EVM transaction", "error", txpool.ErrUnderpriced, "tx_hash", "0xbb")
	require.True(t, strings.Contains(buf.String(), "ERR"), buf.String())
}
