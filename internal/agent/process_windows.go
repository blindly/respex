//go:build windows

package agent

import (
	"os/exec"
	"strconv"
	"syscall"

	"golang.org/x/sys/windows"
)

func configureProcess(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: windows.CREATE_NO_WINDOW}
}

func interruptProcess(_ *exec.Cmd) bool {
	return false
}

func killProcess(cmd *exec.Cmd) error {
	if err := exec.Command("taskkill", "/PID", strconv.Itoa(cmd.Process.Pid), "/T", "/F").Run(); err == nil {
		return nil
	}
	return cmd.Process.Kill()
}
