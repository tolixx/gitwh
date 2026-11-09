package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func gitRun(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

// setup makes origin, a work clone that pushes, and a deploy clone gitwh pulls into
func setup(t *testing.T) (work, deploy string) {
	root := t.TempDir()
	origin := filepath.Join(root, "origin.git")
	work = filepath.Join(root, "work")
	deploy = filepath.Join(root, "deploy")
	gitRun(t, root, "init", "-q", "--bare", origin)
	gitRun(t, root, "clone", "-q", origin, work)
	gitRun(t, work, "commit", "-q", "--allow-empty", "-m", "one")
	gitRun(t, work, "push", "-q", "origin", "HEAD")
	gitRun(t, root, "clone", "-q", origin, deploy)
	return work, deploy
}

func push(t *testing.T, work, msg string) {
	gitRun(t, work, "commit", "-q", "--allow-empty", "-m", msg)
	gitRun(t, work, "push", "-q", "origin", "HEAD")
}

func TestAfterPullRunsOnNewCommits(t *testing.T) {
	work, deploy := setup(t)
	out := filepath.Join(t.TempDir(), "out")
	hook := Hook{
		Repo:    "r",
		Command: `echo "$GITWH_REPO $(pwd -P) $GITWH_OLD_HEAD $GITWH_NEW_HEAD" >> ` + out,
		Timeout: 10 * time.Second,
	}
	p := NewWithHooks(10, map[string]Hook{deploy: hook}).(*simplePuller)

	before := head(deploy)
	push(t, work, "two")
	p.pullPath(deploy)
	after := head(deploy)
	if after == before {
		t.Fatal("pull did not move HEAD")
	}

	got, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("after_pull did not run: %v", err)
	}
	real, _ := filepath.EvalSymlinks(deploy)
	want := "r " + real + " " + before + " " + after
	if strings.TrimSpace(string(got)) != want {
		t.Errorf("got %q\nwant %q", strings.TrimSpace(string(got)), want)
	}

	// Nothing new: the command must not run again
	p.pullPath(deploy)
	got, _ = os.ReadFile(out)
	if n := strings.Count(string(got), "\n"); n != 1 {
		t.Errorf("after_pull ran %d times, want 1", n)
	}
}

func TestAfterPullNotRunWhenPullFails(t *testing.T) {
	dir := t.TempDir() // not a git repo
	out := filepath.Join(t.TempDir(), "out")
	p := NewWithHooks(10, map[string]Hook{dir: {Command: "touch " + out, Timeout: time.Second}}).(*simplePuller)
	p.pullPath(dir)
	if _, err := os.Stat(out); err == nil {
		t.Error("after_pull ran after a failed pull")
	}
}

func TestAfterPullTimeoutKillsChildren(t *testing.T) {
	work, deploy := setup(t)
	marker := filepath.Join(t.TempDir(), "survived")
	// The child sleeps past the timeout and would write the marker if it survived the kill
	hook := Hook{Command: "(sleep 2; touch " + marker + ") & wait", Timeout: 300 * time.Millisecond}
	p := NewWithHooks(10, map[string]Hook{deploy: hook}).(*simplePuller)

	push(t, work, "two")
	start := time.Now()
	p.pullPath(deploy)
	if took := time.Since(start); took > 1500*time.Millisecond {
		t.Errorf("pullPath took %s, the timeout did not stop the command", took)
	}
	time.Sleep(2500 * time.Millisecond)
	if _, err := os.Stat(marker); err == nil {
		t.Error("child process outlived the after_pull timeout")
	}
}

func TestNoHookNoHeadCheck(t *testing.T) {
	work, deploy := setup(t)
	p := New(10).(*simplePuller)
	push(t, work, "two")
	p.pullPath(deploy) // must just pull
	gitRun(t, deploy, "log", "-1", "--format=%s")
}
