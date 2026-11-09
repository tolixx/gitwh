package git

import (
	"bytes"
	"context"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"
)

// maxHookLog is how much of the command output goes to the log (the tail)
const maxHookLog = 8 << 10

// Hook is a shell command run in a folder after a pull that moved HEAD
type Hook struct {
	Repo    string
	Command string
	Timeout time.Duration
}

// head returns the folder's HEAD commit, or "" if it cannot be read
func head(path string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", "rev-parse", "HEAD")
	cmd.Dir = path
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func (h Hook) run(path, before, after string) {
	start := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), h.Timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "/bin/sh", "-c", h.Command)
	cmd.Dir = path
	cmd.Env = append(os.Environ(),
		"GITWH_REPO="+h.Repo,
		"GITWH_FOLDER="+path,
		"GITWH_OLD_HEAD="+before,
		"GITWH_NEW_HEAD="+after,
	)
	killGroupOnCancel(cmd)
	// Do not wait forever for children that keep the output pipe open after the kill
	cmd.WaitDelay = 5 * time.Second

	out, err := cmd.CombinedOutput()
	out = bytes.TrimSpace(out)
	if len(out) > maxHookLog {
		out = append([]byte("..."), out[len(out)-maxHookLog:]...)
	}

	took := time.Since(start).Seconds()
	switch {
	case ctx.Err() == context.DeadlineExceeded:
		log.Printf("[%s] after_pull killed after %s\n%s", path, h.Timeout, out)
	case err != nil:
		log.Printf("[%s] after_pull failed in %.3fs: %v\n%s", path, took, err, out)
	default:
		log.Printf("[%s] after_pull done in %.3fs\n%s", path, took, out)
	}
}
