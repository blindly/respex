//go:build windows

package agent

import "os/exec"

func configureProcess(_ *exec.Cmd) {}

func killProcess(cmd *exec.Cmd) error {
	return cmd.Process.Kill()
}
