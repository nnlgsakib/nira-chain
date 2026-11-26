//go:build windows

package node

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// hideWindow keeps nirad from opening a console window next to the app.
func hideWindow(c *exec.Cmd) {
	c.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000} // CREATE_NO_WINDOW
}

// interrupt: Windows has no SIGINT for a process without a console; nirad's
// databases are crash-safe, so a kill is acceptable here.
func interrupt(p *os.Process) { _ = p.Kill() }

// All children go into one job object that kills them when its last handle
// closes, i.e. when the app exits or crashes. Without it a nirad started by a
// crashed app kept the node database open and the next start failed with
// "The process cannot access the file because it is being used by another process".
var (
	jobOnce sync.Once
	job     windows.Handle
)

func killOnExitJob() windows.Handle {
	jobOnce.Do(func() {
		h, err := windows.CreateJobObject(nil, nil)
		if err != nil {
			return
		}
		info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
		info.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
		if _, err := windows.SetInformationJobObject(h, windows.JobObjectExtendedLimitInformation,
			uintptr(unsafe.Pointer(&info)), uint32(unsafe.Sizeof(info))); err != nil {
			windows.CloseHandle(h)
			return
		}
		job = h // never closed: the OS closes it when the app process ends
	})
	return job
}

// afterStart ties the child's lifetime to the app's.
func afterStart(c *exec.Cmd) {
	j := killOnExitJob()
	if j == 0 || c.Process == nil {
		return
	}
	h, err := windows.OpenProcess(windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE, false, uint32(c.Process.Pid))
	if err != nil {
		return
	}
	defer windows.CloseHandle(h)
	_ = windows.AssignProcessToJobObject(j, h)
}

// processImage returns the executable path of a running process, "" if gone.
func processImage(pid int) string {
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		return ""
	}
	defer windows.CloseHandle(h)
	buf := make([]uint16, windows.MAX_PATH)
	n := uint32(len(buf))
	if err := windows.QueryFullProcessImageName(h, 0, &buf[0], &n); err != nil {
		return ""
	}
	return windows.UTF16ToString(buf[:n])
}

func isNirad(image string) bool {
	return strings.EqualFold(filepath.Base(image), "nirad.exe")
}

// pidsOf lists running processes whose executable is exactly path.
func pidsOf(path string) []int {
	snap, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return nil
	}
	defer windows.CloseHandle(snap)
	var e windows.ProcessEntry32
	e.Size = uint32(unsafe.Sizeof(e))
	var out []int
	for err = windows.Process32First(snap, &e); err == nil; err = windows.Process32Next(snap, &e) {
		if !strings.EqualFold(windows.UTF16ToString(e.ExeFile[:]), filepath.Base(path)) {
			continue
		}
		if img := processImage(int(e.ProcessID)); img != "" && strings.EqualFold(filepath.Clean(img), filepath.Clean(path)) {
			out = append(out, int(e.ProcessID))
		}
	}
	return out
}
