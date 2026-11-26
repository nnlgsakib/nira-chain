//go:build release

// Against the real published release (network): the installer downloads this
// platform's nirad NiradVersion, verifies it against SHA256SUMS and records it.
//
//	go test -tags release ./internal/node -run TestInstallFromRelease -v
package node

import (
	"context"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallFromRelease(t *testing.T) {
	t.Setenv("NIRA_NODE_CONFIG", filepath.Join(t.TempDir(), "config.json"))
	data := t.TempDir()
	last := ""
	in, err := InstallNirad(context.Background(), data, func(stage string, done, total int64) {
		if stage != last {
			t.Logf("%s (%d/%d bytes)", stage, done, total)
			last = stage
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(in.NiradVersion, NiradVersion) {
		t.Fatalf("installed version %q, want %s", in.NiradVersion, NiradVersion)
	}
	out, err := exec.Command(in.NiradPath, "tx", "community", "--help").CombinedOutput()
	if err != nil || !strings.Contains(string(out), "run") {
		t.Fatalf("installed nirad lacks the community commands: %v", err)
	}
	if got, ok := LoadInstall(); !ok || got.NiradPath != in.NiradPath {
		t.Fatalf("not recorded: %+v", got)
	}
	t.Logf("installed %s at %s", in.NiradVersion, in.NiradPath)
}
