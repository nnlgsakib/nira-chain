//go:build windows

package node

import (
	"os"
	"os/exec"
	"syscall"
)

// hideWindow keeps nirad from opening a console window next to the app.
func hideWindow(c *exec.Cmd) {
	c.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000} // CREATE_NO_WINDOW
}

// interrupt: Windows has no SIGINT for a process without a console; nirad's
// databases are crash-safe, so a kill is acceptable here.
func interrupt(p *os.Process) { _ = p.Kill() }
