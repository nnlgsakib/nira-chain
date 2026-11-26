package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"sync"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"nira-chain/desktop/internal/node"
)

// App is bound to the interface (window.go.main.App). Methods return plain
// data or an error; the interface shows the error text.
//
// First run: there is no install record yet (node.ConfigPath), so the
// interface shows the install screen; Install picks the data folder, downloads
// and verifies nirad into it and writes the record. Every later start reads
// the record and opens the manager on that folder.
type App struct {
	ctx context.Context
	mu  sync.Mutex
	m   *node.Manager
}

var errNotInstalled = errors.New("the node is not installed yet")

func NewApp() *App {
	a := &App{}
	// Development override: NIRA_NODE_DIR uses that folder and nirad from PATH.
	if d := os.Getenv("NIRA_NODE_DIR"); d != "" {
		if m, err := node.NewManager(d); err == nil {
			a.m = m
		}
		return a
	}
	if in, ok := node.LoadInstall(); ok {
		a.open(in)
	}
	return a
}

func (a *App) open(in node.Install) {
	m, err := node.NewManager(in.DataDir)
	if err != nil {
		return
	}
	m.Nirad = in.NiradPath
	a.mu.Lock()
	a.m = m
	a.mu.Unlock()
}

func (a *App) mgr() (*node.Manager, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.m == nil {
		return nil, errNotInstalled
	}
	return a.m, nil
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	// a configured node starts with the app; the work loop waits for the password
	if m, err := a.mgr(); err == nil && m.Initialized() {
		_ = m.StartNode()
	}
}

// shutdown stops both processes cleanly so the node's databases are flushed.
func (a *App) shutdown(context.Context) {
	if m, err := a.mgr(); err == nil {
		m.StopNode()
	}
}

// ------------------------------------------------------------------ install

type InstallInfo struct {
	Installed    bool   `json:"installed"`
	DataDir      string `json:"data_dir"`
	DefaultDir   string `json:"default_dir"`
	ConfigPath   string `json:"config_path"`
	NiradVersion string `json:"nirad_version"` // installed, or the one that will be
	Platform     string `json:"platform"`
	Asset        string `json:"asset"`
	Supported    bool   `json:"supported"`
}

func (a *App) InstallInfo() InstallInfo {
	cfg, _ := node.ConfigPath()
	asset, err := node.AssetName(runtime.GOOS, runtime.GOARCH)
	info := InstallInfo{DefaultDir: node.DefaultDataDir(), ConfigPath: cfg, NiradVersion: node.NiradVersion,
		Platform: runtime.GOOS + "/" + runtime.GOARCH, Asset: asset, Supported: err == nil}
	if m, err := a.mgr(); err == nil {
		info.Installed, info.DataDir = true, m.Dir
		if in, ok := node.LoadInstall(); ok {
			info.NiradVersion = in.NiradVersion
		}
	}
	return info
}

// ChooseFolder opens the OS folder picker; "" when cancelled. The dialog
// refuses a start folder that does not exist (the suggested data folder does
// not until Install creates it), so it opens at the nearest existing parent.
func (a *App) ChooseFolder(start string) (string, error) {
	return wruntime.OpenDirectoryDialog(a.ctx, wruntime.OpenDialogOptions{
		Title: "Choose where Nira Node keeps its data", DefaultDirectory: existingParent(start), CanCreateDirectories: true,
	})
}

// existingParent returns dir if it exists, else its nearest existing ancestor,
// else "" (the dialog then uses its own default).
func existingParent(dir string) string {
	for dir != "" {
		if fi, err := os.Stat(dir); err == nil && fi.IsDir() {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
	return ""
}

// Install downloads and verifies nirad into dir and records it; progress goes
// out as install:progress events {stage, done, total}.
func (a *App) Install(dir string) error {
	in, err := node.InstallNirad(a.ctx, dir, func(stage string, done, total int64) {
		wruntime.EventsEmit(a.ctx, "install:progress", map[string]any{"stage": stage, "done": done, "total": total})
	})
	if err != nil {
		return err
	}
	a.open(in)
	return nil
}

// ------------------------------------------------------------------ node

func (a *App) Networks() []node.Network { return node.Networks() }

func (a *App) State() node.Status {
	m, err := a.mgr()
	if err != nil {
		return node.Status{}
	}
	return m.Status(a.ctx)
}

func (a *App) Settings() (node.Settings, error) {
	m, err := a.mgr()
	if err != nil {
		return node.Settings{}, err
	}
	return m.Settings(), nil
}

type SystemInfo struct {
	CPUs    int    `json:"cpus"`
	OS      string `json:"os"`
	DataDir string `json:"data_dir"`
}

func (a *App) System() SystemInfo {
	s := SystemInfo{CPUs: runtime.NumCPU(), OS: runtime.GOOS}
	if m, err := a.mgr(); err == nil {
		s.DataDir = m.Dir
	}
	return s
}

func (a *App) SaveLimits(cpus, memMB int) error {
	m, err := a.mgr()
	if err != nil {
		return err
	}
	if cpus < 0 || cpus > runtime.NumCPU() {
		return errors.New("CPU cores must be between 1 and the number of cores")
	}
	if memMB != 0 && memMB < 1024 {
		return errors.New("give the node at least 1024 MB")
	}
	s := m.Settings()
	s.MaxCPUs, s.MaxMemoryMB = cpus, memMB
	return m.SaveSettings(s)
}

// Setup configures the node and starts it; progress goes out as setup:step events.
func (a *App) Setup(network, moniker string) error {
	m, err := a.mgr()
	if err != nil {
		return err
	}
	m.StopNode()
	if err := m.Setup(a.ctx, network, moniker, func(step string) { wruntime.EventsEmit(a.ctx, "setup:step", step) }); err != nil {
		return err
	}
	return m.StartNode()
}

func (a *App) CreateKey(pass string) (string, error) {
	m, err := a.mgr()
	if err != nil {
		return "", err
	}
	return m.CreateKey(pass)
}

func (a *App) ImportKey(mnemonic, pass string) (string, error) {
	m, err := a.mgr()
	if err != nil {
		return "", err
	}
	return m.ImportKey(mnemonic, pass)
}

func (a *App) ImportPrivateKey(key, pass string) (string, error) {
	m, err := a.mgr()
	if err != nil {
		return "", err
	}
	return m.ImportPrivateKey(key, pass)
}

func (a *App) Unlock(pass string) error {
	m, err := a.mgr()
	if err != nil {
		return err
	}
	return m.Unlock(pass)
}

func (a *App) Lock() {
	if m, err := a.mgr(); err == nil {
		m.Lock()
	}
}

func (a *App) StartNode() error {
	m, err := a.mgr()
	if err != nil {
		return err
	}
	return m.StartNode()
}

func (a *App) StopNode() {
	if m, err := a.mgr(); err == nil {
		m.StopNode()
	}
}

func (a *App) StopWork() {
	if m, err := a.mgr(); err == nil {
		m.StopWorker()
	}
}

func (a *App) StartWork() error {
	m, err := a.mgr()
	if err != nil {
		return err
	}
	gp, err := m.GasPrice(a.ctx)
	if err != nil {
		return errors.New("the node is not ready yet")
	}
	return m.StartWorker(gp)
}

func (a *App) Register(bondNira int64) (string, error) {
	m, err := a.mgr()
	if err != nil {
		return "", err
	}
	return m.Register(a.ctx, bondNira)
}

func (a *App) RotateOnion() (string, error) {
	m, err := a.mgr()
	if err != nil {
		return "", err
	}
	return m.RotateOnion(a.ctx)
}

func (a *App) Deregister() (string, error) {
	m, err := a.mgr()
	if err != nil {
		return "", err
	}
	return m.Deregister(a.ctx)
}

func (a *App) Logs(n int) []node.LogLine {
	if m, err := a.mgr(); err == nil {
		return m.Logs(n)
	}
	return nil
}
