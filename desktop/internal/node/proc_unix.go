//go:build !windows

package node

import (
	"os"
	"path/filepath"
	"syscall"
)

// interrupt asks the process to shut down cleanly (nirad flushes its databases on SIGINT).
func interrupt(p *os.Process) { _ = p.Signal(syscall.SIGINT) }

func isNirad(image string) bool { return filepath.Base(image) == "nirad" }
