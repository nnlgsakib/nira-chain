package main

import (
	"os"
	"path/filepath"
	"testing"
)

// The folder dialog refuses a missing start folder (seen on Windows with the
// default %LOCALAPPDATA%\NiraNode before install): open at the nearest parent.
func TestExistingParent(t *testing.T) {
	base := t.TempDir()
	if got := existingParent(base); got != base {
		t.Fatalf("existing dir: %q", got)
	}
	missing := filepath.Join(base, "NiraNode", "deeper")
	if got := existingParent(missing); got != base {
		t.Fatalf("missing dir: %q, want %q", got, base)
	}
	if got := existingParent(""); got != "" {
		t.Fatalf("empty: %q", got)
	}
	f := filepath.Join(base, "file")
	_ = os.WriteFile(f, nil, 0o600)
	if got := existingParent(filepath.Join(f, "x")); got != base {
		t.Fatalf("under a file: %q", got)
	}
}
