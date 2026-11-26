package node

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

// KeyName is the operator key's name in the node's keyring.
const KeyName = "operator"

// Settings is what the app remembers between runs (settings.json in the app dir).
type Settings struct {
	Network  string `json:"network"`
	Moniker  string `json:"moniker"`
	Operator string `json:"operator"` // bech32 nira1…, known after key creation
	// Resource limits for the node process.
	MaxCPUs      int  `json:"max_cpus"`      // GOMAXPROCS; 0 = half the cores
	MaxMemoryMB  int  `json:"max_memory_mb"` // GOMEMLIMIT soft limit; 0 = 4096
	StartAtLogin bool `json:"start_at_login"`
}

// Manager owns one node home and its two processes: `nirad start` and the
// community work loop (`nirad tx community run`).
type Manager struct {
	Dir   string // app data dir: settings.json, homes/<network>
	Nirad string // path to the nirad binary

	mu       sync.Mutex
	settings Settings
	node     *proc
	worker   *proc
	pass     string // keyring passphrase while unlocked; never written to disk
	logs     *ring
	exits    map[string]ExitInfo // last exit of each process ("node", "work")
}

// ExitInfo says why a process stopped: its error and its last output lines.
type ExitInfo struct {
	At    time.Time `json:"at"`
	Error string    `json:"error"`
	Tail  []string  `json:"tail"`
}

// LastExit returns how the given process ("node" or "work") last stopped.
func (m *Manager) LastExit(tag string) (ExitInfo, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	e, ok := m.exits[tag]
	return e, ok
}

// LogPath is the file both processes' output is appended to (for support).
func (m *Manager) LogPath() string { return filepath.Join(m.Dir, "nira-node.log") }

// NewManager loads settings from dir and locates nirad (next to the app
// executable first, then PATH).
func NewManager(dir string) (*Manager, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	m := &Manager{Dir: dir, logs: newRing(400), exits: map[string]ExitInfo{}}
	if b, err := os.ReadFile(filepath.Join(dir, "settings.json")); err == nil {
		_ = json.Unmarshal(b, &m.settings)
	}
	m.Nirad = findNirad()
	return m, nil
}

func findNirad() string {
	name := "nirad"
	if runtime.GOOS == "windows" {
		name = "nirad.exe"
	}
	if exe, err := os.Executable(); err == nil {
		p := filepath.Join(filepath.Dir(exe), name)
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	if p, err := exec.LookPath(name); err == nil {
		return p
	}
	return ""
}

// Settings returns a copy of the current settings.
func (m *Manager) Settings() Settings {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.settings
}

// SaveSettings updates and persists settings.
func (m *Manager) SaveSettings(s Settings) error {
	m.mu.Lock()
	m.settings = s
	m.mu.Unlock()
	b, _ := json.MarshalIndent(s, "", "  ")
	return os.WriteFile(filepath.Join(m.Dir, "settings.json"), b, 0o600)
}

// Home is the node home of the selected network.
func (m *Manager) Home() string {
	return filepath.Join(m.Dir, "homes", m.Settings().Network)
}

// Network is the selected network preset.
func (m *Manager) Network() (Network, error) {
	n, ok := NetworkByID(m.Settings().Network)
	if !ok {
		return n, errors.New("no network selected")
	}
	return n, nil
}

// Unlock keeps the keyring passphrase in memory for signing.
func (m *Manager) Unlock(pass string) error {
	if len(pass) < 8 {
		return errors.New("the password must have at least 8 characters")
	}
	// prove it opens the keyring
	if _, err := m.run(context.Background(), pass+"\n", "keys", "show", KeyName, "-a", "--keyring-backend", "file", "--home", m.Home()); err != nil {
		return errors.New("wrong password")
	}
	m.mu.Lock()
	m.pass = pass
	m.mu.Unlock()
	return nil
}

// Locked reports whether signing needs the password again.
func (m *Manager) Locked() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.pass == ""
}

// Lock forgets the passphrase (the work loop keeps running with the key it opened).
func (m *Manager) Lock() {
	m.mu.Lock()
	m.pass = ""
	m.mu.Unlock()
}

// run executes nirad with stdin and returns combined output.
func (m *Manager) run(ctx context.Context, stdin string, args ...string) (string, error) {
	if m.Nirad == "" {
		return "", errors.New("nirad not found: put it next to the app or on PATH")
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, m.Nirad, args...)
	hideWindow(cmd)
	cmd.Stdin = strings.NewReader(stdin)
	var out bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &out
	err := cmd.Run()
	s := out.String()
	if err != nil {
		return s, fmt.Errorf("nirad %s: %v: %s", args[0], err, lastLines(s, 3))
	}
	return s, nil
}

func lastLines(s string, n int) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, " | ")
}

// ----------------------------------------------------------------- processes

type proc struct {
	cmd  *exec.Cmd
	done chan struct{}
	err  error
}

func (p *proc) running() bool {
	if p == nil {
		return false
	}
	select {
	case <-p.done:
		return false
	default:
		return true
	}
}

// env applies the resource limits (Go runtime limits work on every OS).
// The caller holds m.mu (spawn runs inside StartNode/StartWorker), so it reads
// m.settings directly; calling m.Settings() here would deadlock.
func (m *Manager) env() []string {
	s := m.settings
	cpus := s.MaxCPUs
	if cpus <= 0 {
		cpus = max(1, runtime.NumCPU()/2)
	}
	mem := s.MaxMemoryMB
	if mem <= 0 {
		mem = 4096
	}
	return append(os.Environ(), fmt.Sprintf("GOMAXPROCS=%d", cpus), fmt.Sprintf("GOMEMLIMIT=%dMiB", mem))
}

func (m *Manager) spawn(tag, stdin string, args ...string) (*proc, error) {
	if m.Nirad == "" {
		return nil, errors.New("nirad not found")
	}
	cmd := exec.Command(m.Nirad, args...)
	hideWindow(cmd)
	cmd.Env = m.env()
	if stdin != "" {
		cmd.Stdin = strings.NewReader(stdin)
	}
	r, w := io.Pipe()
	cmd.Stdout, cmd.Stderr = w, w
	logf := m.openLog()
	if err := cmd.Start(); err != nil {
		if logf != nil {
			logf.Close()
		}
		return nil, err
	}
	m.logs.add(tag, "started: nirad "+strings.Join(args, " "))
	p := &proc{cmd: cmd, done: make(chan struct{})}
	scanned := make(chan []string)
	go func() {
		var tail []string
		sc := bufio.NewScanner(r)
		sc.Buffer(make([]byte, 64*1024), 1024*1024)
		for sc.Scan() {
			line := sc.Text()
			m.logs.add(tag, line)
			if logf != nil {
				fmt.Fprintf(logf, "%s [%s] %s\n", time.Now().Format(time.RFC3339), tag, line)
			}
			if tail = append(tail, line); len(tail) > 12 {
				tail = tail[1:]
			}
		}
		scanned <- tail
	}()
	go func() {
		p.err = cmd.Wait()
		w.Close()
		tail := <-scanned
		msg := "exited normally"
		if p.err != nil {
			msg = p.err.Error()
		}
		m.logs.add(tag, "process exited: "+msg)
		if logf != nil {
			fmt.Fprintf(logf, "%s [%s] process exited: %s\n", time.Now().Format(time.RFC3339), tag, msg)
			logf.Close()
		}
		m.mu.Lock()
		m.exits[tag] = ExitInfo{At: time.Now(), Error: msg, Tail: tail}
		m.mu.Unlock()
		close(p.done)
	}()
	return p, nil
}

// openLog opens the support log for appending, starting it over past 20 MB.
func (m *Manager) openLog() *os.File {
	if fi, err := os.Stat(m.LogPath()); err == nil && fi.Size() > 20<<20 {
		_ = os.Remove(m.LogPath())
	}
	f, err := os.OpenFile(m.LogPath(), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return nil
	}
	return f
}

// StartNode starts `nirad start` for the selected network.
func (m *Manager) StartNode() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.node.running() {
		return nil
	}
	if err := portsFree(); err != nil {
		return err
	}
	p, err := m.spawn("node", "", "start", "--home", filepath.Join(m.Dir, "homes", m.settings.Network))
	if err != nil {
		return err
	}
	m.node = p
	return nil
}

// StopNode stops the work loop and the node (gracefully where the OS allows).
func (m *Manager) StopNode() {
	m.StopWorker()
	m.mu.Lock()
	p := m.node
	m.mu.Unlock()
	stop(p)
}

// StartWorker starts the community work loop. It needs the password once: the
// keyring stays open inside that process.
func (m *Manager) StartWorker(gasPrice string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.worker.running() {
		return nil
	}
	if m.pass == "" {
		return errors.New("locked: enter your password")
	}
	n, _ := NetworkByID(m.settings.Network)
	home := filepath.Join(m.Dir, "homes", m.settings.Network)
	p, err := m.spawn("work", m.pass+"\n", "tx", "community", "run", "--from", KeyName,
		"--keyring-backend", "file", "--home", home, "--chain-id", n.ChainID,
		"--node", fmt.Sprintf("tcp://127.0.0.1:%d", PortRPC), "--gas", "auto", "--gas-adjustment", "1.5",
		"--gas-prices", gasPrice, "-y")
	if err != nil {
		return err
	}
	m.worker = p
	return nil
}

// StopWorker stops the work loop.
func (m *Manager) StopWorker() {
	m.mu.Lock()
	p := m.worker
	m.mu.Unlock()
	stop(p)
}

func stop(p *proc) {
	if !p.running() {
		return
	}
	interrupt(p.cmd.Process)
	select {
	case <-p.done:
	case <-time.After(20 * time.Second):
		_ = p.cmd.Process.Kill()
		<-p.done
	}
}

// NodeRunning / WorkerRunning report process state.
func (m *Manager) NodeRunning() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.node.running()
}

func (m *Manager) WorkerRunning() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.worker.running()
}

// Logs returns the latest log lines of both processes.
func (m *Manager) Logs(n int) []LogLine { return m.logs.last(n) }

// ----------------------------------------------------------------- log ring

// LogLine is one line of process output.
type LogLine struct {
	Time   time.Time `json:"time"`
	Source string    `json:"source"` // node | work
	Text   string    `json:"text"`
}

type ring struct {
	mu    sync.Mutex
	lines []LogLine
	size  int
}

func newRing(n int) *ring { return &ring{size: n} }

func (r *ring) add(src, text string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.lines = append(r.lines, LogLine{time.Now(), src, text})
	if len(r.lines) > r.size {
		r.lines = r.lines[len(r.lines)-r.size:]
	}
}

func (r *ring) last(n int) []LogLine {
	r.mu.Lock()
	defer r.mu.Unlock()
	if n > len(r.lines) {
		n = len(r.lines)
	}
	return append(make([]LogLine, 0, n), r.lines[len(r.lines)-n:]...) // never nil: the UI expects a list
}

// portsFree checks that the node's ports can be opened before starting it, so a
// clash (another program, or a range Windows reserves for Hyper-V/WSL) is
// reported by name instead of the node exiting a moment after it starts.
func portsFree() error {
	for _, p := range []struct {
		name string
		addr string
	}{
		{"peer (P2P)", fmt.Sprintf("0.0.0.0:%d", PortP2P)},
		{"RPC", fmt.Sprintf("127.0.0.1:%d", PortRPC)},
		{"REST API", fmt.Sprintf("127.0.0.1:%d", PortREST)},
		{"gRPC", fmt.Sprintf("127.0.0.1:%d", PortGRPC)},
		{"metrics", fmt.Sprintf("127.0.0.1:%d", PortPrometheus)},
	} {
		l, err := net.Listen("tcp", p.addr)
		if err != nil {
			hint := "another program is using it"
			if runtime.GOOS == "windows" {
				hint = "another program is using it, or Windows reserves it (check: netsh interface ipv4 show excludedportrange protocol=tcp)"
			}
			return fmt.Errorf("the node's %s port %s can't be opened: %s (%v)", p.name, p.addr, hint, err)
		}
		_ = l.Close()
	}
	return nil
}
