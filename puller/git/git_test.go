package git

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestNew(t *testing.T) {
	timeout := 15
	puller := New(timeout)

	if puller == nil {
		t.Error("Expected puller to be created")
	}

	sp, ok := puller.(*simplePuller)
	if !ok {
		t.Error("Expected simplePuller type")
	}

	if sp.gitTimeout != timeout {
		t.Errorf("Expected gitTimeout %d, got %d", timeout, sp.gitTimeout)
	}

	if sp.dirs == nil {
		t.Error("Expected dirs map to be initialized")
	}

	if sp.lock == nil {
		t.Error("Expected lock to be initialized")
	}
}

func TestGetState(t *testing.T) {
	puller := New(10).(*simplePuller)

	s1a := puller.getState("/path/to/repo1")
	s1b := puller.getState("/path/to/repo1")
	s2 := puller.getState("/path/to/repo2")

	if s1a != s1b {
		t.Error("Expected the same state for the same path")
	}
	if s1a == s2 {
		t.Error("Expected different states for different paths")
	}
	if len(puller.dirs) != 2 {
		t.Errorf("Expected 2 states, got %d", len(puller.dirs))
	}
}

func TestGetStateConcurrency(t *testing.T) {
	puller := New(10).(*simplePuller)
	states := make([]*dirState, 10)

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			states[index] = puller.getState("/same/path")
		}(i)
	}
	wg.Wait()

	for i := 1; i < len(states); i++ {
		if states[i] != states[0] {
			t.Errorf("State %d differs from the first one", i)
		}
	}
	if len(puller.dirs) != 1 {
		t.Errorf("Expected 1 state in map, got %d", len(puller.dirs))
	}
}

// A burst of pushes while a pull runs gives one follow-up pull, never parallel pulls
func TestPullCoalesces(t *testing.T) {
	p := New(10).(*simplePuller)

	var mu sync.Mutex
	runs, active, maxActive := 0, 0, 0
	release := make(chan struct{})
	started := make(chan struct{}, 10)
	p.run = func(path string) {
		mu.Lock()
		runs++
		active++
		if active > maxActive {
			maxActive = active
		}
		mu.Unlock()
		started <- struct{}{}
		<-release
		mu.Lock()
		active--
		mu.Unlock()
	}

	p.Pull([]string{"/repo"})
	<-started
	for i := 0; i < 20; i++ {
		p.Pull([]string{"/repo"})
	}
	release <- struct{}{}
	<-started
	release <- struct{}{}

	deadline := time.Now().Add(2 * time.Second)
	for {
		s := p.getState("/repo")
		s.mu.Lock()
		done := !s.running
		s.mu.Unlock()
		if done || time.Now().After(deadline) {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}

	mu.Lock()
	defer mu.Unlock()
	if runs != 2 {
		t.Errorf("Expected 2 pulls (current + one follow-up), got %d", runs)
	}
	if maxActive != 1 {
		t.Errorf("Expected at most 1 pull at a time, got %d", maxActive)
	}
}

func TestNewZeroTimeoutUsesDefault(t *testing.T) {
	if p := New(0).(*simplePuller); p.gitTimeout != defaultGitTimeout {
		t.Errorf("Expected default timeout %d, got %d", defaultGitTimeout, p.gitTimeout)
	}
}

func TestPullNilPaths(t *testing.T) {
	puller := New(10)

	err := puller.Pull(nil)
	if err != nil {
		t.Errorf("Expected no error for nil paths, got %v", err)
	}
}

func TestPullEmptyPaths(t *testing.T) {
	puller := New(10)

	err := puller.Pull([]string{})
	if err != nil {
		t.Errorf("Expected no error for empty paths, got %v", err)
	}
}

func TestPullValidPath(t *testing.T) {
	tmpDir := t.TempDir()

	gitDir := filepath.Join(tmpDir, ".git")
	err := os.MkdirAll(gitDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create .git directory: %v", err)
	}

	puller := New(1)

	err = puller.Pull([]string{tmpDir})
	if err != nil {
		t.Errorf("Expected no error for valid pull, got %v", err)
	}

	time.Sleep(100 * time.Millisecond)
}

func TestPullInvalidPath(t *testing.T) {
	puller := New(1)

	err := puller.Pull([]string{"/non/existent/path"})
	if err != nil {
		t.Errorf("Expected no error (async operation), got %v", err)
	}

	time.Sleep(100 * time.Millisecond)
}

func TestPullMultiplePaths(t *testing.T) {
	tmpDir1 := t.TempDir()
	tmpDir2 := t.TempDir()

	for _, dir := range []string{tmpDir1, tmpDir2} {
		gitDir := filepath.Join(dir, ".git")
		err := os.MkdirAll(gitDir, 0755)
		if err != nil {
			t.Fatalf("Failed to create .git directory: %v", err)
		}
	}

	puller := New(1)

	err := puller.Pull([]string{tmpDir1, tmpDir2})
	if err != nil {
		t.Errorf("Expected no error for multiple paths, got %v", err)
	}

	time.Sleep(200 * time.Millisecond)
}

func TestPullTimeout(t *testing.T) {
	puller := New(1)

	err := puller.Pull([]string{"/non/existent/path"})
	if err != nil {
		t.Errorf("Expected no error (async operation), got %v", err)
	}

	time.Sleep(2 * time.Second)
}

func TestMutexedPullConcurrency(t *testing.T) {
	tmpDir := t.TempDir()

	gitDir := filepath.Join(tmpDir, ".git")
	err := os.MkdirAll(gitDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create .git directory: %v", err)
	}

	puller := New(1)

	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := puller.Pull([]string{tmpDir})
			if err != nil {
				t.Errorf("Expected no error, got %v", err)
			}
		}()
	}

	wg.Wait()
	time.Sleep(500 * time.Millisecond)
}

func TestDefaultGitTimeout(t *testing.T) {
	if defaultGitTimeout != 10 {
		t.Errorf("Expected defaultGitTimeout to be 10, got %d", defaultGitTimeout)
	}
}
