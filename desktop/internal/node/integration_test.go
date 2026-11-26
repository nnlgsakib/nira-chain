//go:build integration

// End to end against a running localnet (scripts/localnet.sh + `nirad start`):
// the app's own code sets up a lean node next to the validator, syncs it,
// creates a key, gets funded, registers, runs the work loop and earns units.
//
//	go test -tags integration ./internal/node -run TestDesktopEndToEnd -v -timeout 20m
package node

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func TestDesktopEndToEnd(t *testing.T) {
	ctx := context.Background()
	m, err := NewManager(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if m.Nirad == "" {
		t.Skip("nirad not on PATH")
	}
	var steps []string
	if err := m.Setup(ctx, "local", "desktop-e2e", func(s string) { steps = append(steps, s) }); err != nil {
		t.Fatalf("setup: %v (steps %v)", err, steps)
	}
	t.Logf("setup steps: %s", strings.Join(steps, " → "))
	if !m.Initialized() {
		t.Fatal("not initialized after setup")
	}
	app, _ := os.ReadFile(m.Home() + "/config/app.toml")
	for _, want := range []string{`pruning = "custom"`, `pruning-keep-recent = "2000"`, "min-retain-blocks = 2000"} {
		if !strings.Contains(string(app), want) {
			t.Fatalf("app.toml lacks %s", want)
		}
	}

	if err := m.StartNode(); err != nil {
		t.Fatal(err)
	}
	defer m.StopNode()
	waitFor(t, 3*time.Minute, "node synced", func() bool {
		st := m.Status(ctx)
		return st.Reachable && !st.CatchingUp && st.Height > 2 && st.Peers >= 1
	})

	// key: created through the app, then funded from dev1 on the validator node
	mnemonic, err := m.CreateKey("desktop-e2e-pass")
	if err != nil || len(strings.Fields(mnemonic)) != 24 {
		t.Fatalf("create key: %v (%d words)", err, len(strings.Fields(mnemonic)))
	}
	op := m.Settings().Operator
	m.Lock()
	if err := m.Unlock("wrong-password"); err == nil {
		t.Fatal("wrong password accepted")
	}
	if err := m.Unlock("desktop-e2e-pass"); err != nil {
		t.Fatal(err)
	}
	fund := exec.Command(m.Nirad, "tx", "bank", "send", "dev1", op, "2000000000000000000000anira",
		"--keyring-backend", "test", "--chain-id", "nira_93000-1", "--gas", "auto", "--gas-adjustment", "1.5",
		"--gas-prices", "100000000000anira", "-y")
	if out, err := fund.CombinedOutput(); err != nil {
		t.Fatalf("fund: %v %s", err, out)
	}
	waitFor(t, time.Minute, "operator funded", func() bool { return m.Status(ctx).Balance != "" && m.Status(ctx).Balance != "0" })

	if _, err := m.Register(ctx, 1000); err != nil {
		t.Fatalf("register: %v", err)
	}
	waitFor(t, time.Minute, "node ACTIVE", func() bool { return m.Status(ctx).NodeStatus == "ACTIVE" })

	gp, err := m.GasPrice(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := m.StartWorker(gp); err != nil {
		t.Fatal(err)
	}
	waitFor(t, 6*time.Minute, "units earned by the work loop", func() bool {
		u := m.Status(ctx).UnitsTotal
		return u != "" && u != "0"
	})
	st := m.Status(ctx)
	t.Logf("units %s, rewards %s anira, disk %d bytes", st.UnitsTotal, st.RewardsTotal, st.DiskBytes)
	if !st.WorkerRunning || !st.NodeRunning {
		t.Fatalf("processes stopped: %+v", st)
	}
	for _, l := range m.Logs(5) {
		t.Logf("[%s] %s", l.Source, l.Text)
	}
}

func waitFor(t *testing.T, d time.Duration, what string, ok func() bool) {
	t.Helper()
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		if ok() {
			return
		}
		time.Sleep(2 * time.Second)
	}
	t.Fatalf("timed out waiting for %s", what)
}

var _ = fmt.Sprint
