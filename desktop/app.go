package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"nira-chain/desktop/internal/node"
)

// App is bound to the interface (window.go.main.App). Methods return plain
// data or an error; the interface shows the error text.
type App struct {
	ctx context.Context
	m   *node.Manager
}

func dataDir() string {
	if d := os.Getenv("NIRA_NODE_DIR"); d != "" {
		return d
	}
	base, err := os.UserConfigDir()
	if err != nil {
		base = os.TempDir()
	}
	return filepath.Join(base, "NiraNode")
}

func NewApp() (*App, error) {
	m, err := node.NewManager(dataDir())
	if err != nil {
		return nil, err
	}
	return &App{m: m}, nil
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	// a configured node starts with the app; the work loop waits for the password
	if a.m.Initialized() {
		_ = a.m.StartNode()
	}
}

// shutdown stops both processes cleanly so the node's databases are flushed.
func (a *App) shutdown(context.Context) { a.m.StopNode() }

func (a *App) Networks() []node.Network { return node.Networks() }
func (a *App) State() node.Status       { return a.m.Status(a.ctx) }
func (a *App) Settings() node.Settings  { return a.m.Settings() }

type SystemInfo struct {
	CPUs    int    `json:"cpus"`
	OS      string `json:"os"`
	DataDir string `json:"data_dir"`
}

func (a *App) System() SystemInfo {
	return SystemInfo{CPUs: runtime.NumCPU(), OS: runtime.GOOS, DataDir: a.m.Dir}
}

func (a *App) SaveLimits(cpus, memMB int) error {
	if cpus < 0 || cpus > runtime.NumCPU() {
		return errors.New("CPU cores must be between 1 and the number of cores")
	}
	if memMB != 0 && memMB < 1024 {
		return errors.New("give the node at least 1024 MB")
	}
	s := a.m.Settings()
	s.MaxCPUs, s.MaxMemoryMB = cpus, memMB
	return a.m.SaveSettings(s)
}

// Setup configures the node and starts it; progress goes out as setup:step events.
func (a *App) Setup(network, moniker string) error {
	a.m.StopNode()
	err := a.m.Setup(a.ctx, network, moniker, func(step string) { wruntime.EventsEmit(a.ctx, "setup:step", step) })
	if err != nil {
		return err
	}
	return a.m.StartNode()
}

func (a *App) CreateKey(pass string) (string, error)           { return a.m.CreateKey(pass) }
func (a *App) ImportKey(mnemonic, pass string) (string, error) { return a.m.ImportKey(mnemonic, pass) }
func (a *App) Unlock(pass string) error                        { return a.m.Unlock(pass) }
func (a *App) Lock()                                           { a.m.Lock() }
func (a *App) StartNode() error                                { return a.m.StartNode() }
func (a *App) StopNode()                                       { a.m.StopNode() }
func (a *App) StopWork()                                       { a.m.StopWorker() }

func (a *App) StartWork() error {
	gp, err := a.m.GasPrice(a.ctx)
	if err != nil {
		return errors.New("the node is not ready yet")
	}
	return a.m.StartWorker(gp)
}

func (a *App) Register(bondNira int64) (string, error) { return a.m.Register(a.ctx, bondNira) }
func (a *App) Deregister() (string, error)             { return a.m.Deregister(a.ctx) }
func (a *App) Logs(n int) []node.LogLine               { return a.m.Logs(n) }
