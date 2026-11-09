package git

import (
	"bytes"
	"context"
	"log"
	"os"
	"os/exec"
	"sync"
	"time"

	"gitwh/puller"
)

const defaultGitTimeout = 10

// dirState tracks one folder: at most one git pull runs for it, and pushes that
// arrive meanwhile collapse into a single follow-up pull
type dirState struct {
	mu      sync.Mutex
	running bool
	pending bool
}

type simplePuller struct {
	dirs       map[string]*dirState
	lock       *sync.Mutex
	gitTimeout int
	// hooks maps a folder to the command run after a pull that brought new commits
	hooks map[string]Hook
	// run executes the pull, swapped in tests
	run func(path string)
}

// New creates a git puller: one worker per folder, repeated pushes coalesced
func New(timeout int) puller.Puller {
	return NewWithHooks(timeout, nil)
}

// NewWithHooks is New plus an after-pull command per folder
func NewWithHooks(timeout int, hooks map[string]Hook) puller.Puller {
	if timeout <= 0 {
		timeout = defaultGitTimeout
	}
	p := &simplePuller{dirs: make(map[string]*dirState), lock: &sync.Mutex{}, gitTimeout: timeout, hooks: hooks}
	p.run = p.pullPath
	return p
}

func (p *simplePuller) getState(path string) *dirState {
	p.lock.Lock()
	defer p.lock.Unlock()

	s, ok := p.dirs[path]
	if !ok {
		s = &dirState{}
		p.dirs[path] = s
	}
	return s
}

func (p *simplePuller) schedule(path string) {
	s := p.getState(path)
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		s.pending = true
		return
	}
	s.running = true
	go p.worker(path, s)
}

func (p *simplePuller) worker(path string, s *dirState) {
	for {
		p.run(path)

		s.mu.Lock()
		if !s.pending {
			s.running = false
			s.mu.Unlock()
			return
		}
		s.pending = false
		s.mu.Unlock()
	}
}

func (p *simplePuller) pullPath(path string) {
	hook, hasHook := p.hooks[path]
	before := ""
	if hasHook {
		before = head(path)
	}

	if !p.gitPull(path) || !hasHook {
		return
	}

	after := head(path)
	if after == "" || after == before {
		log.Printf("[%s] no new commits, after_pull skipped", path)
		return
	}
	hook.run(path, before, after)
}

func (p *simplePuller) gitPull(path string) bool {
	start := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(p.gitTimeout)*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", "pull")
	cmd.Dir = path
	// Fail at once instead of waiting for a password nobody will type
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")

	out, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("[%s] git pull failed: %v\n%s", path, err, bytes.TrimSpace(out))
		return false
	}

	log.Printf("[%s] git pull done in %.3fs", path, time.Since(start).Seconds())
	return true
}

// Pull starts a pull for each folder and returns at once
func (p *simplePuller) Pull(paths []string) error {
	if len(paths) == 0 {
		log.Printf("Pull: no folders")
		return nil
	}
	for _, path := range paths {
		p.schedule(path)
	}
	return nil
}
