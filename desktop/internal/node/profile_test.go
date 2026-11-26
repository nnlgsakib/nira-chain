package node

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLeanKeep(t *testing.T) {
	cases := map[string]int{
		`{"app_state":{"community":{"params":{"apphash_window":"100","epoch_blocks":"600"}}}}`:  2000, // 1400 → floor
		`{"app_state":{"community":{"params":{"apphash_window":"500","epoch_blocks":"1000"}}}}`: 3000,
		`{"app_state":{"community":{"params":{"apphash_window":"10","epoch_blocks":"20"}}}}`:    2000,
		`not json`: 2000,
		`{}`:       2000,
	}
	for g, want := range cases {
		if got := LeanKeep([]byte(g)); got != want {
			t.Errorf("LeanKeep(%s) = %d, want %d", g, got, want)
		}
	}
}

func TestSetTOML(t *testing.T) {
	in := "pruning = \"default\"\n\n[api]\nenable = false\naddress = \"tcp://localhost:1317\"\n\n[grpc]\nenable = true\n"
	out, changed, err := SetTOML(in, Setting{"api", "enable", "true"})
	if err != nil || !changed || !strings.Contains(out, "[api]\nenable = true") || !strings.Contains(out, "[grpc]\nenable = true") {
		t.Fatalf("section key: %v %v\n%s", changed, err, out)
	}
	out, changed, _ = SetTOML(out, Setting{"", "pruning", `"custom"`})
	if !changed || !strings.HasPrefix(out, `pruning = "custom"`) {
		t.Fatalf("top-level key:\n%s", out)
	}
	if _, changed, _ = SetTOML(out, Setting{"", "pruning", `"custom"`}); changed {
		t.Fatal("re-applying the same value must be a no-op")
	}
	if _, _, err = SetTOML(out, Setting{"api", "missing", "1"}); err == nil {
		t.Fatal("missing key must be an error")
	}
	// a key of the same name in another section is not touched
	if _, _, err = SetTOML("[a]\nx = 1\n", Setting{"b", "x", "2"}); err == nil {
		t.Fatal("key in a different section must not match")
	}
}

// The Go profile must match scripts/node-profile.py's community role on the keys
// both set (the script is the reference for operators without the app).
func TestLeanProfileMatchesScript(t *testing.T) {
	cfg, app := LeanProfile(2000, DefaultPorts())
	want := map[string]string{
		"/pruning": `"custom"`, "/pruning-keep-recent": `"2000"`, "/pruning-interval": `"100"`, "/min-retain-blocks": "2000",
		"tx_index/indexer": `"null"`, "json-rpc/enable": "false", "api/enable": "true", "p2p/pex": "true",
		"p2p/max_num_inbound_peers": "20", "p2p/max_num_outbound_peers": "10",
	}
	got := map[string]string{}
	for _, s := range append(cfg, app...) {
		got[s.Section+"/"+s.Key] = s.Value
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s = %q, want %q", k, got[k], v)
		}
	}
	script, err := os.ReadFile(filepath.Join("..", "..", "..", "scripts", "node-profile.py"))
	if err != nil {
		t.Skip("script not found")
	}
	for _, frag := range []string{`(None, "pruning"): '"custom"'`, `("tx_index", "indexer"): '"null"'`, "LEAN_FLOOR = 2000"} {
		if !strings.Contains(string(script), frag) {
			t.Errorf("scripts/node-profile.py no longer contains %s: keep the two profiles in step", frag)
		}
	}
}
