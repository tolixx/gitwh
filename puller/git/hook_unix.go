//go:build unix

package git

import (
	"os/exec"
	"syscall"
)

// killGroupOnCancel puts the command in its own process group and kills the
// whole group on timeout, so a script's children do not outlive it
func killGroupOnCancel(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
}
