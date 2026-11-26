// loadgen measures throughput on a running Nira network (G-TEST-4).
//
// It funds `--accounts` fresh eth_secp256k1 accounts from a funder key, then
// every account sends `--txs` transfers as fast as the node accepts them:
//   - native: Cosmos x/bank MsgSend (SIGN_MODE_DIRECT, broadcast via REST)
//   - evm:    Ethereum legacy value transfers (JSON-RPC eth_sendRawTransaction)
//   - mixed:  half the accounts native, half EVM
//
// Results come from committed blocks (CometBFT RPC), not from submission
// counts: included txs, TPS over the measured block range, block-time p50/p95.
//
//	go run ./tools/loadgen --mode mixed --accounts 40 --txs 25 --funder-key <hex>
package main

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"os"
	"sort"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"cosmossdk.io/math"
	"cosmossdk.io/x/tx/signing"
	"github.com/cosmos/cosmos-sdk/client"
	clienttx "github.com/cosmos/cosmos-sdk/client/tx"
	"github.com/cosmos/cosmos-sdk/codec"
	addresscodec "github.com/cosmos/cosmos-sdk/codec/address"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	cryptocodec "github.com/cosmos/cosmos-sdk/crypto/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	signingtypes "github.com/cosmos/cosmos-sdk/types/tx/signing"
	authsigning "github.com/cosmos/cosmos-sdk/x/auth/signing"
	authtx "github.com/cosmos/cosmos-sdk/x/auth/tx"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	evmcrypto "github.com/cosmos/evm/crypto/codec"
	"github.com/cosmos/evm/crypto/ethsecp256k1"
	"github.com/cosmos/gogoproto/proto"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

const denom = "anira"

var (
	mode      = flag.String("mode", "mixed", "native | evm | mixed")
	nAccounts = flag.Int("accounts", 40, "sending accounts")
	nTxs      = flag.Int("txs", 25, "transfers per account")
	funderHex = flag.String("funder-key", "", "hex eth private key of a funded account")
	rpcURL    = flag.String("evm-rpc", "http://127.0.0.1:8545", "Ethereum JSON-RPC")
	restURL   = flag.String("rest", "http://127.0.0.1:1317", "Cosmos REST")
	cmtURL    = flag.String("cmt", "http://127.0.0.1:26657", "CometBFT RPC")
	chainID   = flag.String("chain-id", "nira_93000-1", "Cosmos chain-id")
	evmChain  = flag.Int64("evm-chain-id", 93000, "EVM chain ID")
	gasPrice  = flag.Int64("gas-price", 2_000_000_000, "gas price in anira (≥ fee floor)")
)

type account struct {
	priv  *ethsecp256k1.PrivKey
	ecdsa *ecdsa.PrivateKey
	addr  common.Address
	bech  string
}

func newAccount(priv *ethsecp256k1.PrivKey) account {
	k, err := priv.ToECDSA()
	must(err)
	addr := crypto.PubkeyToAddress(k.PublicKey)
	bech := sdk.MustBech32ifyAddressBytes("nira", addr.Bytes())
	return account{priv: priv, ecdsa: k, addr: addr, bech: bech}
}

func must(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "loadgen:", err)
		os.Exit(1)
	}
}

func main() {
	flag.Parse()
	sdk.GetConfig().SetBech32PrefixForAccount("nira", "nirapub")
	keyBz, err := hex.DecodeString(*funderHex)
	must(err)
	funder := newAccount(&ethsecp256k1.PrivKey{Key: keyBz})
	ec, err := ethclient.Dial(*rpcURL)
	must(err)
	ctx := context.Background()
	signer := ethtypes.LatestSignerForChainID(big.NewInt(*evmChain))
	gp := big.NewInt(*gasPrice)

	// 1. create + fund accounts (EVM transfers from the funder, nonces in flight)
	accs := make([]account, *nAccounts)
	nonce, err := ec.PendingNonceAt(ctx, funder.addr)
	must(err)
	fund := new(big.Int).Mul(big.NewInt(1000), big.NewInt(1e18)) // 1000 NIRA each
	var last common.Hash
	for i := range accs {
		k, err := ethsecp256k1.GenerateKey()
		must(err)
		accs[i] = newAccount(k)
		tx, err := ethtypes.SignTx(ethtypes.NewTransaction(nonce+uint64(i), accs[i].addr, fund, 21000, gp, nil), signer, funder.ecdsa)
		must(err)
		must(ec.SendTransaction(ctx, tx))
		last = tx.Hash()
	}
	waitReceipt(ctx, ec, last)
	// native senders need an on-chain account (number/sequence): one tiny MsgSend-free touch is
	// enough because the EVM funding already created the accounts in x/auth.
	time.Sleep(2 * time.Second)

	txCfg, cdc := txConfig()
	startH := cmtHeight()
	var sent, rejected atomic.Int64
	var wg sync.WaitGroup
	t0 := time.Now()
	for i, a := range accs {
		native := *mode == "native" || (*mode == "mixed" && i%2 == 0)
		wg.Add(1)
		go func(a account, native bool) {
			defer wg.Done()
			if native {
				accNum, seq := authInfo(cdc, a.bech)
				for j := 0; j < *nTxs; j++ {
					if err := sendNative(txCfg, a, accNum, seq+uint64(j)); err != nil {
						rejected.Add(1)
						continue
					}
					sent.Add(1)
				}
				return
			}
			for j := 0; j < *nTxs; j++ {
				tx, err := ethtypes.SignTx(ethtypes.NewTransaction(uint64(j), funder.addr, big.NewInt(1), 21000, gp, nil), signer, a.ecdsa)
				must(err)
				if err := ec.SendTransaction(ctx, tx); err != nil {
					rejected.Add(1)
					continue
				}
				sent.Add(1)
			}
		}(a, native)
	}
	wg.Wait()
	submitDur := time.Since(t0)

	// 2. wait until the mempool drains (no txs for 3 consecutive blocks), then measure from blocks
	endH := waitDrain(startH)
	included, times, perBlock := blockStats(startH+1, endH)
	span := times[len(times)-1].Sub(times[0]).Seconds()
	var gaps []float64
	for i := 1; i < len(times); i++ {
		gaps = append(gaps, times[i].Sub(times[i-1]).Seconds())
	}
	sort.Float64s(gaps)
	pct := func(p float64) float64 { return gaps[int(float64(len(gaps)-1)*p)] }
	busy := 0
	maxTx := 0
	for _, n := range perBlock {
		if n > 0 {
			busy++
		}
		if n > maxTx {
			maxTx = n
		}
	}
	res := map[string]any{
		"mode": *mode, "accounts": *nAccounts, "txs_per_account": *nTxs,
		"submitted": sent.Load(), "rejected_at_submit": rejected.Load(), "included": included,
		"submit_seconds": round(submitDur.Seconds()), "blocks": endH - startH, "busy_blocks": busy,
		"measured_seconds": round(span), "tps": round(float64(included) / span),
		"block_time_p50_s": round(pct(0.5)), "block_time_p95_s": round(pct(0.95)), "max_txs_per_block": maxTx,
	}
	bz, _ := json.MarshalIndent(res, "", "  ")
	fmt.Println(string(bz))
}

func round(f float64) float64 { return float64(int(f*100+0.5)) / 100 }

func txConfig() (client.TxConfig, codec.Codec) {
	reg, err := codectypes.NewInterfaceRegistryWithOptions(codectypes.InterfaceRegistryOptions{
		ProtoFiles: proto.HybridResolver,
		SigningOptions: signing.Options{
			AddressCodec:          addresscodec.NewBech32Codec("nira"),
			ValidatorAddressCodec: addresscodec.NewBech32Codec("niravaloper"),
		},
	})
	must(err)
	authtypes.RegisterInterfaces(reg)
	banktypes.RegisterInterfaces(reg)
	cryptocodec.RegisterInterfaces(reg)
	evmcrypto.RegisterInterfaces(reg)
	cdc := codec.NewProtoCodec(reg)
	return authtx.NewTxConfig(cdc, authtx.DefaultSignModes), cdc
}

func authInfo(cdc codec.Codec, bech string) (uint64, uint64) {
	var out struct {
		Account json.RawMessage `json:"account"`
	}
	getJSON(*restURL+"/cosmos/auth/v1beta1/accounts/"+bech, &out)
	var acc struct {
		AccountNumber string `json:"account_number"`
		Sequence      string `json:"sequence"`
	}
	must(json.Unmarshal(out.Account, &acc))
	n, _ := strconv.ParseUint(acc.AccountNumber, 10, 64)
	s, _ := strconv.ParseUint(acc.Sequence, 10, 64)
	return n, s
}

func sendNative(txCfg client.TxConfig, a account, accNum, seq uint64) error {
	b := txCfg.NewTxBuilder()
	from := sdk.AccAddress(a.addr.Bytes())
	if err := b.SetMsgs(banktypes.NewMsgSend(from, from, sdk.NewCoins(sdk.NewCoin(denom, math.NewInt(1))))); err != nil {
		return err
	}
	gas := uint64(120_000)
	b.SetGasLimit(gas)
	b.SetFeeAmount(sdk.NewCoins(sdk.NewCoin(denom, math.NewInt(*gasPrice).MulRaw(int64(gas)))))
	pub := a.priv.PubKey()
	if err := b.SetSignatures(signingtypes.SignatureV2{PubKey: pub, Sequence: seq,
		Data: &signingtypes.SingleSignatureData{SignMode: signingtypes.SignMode_SIGN_MODE_DIRECT}}); err != nil {
		return err
	}
	sd := authsigning.SignerData{
		ChainID: *chainID, AccountNumber: accNum, Sequence: seq, PubKey: pub,
		Address: sdk.AccAddress(a.addr.Bytes()).String(),
	}
	sig, err := clienttx.SignWithPrivKey(context.Background(), signingtypes.SignMode_SIGN_MODE_DIRECT, sd, b, a.priv, txCfg, seq)
	if err != nil {
		return err
	}
	if err := b.SetSignatures(sig); err != nil {
		return err
	}
	bz, err := txCfg.TxEncoder()(b.GetTx())
	if err != nil {
		return err
	}
	body, _ := json.Marshal(map[string]any{"tx_bytes": bz, "mode": "BROADCAST_MODE_SYNC"})
	resp, err := http.Post(*restURL+"/cosmos/tx/v1beta1/txs", "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	var r struct {
		TxResponse struct {
			Code   int    `json:"code"`
			RawLog string `json:"raw_log"`
		} `json:"tx_response"`
	}
	raw, _ := io.ReadAll(resp.Body)
	if err := json.Unmarshal(raw, &r); err != nil {
		return err
	}
	if r.TxResponse.Code != 0 {
		return fmt.Errorf("checktx %d: %s", r.TxResponse.Code, r.TxResponse.RawLog)
	}
	return nil
}

func getJSON(url string, out any) {
	resp, err := http.Get(url)
	must(err)
	defer resp.Body.Close()
	must(json.NewDecoder(resp.Body).Decode(out))
}

func cmtHeight() int64 {
	var s struct {
		Result struct {
			SyncInfo struct {
				H string `json:"latest_block_height"`
			} `json:"sync_info"`
		} `json:"result"`
	}
	getJSON(*cmtURL+"/status", &s)
	h, _ := strconv.ParseInt(s.Result.SyncInfo.H, 10, 64)
	return h
}

func blockAt(h int64) (int, time.Time) {
	var b struct {
		Result struct {
			Block struct {
				Header struct {
					Time time.Time `json:"time"`
				} `json:"header"`
				Data struct {
					Txs []string `json:"txs"`
				} `json:"data"`
			} `json:"block"`
		} `json:"result"`
	}
	getJSON(fmt.Sprintf("%s/block?height=%d", *cmtURL, h), &b)
	return len(b.Result.Block.Data.Txs), b.Result.Block.Header.Time
}

func waitDrain(start int64) int64 {
	idle := 0
	h := start
	lastBusy := start
	for idle < 3 {
		time.Sleep(500 * time.Millisecond)
		cur := cmtHeight()
		for ; h < cur; h++ {
			n, _ := blockAt(h + 1)
			if n > 0 {
				idle = 0
				lastBusy = h + 1
			} else {
				idle++
			}
		}
	}
	return lastBusy
}

func blockStats(from, to int64) (int, []time.Time, []int) {
	total := 0
	var times []time.Time
	var per []int
	_, t := blockAt(from - 1)
	times = append(times, t)
	for h := from; h <= to; h++ {
		n, t := blockAt(h)
		total += n
		times = append(times, t)
		per = append(per, n)
	}
	return total, times, per
}

func waitReceipt(ctx context.Context, ec *ethclient.Client, h common.Hash) {
	for i := 0; i < 60; i++ {
		if r, err := ec.TransactionReceipt(ctx, h); err == nil && r != nil {
			return
		}
		time.Sleep(time.Second)
	}
	must(fmt.Errorf("funding tx %s not included", h))
}
