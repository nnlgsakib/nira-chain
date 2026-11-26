// remotesigner is a minimal REFERENCE remote signer for Nira validators (G-SEC-7).
//
// It holds the consensus key and the double-sign protection state outside the
// node, and serves signatures over CometBFT's authenticated privval protocol:
// the node listens on [base] priv_validator_laddr, the signer dials in. This is
// the same protocol TMKMS and Horcrux speak; for mainnet use one of those with
// an HSM. This tool exists to test the node side of remote signing.
//
//	go run ./tools/remotesigner --key signer/priv_validator_key.json --state signer/priv_validator_state.json \
//	    --node 127.0.0.1:26659 --chain-id nira_93000-1
package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/cometbft/cometbft/crypto/ed25519"
	cmtlog "github.com/cometbft/cometbft/libs/log"
	"github.com/cometbft/cometbft/privval"
)

func main() {
	key := flag.String("key", "", "priv_validator_key.json (moved OFF the node)")
	state := flag.String("state", "", "priv_validator_state.json (last signed height/round/step: double-sign protection)")
	node := flag.String("node", "127.0.0.1:26659", "node's priv_validator_laddr (host:port)")
	chainID := flag.String("chain-id", "", "chain ID the signer refuses to sign for any other")
	flag.Parse()
	if *key == "" || *state == "" || *chainID == "" {
		fmt.Fprintln(os.Stderr, "need --key, --state and --chain-id")
		os.Exit(2)
	}
	logger := cmtlog.NewTMLogger(cmtlog.NewSyncWriter(os.Stdout)).With("module", "remotesigner")
	var pv *privval.FilePV
	if _, err := os.Stat(*state); err == nil {
		pv = privval.LoadFilePV(*key, *state)
	} else {
		pv = privval.LoadFilePVEmptyState(*key, *state)
		pv.Save()
	}
	dialer := privval.DialTCPFn(*node, 3*time.Second, ed25519.GenPrivKey())
	endpoint := privval.NewSignerDialerEndpoint(logger, dialer,
		privval.SignerDialerEndpointRetryWaitInterval(time.Second), privval.SignerDialerEndpointConnRetries(600))
	srv := privval.NewSignerServer(endpoint, *chainID, pv)
	if err := srv.Start(); err != nil {
		fmt.Fprintln(os.Stderr, "start:", err)
		os.Exit(1)
	}
	logger.Info("signing", "validator", pv.GetAddress().String(), "node", *node)
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	<-ch
	_ = srv.Stop()
}
