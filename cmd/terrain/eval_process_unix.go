//go:build unix

package main

import (
	"errors"
	"os"
	"os/exec"
	"syscall"
)

func configureEvalCommandForCancellation(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return os.ErrProcessDone
		}
		pgid, err := syscall.Getpgid(cmd.Process.Pid)
		if err != nil {
			if killErr := cmd.Process.Kill(); killErr != nil {
				return killErr
			}
			return nil
		}
		if err := syscall.Kill(-pgid, syscall.SIGKILL); err != nil {
			if errors.Is(err, syscall.ESRCH) {
				return os.ErrProcessDone
			}
			if killErr := cmd.Process.Kill(); killErr != nil {
				return killErr
			}
		}
		return nil
	}
}
