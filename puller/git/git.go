package git

import (
	"context"
	"fmt"
	"gitwh/puller"
	"os/exec"
	"sync"
	"time"
)

const defaultGitTimeout = 10

type simplePuller struct {
	mutexes    map[string]*sync.Mutex
	lock       *sync.Mutex
	gitTimeout int
}

// New creates simple gitpuller ( now with mutex per dir )
func New(timeout int) puller.Puller {
	return &simplePuller{mutexes: make(map[string]*sync.Mutex), lock: &sync.Mutex{}, gitTimeout: timeout}
}

func (p *simplePuller) getMutex(path string) *sync.Mutex {
	p.lock.Lock()
	defer p.lock.Unlock()

	if _, ok := p.mutexes[path]; !ok {
		p.mutexes[path] = &sync.Mutex{}
	}
	return p.mutexes[path]
}

func (p *simplePuller) mutexedPull(paths []string) {
	if paths == nil {
		fmt.Printf("mutexedPull: empty path")
		return
	}
	for _, path := range paths {
		p.pullPath(path)
	}
}

func (p *simplePuller) pullPath(path string) {
	m := p.getMutex(path)
	m.Lock()
	defer m.Unlock()

	start := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(p.gitTimeout)*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", "pull")
	cmd.Dir = path

	if err := cmd.Run(); err != nil {
		fmt.Printf("PullPath %s: Git pull returned error : %v\n", path, err)
		return
	}

	fmt.Printf("[%s] Git pull done in %.3f\n", path, time.Now().Sub(start).Seconds())
}

func (p *simplePuller) Pull(path []string) error {
	go p.mutexedPull(path)
	return nil
}
