package node

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// LeanFloor is the minimum number of state versions and blocks a lean node keeps
// (scripts/node-profile.py LEAN_FLOOR).
const LeanFloor = 2000

// LeanKeep returns how many state versions and blocks a community node must keep:
// 2 × (apphash_window + epoch_blocks) from genesis, at least LeanFloor. Proof
// heights never leave apphash_window + epoch_blocks (docs/spec/community.md).
func LeanKeep(genesis []byte) int {
	var g struct {
		AppState struct {
			Community struct {
				Params struct {
					ApphashWindow json.Number `json:"apphash_window"`
					EpochBlocks   json.Number `json:"epoch_blocks"`
				} `json:"params"`
			} `json:"community"`
		} `json:"app_state"`
	}
	need := 0
	if json.Unmarshal(genesis, &g) == nil {
		w, err1 := g.AppState.Community.Params.ApphashWindow.Int64()
		e, err2 := g.AppState.Community.Params.EpochBlocks.Int64()
		if err1 == nil && err2 == nil {
			need = int(2 * (w + e))
		}
	}
	if need < LeanFloor {
		return LeanFloor
	}
	return need
}

// Setting is one `key = value` line inside a TOML section ("" = top level).
type Setting struct {
	Section, Key, Value string
}

// LeanProfile is the community-node configuration of scripts/node-profile.py
// (role "community"), plus the app's own ports so it never clashes with another
// local node. JSON-RPC stays off: proving needs only CometBFT RPC and REST.
func LeanProfile(keep int, ports Ports) (cfg, app []Setting) {
	k := strconv.Itoa(keep)
	cfg = []Setting{
		{"p2p", "laddr", fmt.Sprintf(`"tcp://0.0.0.0:%d"`, ports.P2P)},
		{"p2p", "pex", "true"},
		{"p2p", "addr_book_strict", "true"},
		{"p2p", "max_num_inbound_peers", "20"},
		{"p2p", "max_num_outbound_peers", "10"},
		{"p2p", "send_rate", "20480000"},
		{"p2p", "recv_rate", "20480000"},
		{"rpc", "laddr", fmt.Sprintf(`"tcp://127.0.0.1:%d"`, ports.RPC)},
		{"rpc", "pprof_laddr", `""`},
		{"tx_index", "indexer", `"null"`},
		{"instrumentation", "prometheus", "true"},
		{"instrumentation", "prometheus_listen_addr", fmt.Sprintf(`"127.0.0.1:%d"`, ports.Metrics)},
	}
	app = []Setting{
		{"", "pruning", `"custom"`},
		{"", "pruning-keep-recent", `"` + k + `"`},
		{"", "pruning-interval", `"100"`},
		{"", "min-retain-blocks", k},
		{"api", "enable", "true"},
		{"api", "address", fmt.Sprintf(`"tcp://127.0.0.1:%d"`, ports.REST)},
		{"grpc", "address", fmt.Sprintf(`"127.0.0.1:%d"`, ports.GRPC)},
		{"json-rpc", "enable", "false"},
	}
	return cfg, app
}

var sectionRe = regexp.MustCompile(`^\[([^\]]+)\]\s*$`)

// SetTOML sets `key = value` inside section (top level when section is ""),
// leaving every other line untouched. The key must already exist: nirad init
// writes every setting, so a missing key means the file is not what we expect.
func SetTOML(text string, s Setting) (string, bool, error) {
	lines := strings.Split(text, "\n")
	cur := ""
	keyRe := regexp.MustCompile(`^` + regexp.QuoteMeta(s.Key) + `\s*=`)
	for i, line := range lines {
		if m := sectionRe.FindStringSubmatch(line); m != nil {
			cur = m[1]
			continue
		}
		if cur == s.Section && keyRe.MatchString(line) {
			want := s.Key + " = " + s.Value
			if strings.TrimSpace(line) == want {
				return text, false, nil
			}
			lines[i] = want
			return strings.Join(lines, "\n"), true, nil
		}
	}
	return text, false, fmt.Errorf("[%s] %s not found", s.Section, s.Key)
}

// ApplyProfile writes the lean profile into home/config/{config,app}.toml and
// returns the number of changed lines.
func ApplyProfile(home string, keep int, ports Ports) (int, error) {
	cfg, app := LeanProfile(keep, ports)
	n := 0
	for file, settings := range map[string][]Setting{"config.toml": cfg, "app.toml": app} {
		p := filepath.Join(home, "config", file)
		b, err := os.ReadFile(p)
		if err != nil {
			return n, err
		}
		text := string(b)
		for _, s := range settings {
			var changed bool
			if text, changed, err = SetTOML(text, s); err != nil {
				return n, fmt.Errorf("%s: %w", file, err)
			}
			if changed {
				n++
			}
		}
		if err := os.WriteFile(p, []byte(text), 0o600); err != nil {
			return n, err
		}
	}
	return n, nil
}
