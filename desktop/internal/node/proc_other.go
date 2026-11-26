//go:build !windows && !linux

package node

import (
	"os/exec"
	"strconv"
	"strings"
)

func hideWindow(*exec.Cmd) {}
func afterStart(*exec.Cmd) {}

// processImage asks ps for the command of a running process, "" if gone.
func processImage(pid int) string {
	out, err := exec.Command("ps", "-o", "comm=", "-p", strconv.Itoa(pid)).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// pidsOf lists running processes started from path (via pgrep).
func pidsOf(path string) []int {
	out, err := exec.Command("pgrep", "-f", "^"+path+" ").Output()
	if err != nil {
		return nil
	}
	var pids []int
	for _, f := range strings.Fields(string(out)) {
		if p, err := strconv.Atoi(f); err == nil {
			pids = append(pids, p)
		}
	}
	return pids
}
