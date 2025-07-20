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
	
	if sp.mutexes == nil {
		t.Error("Expected mutexes map to be initialized")
	}
	
	if sp.lock == nil {
		t.Error("Expected lock to be initialized")
	}
}

func TestGetMutex(t *testing.T) {
	puller := New(10).(*simplePuller)
	
	path1 := "/path/to/repo1"
	path2 := "/path/to/repo2"
	
	mutex1a := puller.getMutex(path1)
	mutex1b := puller.getMutex(path1)
	mutex2 := puller.getMutex(path2)
	
	if mutex1a != mutex1b {
		t.Error("Expected same mutex for same path")
	}
	
	if mutex1a == mutex2 {
		t.Error("Expected different mutexes for different paths")
	}
	
	if len(puller.mutexes) != 2 {
		t.Errorf("Expected 2 mutexes, got %d", len(puller.mutexes))
	}
}

func TestGetMutexConcurrency(t *testing.T) {
	puller := New(10).(*simplePuller)
	path := "/test/path"
	
	var wg sync.WaitGroup
	mutexes := make([]*sync.Mutex, 10)
	
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			mutexes[index] = puller.getMutex(path)
		}(i)
	}
	
	wg.Wait()
	
	firstMutex := mutexes[0]
	for i := 1; i < 10; i++ {
		if mutexes[i] != firstMutex {
			t.Error("Expected all goroutines to get the same mutex for same path")
		}
	}
	
	if len(puller.mutexes) != 1 {
		t.Errorf("Expected 1 mutex in map, got %d", len(puller.mutexes))
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