//go:build !unix

package main

import (
	"os"
	"os/exec"
)

func configureEvalCommandForCancellation(cmd *exec.Cmd) {
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return os.ErrProcessDone
		}
		return cmd.Process.Kill()
	}
}
