package node

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var httpc = &http.Client{Timeout: 15 * time.Second}

func getJSON(ctx context.Context, u string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return err
	}
	resp, err := httpc.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GET %s: %s", u, resp.Status)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

// Initialized reports whether the selected network's home has been set up.
func (m *Manager) Initialized() bool {
	_, err := os.Stat(filepath.Join(m.Home(), "config", "genesis.json"))
	return err == nil && m.Settings().Network != ""
}

// SetupStep is reported to the UI while Setup runs.
type SetupStep func(step string)

// Setup creates the node home for a network: init, genesis, peers, the lean
// profile, and state sync (or block sync) configuration. Re-running replaces
// the configuration but keeps the keyring and onion secret.
func (m *Manager) Setup(ctx context.Context, networkID, moniker string, report SetupStep) error {
	net, ok := NetworkByID(networkID)
	if !ok {
		return fmt.Errorf("unknown network %q", networkID)
	}
	if !net.Launched || len(net.RPC) == 0 {
		return fmt.Errorf("%s is not launched yet", net.Name)
	}
	if !regexp.MustCompile(`^[A-Za-z0-9 ._-]{2,32}$`).MatchString(moniker) {
		return errors.New("node name: 2–32 letters, digits, spaces, dots, dashes or underscores")
	}
	s := m.Settings()
	s.Network, s.Moniker = net.ID, moniker
	if err := m.SaveSettings(s); err != nil {
		return err
	}
	home := m.Home()

	report("Creating the node folder")
	if _, err := os.Stat(filepath.Join(home, "config", "config.toml")); err != nil {
		if _, err := m.run(ctx, "", "init", moniker, "--chain-id", net.ChainID, "--home", home); err != nil {
			return err
		}
	}

	report("Downloading the genesis file")
	var gen struct {
		Result struct {
			Genesis json.RawMessage `json:"genesis"`
		} `json:"result"`
	}
	if err := getJSON(ctx, net.RPC[0]+"/genesis", &gen); err != nil {
		return fmt.Errorf("genesis: %w", err)
	}
	var head struct {
		ChainID string `json:"chain_id"`
	}
	if err := json.Unmarshal(gen.Result.Genesis, &head); err != nil || head.ChainID != net.ChainID {
		return fmt.Errorf("genesis is for %q, expected %s", head.ChainID, net.ChainID)
	}
	if err := os.WriteFile(filepath.Join(home, "config", "genesis.json"), gen.Result.Genesis, 0o600); err != nil {
		return err
	}

	report("Applying the lean community-node profile")
	keep := LeanKeep(gen.Result.Genesis)
	if _, err := ApplyProfile(home, keep); err != nil {
		return err
	}

	report("Finding peers")
	peers, err := peersOf(ctx, net)
	if err != nil {
		return err
	}
	cfgPath := filepath.Join(home, "config", "config.toml")
	b, err := os.ReadFile(cfgPath)
	if err != nil {
		return err
	}
	text := string(b)
	set := []Setting{{"p2p", "persistent_peers", `"` + peers + `"`}, {"p2p", "seeds", `"` + net.Seeds + `"`}}
	if net.StateSync {
		report("Fetching a trusted checkpoint for state sync")
		h, hash, err := trustedCheckpoint(ctx, net.RPC)
		if err != nil {
			return err
		}
		set = append(set,
			Setting{"statesync", "enable", "true"},
			Setting{"statesync", "rpc_servers", `"` + strings.Join(net.RPC, ",") + `"`},
			Setting{"statesync", "trust_height", fmt.Sprint(h)},
			Setting{"statesync", "trust_hash", `"` + hash + `"`})
	}
	for _, st := range set {
		if text, _, err = SetTOML(text, st); err != nil {
			return err
		}
	}
	if err := os.WriteFile(cfgPath, []byte(text), 0o600); err != nil {
		return err
	}
	report(fmt.Sprintf("Ready: keeps the last %d blocks and state versions", keep))
	return nil
}

// peersOf asks each RPC for its node id and returns id@host:port peers.
func peersOf(ctx context.Context, net Network) (string, error) {
	var out []string
	for _, rpc := range net.RPC {
		var st struct {
			Result struct {
				NodeInfo struct {
					ID         string `json:"id"`
					ListenAddr string `json:"listen_addr"`
				} `json:"node_info"`
			} `json:"result"`
		}
		if err := getJSON(ctx, rpc+"/status", &st); err != nil {
			continue
		}
		u, _ := url.Parse(rpc)
		port := "26656"
		if i := strings.LastIndex(st.Result.NodeInfo.ListenAddr, ":"); i >= 0 {
			port = st.Result.NodeInfo.ListenAddr[i+1:]
		}
		out = append(out, fmt.Sprintf("%s@%s:%s", st.Result.NodeInfo.ID, u.Hostname(), port))
	}
	if len(out) == 0 {
		return "", errors.New("no peer answered: check your internet connection")
	}
	return strings.Join(out, ","), nil
}

// trustedCheckpoint needs at least two RPCs that report the same block hash at
// the same recent height (scripts/trusted-checkpoint.py).
func trustedCheckpoint(ctx context.Context, rpcs []string) (int64, string, error) {
	if len(rpcs) < 2 {
		return 0, "", errors.New("state sync needs two independent RPC endpoints")
	}
	var latest struct {
		Result struct {
			SyncInfo struct {
				Height string `json:"latest_block_height"`
			} `json:"sync_info"`
		} `json:"result"`
	}
	if err := getJSON(ctx, rpcs[0]+"/status", &latest); err != nil {
		return 0, "", err
	}
	var h int64
	fmt.Sscan(latest.Result.SyncInfo.Height, &h)
	h -= 1000
	if h < 1 {
		return 0, "", errors.New("chain too young for state sync")
	}
	hash := ""
	for _, rpc := range rpcs {
		var blk struct {
			Result struct {
				BlockID struct {
					Hash string `json:"hash"`
				} `json:"block_id"`
			} `json:"result"`
		}
		if err := getJSON(ctx, fmt.Sprintf("%s/block?height=%d", rpc, h), &blk); err != nil {
			return 0, "", err
		}
		if hash == "" {
			hash = blk.Result.BlockID.Hash
		} else if hash != blk.Result.BlockID.Hash {
			return 0, "", fmt.Errorf("RPCs disagree on block %d: refusing to trust either", h)
		}
	}
	return h, hash, nil
}

// ------------------------------------------------------------------- keys

// CreateKey makes a new operator key protected by pass and returns its
// recovery phrase (shown once by the UI, never stored by the app).
func (m *Manager) CreateKey(pass string) (string, error) {
	if len(pass) < 8 {
		return "", errors.New("the password must have at least 8 characters")
	}
	out, err := m.run(context.Background(), pass+"\n"+pass+"\n", "keys", "add", KeyName, "--keyring-backend", "file",
		"--algo", "eth_secp256k1", "--home", m.Home(), "--output", "json")
	if err != nil {
		return "", err
	}
	var k struct {
		Address  string `json:"address"`
		Mnemonic string `json:"mnemonic"`
	}
	if err := json.Unmarshal([]byte(jsonPart(out)), &k); err != nil {
		return "", fmt.Errorf("unexpected keys output")
	}
	return k.Mnemonic, m.keyCreated(k.Address, pass)
}

// ImportKey restores the operator key from a recovery phrase.
func (m *Manager) ImportKey(mnemonic, pass string) (string, error) {
	if len(pass) < 8 {
		return "", errors.New("the password must have at least 8 characters")
	}
	words := strings.Fields(mnemonic)
	if len(words) != 12 && len(words) != 24 {
		return "", errors.New("the recovery phrase has 12 or 24 words")
	}
	out, err := m.run(context.Background(), strings.Join(words, " ")+"\n"+pass+"\n"+pass+"\n", "keys", "add", KeyName, "--recover",
		"--keyring-backend", "file", "--algo", "eth_secp256k1", "--home", m.Home(), "--output", "json")
	if err != nil {
		return "", err
	}
	var k struct {
		Address string `json:"address"`
	}
	if err := json.Unmarshal([]byte(jsonPart(out)), &k); err != nil {
		return "", fmt.Errorf("unexpected keys output")
	}
	return k.Address, m.keyCreated(k.Address, pass)
}

func (m *Manager) keyCreated(addr, pass string) error {
	s := m.Settings()
	s.Operator = addr
	m.mu.Lock()
	m.pass = pass
	m.mu.Unlock()
	return m.SaveSettings(s)
}

func jsonPart(s string) string {
	if i := strings.Index(s, "{"); i >= 0 {
		return s[i:]
	}
	return s
}

// ------------------------------------------------------------ transactions

// GasPrice returns twice the current EIP-1559 base fee (Cosmos txs pay it too).
func (m *Manager) GasPrice(ctx context.Context) (string, error) {
	var bf struct {
		BaseFee string `json:"base_fee"`
	}
	if err := getJSON(ctx, fmt.Sprintf("http://127.0.0.1:%d/cosmos/evm/feemarket/v1/base_fee", PortREST), &bf); err != nil {
		return "", err
	}
	f, ok := new(big.Float).SetString(bf.BaseFee)
	if !ok {
		return "", fmt.Errorf("base fee %q", bf.BaseFee)
	}
	v, _ := new(big.Float).Mul(f, big.NewFloat(2)).Int(nil)
	if v.Sign() <= 0 {
		v = big.NewInt(1)
	}
	return v.String() + "anira", nil
}

func (m *Manager) tx(ctx context.Context, args ...string) (string, error) {
	m.mu.Lock()
	pass := m.pass
	m.mu.Unlock()
	if pass == "" {
		return "", errors.New("locked: enter your password")
	}
	n, err := m.Network()
	if err != nil {
		return "", err
	}
	gp, err := m.GasPrice(ctx)
	if err != nil {
		return "", fmt.Errorf("node not ready: %w", err)
	}
	full := append([]string{"tx"}, args...)
	full = append(full, "--from", KeyName, "--keyring-backend", "file", "--home", m.Home(), "--chain-id", n.ChainID,
		"--node", fmt.Sprintf("tcp://127.0.0.1:%d", PortRPC), "--gas", "auto", "--gas-adjustment", "1.5",
		"--gas-prices", gp, "-y", "--output", "json")
	out, err := m.run(ctx, pass+"\n", full...)
	if err != nil {
		return "", err
	}
	var res struct {
		Code   int    `json:"code"`
		TxHash string `json:"txhash"`
		RawLog string `json:"raw_log"`
	}
	if err := json.Unmarshal([]byte(jsonPart(out)), &res); err != nil {
		return "", fmt.Errorf("unexpected tx output: %s", lastLines(out, 2))
	}
	if res.Code != 0 {
		return "", fmt.Errorf("rejected: %s", res.RawLog)
	}
	return res.TxHash, nil
}

// OnionLength is the number of epochs one onion covers (~1 year at 600-block epochs, 2 s blocks).
const OnionLength = 100000

// Register creates the onion secret (kept in the node home) and registers the
// node with a bond of bondNIRA whole NIRA.
func (m *Manager) Register(ctx context.Context, bondNIRA int64) (string, error) {
	if bondNIRA <= 0 {
		return "", errors.New("bond must be positive")
	}
	out, err := m.run(ctx, "", "tx", "community", "onion-init", "--length", fmt.Sprint(OnionLength), "--overwrite", "--home", m.Home())
	if err != nil {
		return "", err
	}
	tip := ""
	for _, line := range strings.Split(out, "\n") {
		if f := strings.Fields(line); len(f) == 2 && f[0] == "onion_tip:" {
			tip = f[1]
		}
	}
	if tip == "" {
		return "", errors.New("onion-init printed no tip")
	}
	bond := new(big.Int).Mul(big.NewInt(bondNIRA), new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil))
	return m.tx(ctx, "community", "register-node", m.Settings().Moniker, bond.String()+"anira", tip, fmt.Sprint(OnionLength))
}

// Deregister starts the bond's unbonding period.
func (m *Manager) Deregister(ctx context.Context) (string, error) {
	return m.tx(ctx, "community", "deregister-node")
}
