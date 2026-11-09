//go:build !unix

package git

import "os/exec"

func killGroupOnCancel(cmd *exec.Cmd) {}
