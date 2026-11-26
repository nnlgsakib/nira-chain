package app

import (
	"errors"

	"cosmossdk.io/log"

	"github.com/cosmos/evm/mempool/txpool"
)

// mempoolLogger wraps the logger given to the cosmos/evm ExperimentalEVMMempool.
//
// CometBFT gossips each transaction to a node from several peers. When a copy
// arrives that the EVM txpool already holds, Insert fails with
// txpool.ErrAlreadyKnown, and cosmos/evm v0.6.0 logs every failed insert at
// ERROR. Under load that is hundreds of ERROR lines for harmless duplicates,
// which buries real errors and trips log-based alerting. This wrapper logs
// exactly that case at DEBUG; every other insert failure stays at ERROR.
type mempoolLogger struct{ log.Logger }

func newMempoolLogger(l log.Logger) log.Logger { return mempoolLogger{l} }

func (l mempoolLogger) Error(msg string, keyVals ...any) {
	if isAlreadyKnown(keyVals) {
		l.Logger.Debug(msg, keyVals...)
		return
	}
	l.Logger.Error(msg, keyVals...)
}

func (l mempoolLogger) With(keyVals ...any) log.Logger { return mempoolLogger{l.Logger.With(keyVals...)} }

func isAlreadyKnown(keyVals []any) bool {
	for i := 0; i+1 < len(keyVals); i += 2 {
		if k, ok := keyVals[i].(string); ok && k == "error" {
			if err, ok := keyVals[i+1].(error); ok && errors.Is(err, txpool.ErrAlreadyKnown) {
				return true
			}
		}
	}
	return false
}
