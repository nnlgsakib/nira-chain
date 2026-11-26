package node

import (
	"fmt"
	"net"
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

// A busy port is replaced by the next free one: settings, config.toml and the
// activity log all reflect it, and status/tx use the new port.
func TestBusyPortMovesToNextFree(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell script stand-in")
	}
	dir := t.TempDir()
	m, err := NewManager(dir)
	if err != nil {
		t.Fatal(err)
	}
	_ = m.SaveSettings(Settings{Network: "local"})
	home := m.Home()
	if err := os.MkdirAll(filepath.Join(home, "config"), 0o700); err != nil {
		t.Fatal(err)
	}
	// minimal config files with the keys the profile sets
	cfg := "[p2p]\nladdr = \"x\"\n[rpc]\nladdr = \"x\"\n[instrumentation]\nprometheus_listen_addr = \"x\"\n"
	app := "[api]\naddress = \"x\"\n[grpc]\naddress = \"x\"\n"
	_ = os.WriteFile(filepath.Join(home, "config", "config.toml"), []byte(cfg), 0o600)
	_ = os.WriteFile(filepath.Join(home, "config", "app.toml"), []byte(app), 0o600)

	def := DefaultPorts()
	l, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", def.RPC))
	if err != nil {
		t.Skipf("default RPC port already busy here: %v", err)
	}
	defer l.Close()

	fake := filepath.Join(dir, "fake-nirad")
	_ = os.WriteFile(fake, []byte("#!/bin/sh\nsleep 1\n"), 0o755)
	m.Nirad = fake
	if err := m.StartNode(); err != nil {
		t.Fatal(err)
	}
	defer m.StopNode()
	p := m.Ports()
	if p.RPC == def.RPC || p.RPC <= def.RPC {
		t.Fatalf("RPC port not moved: %d", p.RPC)
	}
	b, _ := os.ReadFile(filepath.Join(home, "config", "config.toml"))
	if !strings.Contains(string(b), fmt.Sprintf(`laddr = "tcp://127.0.0.1:%d"`, p.RPC)) {
		t.Fatalf("config.toml not updated:\n%s", b)
	}
	if !strings.Contains(fmt.Sprint(m.Logs(20)), fmt.Sprintf("port %d for the RPC was busy: using %d", def.RPC, p.RPC)) {
		t.Fatal("port change not noted in the activity log")
	}
	if got := m.Settings().Ports; got != p {
		t.Fatalf("settings not saved: %+v vs %+v", got, p)
	}
}

// A nirad left running by an earlier session (same binary path) is stopped
// before a new node starts: on Windows it held the database open and the new
// node failed with "being used by another process".
func TestLeftoverNiradIsStopped(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("uses /proc and a copied sleep binary")
	}
	dir := t.TempDir()
	sleepBin, err := exec.LookPath("sleep")
	if err != nil {
		t.Skip("no sleep")
	}
	b, _ := os.ReadFile(sleepBin)
	fake := filepath.Join(dir, "nirad")
	if err := os.WriteFile(fake, b, 0o755); err != nil {
		t.Fatal(err)
	}
	orphan := exec.Command(fake, "300") // "earlier session": not started by the manager
	if err := orphan.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() { _ = orphan.Wait(); close(done) }()

	m, err := NewManager(dir)
	if err != nil {
		t.Fatal(err)
	}
	m.Nirad = fake
	_ = m.SaveSettings(Settings{Network: "local"})
	if err := m.StartNode(); err != nil { // the new "node" is sleep with bad args: exits at once
		t.Fatal(err)
	}
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		_ = orphan.Process.Kill()
		t.Fatal("leftover nirad was not stopped")
	}
	if !strings.Contains(fmt.Sprint(m.Logs(20)), fmt.Sprintf("pid %d", orphan.Process.Pid)) {
		t.Fatal("stopping the leftover was not logged")
	}
	m.StopNode()
}

func TestStripANSI(t *testing.T) {
	in := "\x1b[90m4:23PM\x1b[0m \x1b[32mINF\x1b[0m \x1b[1mexecuted block\x1b[0m \x1b[36mapp_hash=\x1b[0m19AF \x1b[36mheight=\x1b[0m713"
	want := "4:23PM INF executed block app_hash=19AF height=713"
	if got := stripANSI(in); got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
	if got := stripANSI("plain text"); got != "plain text" {
		t.Fatalf("got %q", got)
	}
}

func TestSpawnStripsANSIFromLogsAndTail(t *testing.T) {
	if os.Getenv("GO_WANT_ANSI_HELPER") == "1" {
		fmt.Println("\x1b[90m4:23PM\x1b[0m \x1b[32mINF\x1b[0m \x1b[1mexecuted block\x1b[0m \x1b[36mheight=\x1b[0m713")
		os.Exit(2)
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("GO_WANT_ANSI_HELPER", "1")
	m, err := NewManager(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	m.Nirad = exe
	p, err := m.spawn("node", "", "-test.run=^TestSpawnStripsANSIFromLogsAndTail$")
	if err != nil {
		t.Fatal(err)
	}
	<-p.done

	want := "4:23PM INF executed block height=713"
	logs := m.Logs(20)
	var found bool
	for _, l := range logs {
		if strings.Contains(l.Text, "\x1b") || strings.Contains(l.Text, "[90m") {
			t.Fatalf("ANSI escape leaked into logs: %q", l.Text)
		}
		if l.Text == want {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected clean line %q in logs: %+v", want, logs)
	}
	ex, ok := m.LastExit("node")
	if !ok || len(ex.Tail) == 0 || ex.Tail[len(ex.Tail)-1] != want {
		t.Fatalf("unexpected exit tail: %+v", ex)
	}
	b, err := os.ReadFile(m.LogPath())
	if err != nil || !strings.Contains(string(b), want) || strings.Contains(string(b), "\x1b") {
		t.Fatalf("support log invalid (err=%v):\n%s", err, b)
	}
}

// A work loop that exits while the node runs is restarted (with a pause and a
// log line); StopWorker keeps it stopped.
func TestWorkerIsRestarted(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell script stand-in")
	}
	dir := t.TempDir()
	fake := filepath.Join(dir, "fake-nirad")
	// "start" keeps running (the node); anything else (the work loop) exits 1 at once
	_ = os.WriteFile(fake, []byte("#!/bin/sh\nif [ \"$1\" = start ]; then sleep 25; fi\nexit 1\n"), 0o755)
	m, err := NewManager(dir)
	if err != nil {
		t.Fatal(err)
	}
	m.Nirad = fake
	_ = m.SaveSettings(Settings{Network: "local"})
	m.pass = "password1"
	if err := m.StartNode(); err != nil {
		t.Fatal(err)
	}
	defer m.StopNode()
	if err := m.StartWorker("1anira"); err != nil {
		t.Fatal(err)
	}
	starts := func() int { return strings.Count(fmt.Sprint(m.Logs(100)), "started: nirad tx community run") }
	deadline := time.Now().Add(15 * time.Second)
	for starts() < 2 && time.Now().Before(deadline) {
		time.Sleep(200 * time.Millisecond)
	}
	if starts() < 2 || !strings.Contains(fmt.Sprint(m.Logs(100)), "restarting in 5s") {
		t.Fatalf("work loop not restarted; log: %v", m.Logs(20))
	}
	m.StopWorker()
	n := starts()
	time.Sleep(12 * time.Second)
	if starts() != n {
		t.Fatal("work loop restarted after StopWorker")
	}
}
