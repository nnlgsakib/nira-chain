package node

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// StartNode/StartWorker spawn while holding m.mu; anything they call must not
// take m.mu again (regression: env() called Settings() and deadlocked).
func TestStartDoesNotDeadlock(t *testing.T) {
	sh, err := exec.LookPath("true")
	if err != nil {
		t.Skip("no `true` binary")
	}
	m, err := NewManager(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	m.Nirad = sh
	_ = m.SaveSettings(Settings{Network: "local", MaxCPUs: 2, MaxMemoryMB: 2048})
	m.pass = "password1"
	done := make(chan error, 1)
	go func() {
		if err := m.StartNode(); err != nil {
			done <- err
			return
		}
		done <- m.StartWorker("1anira")
	}()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("StartNode/StartWorker deadlocked")
	}
	m.StopNode()
}

func TestEnvLimits(t *testing.T) {
	m := &Manager{settings: Settings{MaxCPUs: 3, MaxMemoryMB: 1536}}
	env := strings.Join(m.env(), "\n")
	for _, want := range []string{"GOMAXPROCS=3", "GOMEMLIMIT=1536MiB"} {
		if !strings.Contains(env, want) {
			t.Errorf("env lacks %s", want)
		}
	}
	m.settings = Settings{}
	env = strings.Join(m.env(), "\n")
	if !strings.Contains(env, "GOMEMLIMIT=4096MiB") {
		t.Error("default memory limit not 4096 MiB")
	}
}

// Real nirad (when on PATH): a hex private key imports into the file keyring
// and yields the address that key controls; a malformed key is refused.
func TestImportPrivateKey(t *testing.T) {
	m, err := NewManager(t.TempDir())
	if err != nil || m.Nirad == "" {
		t.Skip("nirad not on PATH")
	}
	_ = m.SaveSettings(Settings{Network: "local"})
	// Hardhat/Anvil account #0: address 0xf39F…2266
	addr, err := m.ImportPrivateKey("0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80", "password-123")
	if err != nil {
		t.Fatal(err)
	}
	if addr != "nira17w0adeg64ky0daxwd2ugyuneellmjgnxgyasls" || m.Settings().Operator != addr {
		t.Fatalf("address %s, settings %s", addr, m.Settings().Operator)
	}
	m.Lock()
	if err := m.Unlock("password-123"); err != nil {
		t.Fatalf("unlock with the new password: %v", err)
	}
	if _, err := m.ImportPrivateKey("zz12", "password-123"); err == nil {
		t.Fatal("malformed key accepted")
	}
}

// A node that dies right away must leave its reason and last lines behind
// (status shows them) and the support log; the log list is never nil.
func TestNodeExitIsReported(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell script stand-in")
	}
	dir := t.TempDir()
	fake := filepath.Join(dir, "fake-nirad")
	if err := os.WriteFile(fake, []byte("#!/bin/sh\necho 'ERR failed to load config: bad port'\nexit 3\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	m, err := NewManager(dir)
	if err != nil {
		t.Fatal(err)
	}
	if l := m.Logs(10); l == nil {
		t.Fatal("Logs returned nil before any output")
	}
	m.Nirad = fake
	_ = m.SaveSettings(Settings{Network: "local"})
	if err := m.StartNode(); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for m.NodeRunning() && time.Now().Before(deadline) {
		time.Sleep(50 * time.Millisecond)
	}
	e, ok := m.LastExit("node")
	if !ok || !strings.Contains(e.Error, "exit status 3") || len(e.Tail) == 0 || !strings.Contains(e.Tail[len(e.Tail)-1], "bad port") {
		t.Fatalf("exit info %+v %v", e, ok)
	}
	b, _ := os.ReadFile(m.LogPath())
	if !strings.Contains(string(b), "bad port") || !strings.Contains(string(b), "process exited") {
		t.Fatalf("support log:\n%s", b)
	}
}
