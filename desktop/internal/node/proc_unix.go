//go:build !windows

package node

import (
	"os"
	"os/exec"
	"syscall"
)

func hideWindow(*exec.Cmd) {}

// interrupt asks the process to shut down cleanly (nirad flushes its databases on SIGINT).
func interrupt(p *os.Process) { _ = p.Signal(syscall.SIGINT) }
