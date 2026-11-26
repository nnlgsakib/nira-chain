//go:build linux

package node

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
)

// hideWindow: the child gets SIGTERM if the app dies, so no orphan keeps the database.
func hideWindow(c *exec.Cmd) { c.SysProcAttr = &syscall.SysProcAttr{Pdeathsig: syscall.SIGTERM} }

func afterStart(*exec.Cmd) {}

// processImage returns the executable path of a running process, "" if gone.
func processImage(pid int) string {
	p, err := os.Readlink(filepath.Join("/proc", strconv.Itoa(pid), "exe"))
	if err != nil {
		return ""
	}
	return p
}

// pidsOf lists running processes whose executable is exactly path.
func pidsOf(path string) []int {
	ents, err := os.ReadDir("/proc")
	if err != nil {
		return nil
	}
	want := filepath.Clean(path)
	var out []int
	for _, e := range ents {
		pid, err := strconv.Atoi(e.Name())
		if err != nil {
			continue
		}
		if img := processImage(pid); img != "" && filepath.Clean(img) == want {
			out = append(out, pid)
		}
	}
	return out
}
