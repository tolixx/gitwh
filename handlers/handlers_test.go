package handlers

import (
	"bytes"
	"encoding/json"
	"gitwh/config"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

type mockPuller struct {
	pulledPaths [][]string
	shouldError bool
}

func (m *mockPuller) Pull(paths []string) error {
	m.pulledPaths = append(m.pulledPaths, paths)
	if m.shouldError {
		return &mockError{"pull error"}
	}
	return nil
}

type mockError struct {
	msg string
}

func (e *mockError) Error() string {
	return e.msg
}

func TestNew(t *testing.T) {
	repos := make(map[string]config.Repo)
	repos["test-repo"] = config.Repo{
		Secret:  "secret",
		Folders: []string{"/path/to/repo"},
	}
	
	puller := &mockPuller{}
	handler := New(repos, 5, puller)
	
	if handler == nil {
		t.Error("Expected handler to be created")
	}
}

func TestNotFound(t *testing.T) {
	repos := make(map[string]config.Repo)
	puller := &mockPuller{}
	handler := New(repos, 1, puller)
	
	req := httptest.NewRequest("GET", "/invalid", nil)
	w := httptest.NewRecorder()
	
	handler.ServeHTTP(w, req)
	
	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status %d, got %d", http.StatusNotFound, w.Code)
	}
}

func TestGithubPayload(t *testing.T) {
	repos := make(map[string]config.Repo)
	puller := &mockPuller{}
	h := &handler{
		repos:  repos,
		puller: puller,
	}
	
	payload := githubPayload{
		Pusher: struct {
			Name  string `json:"name"`
			Email string `json:"email"`
		}{
			Name:  "testuser",
			Email: "test@example.com",
		},
		Commit: struct {
			ID      string `json:"id"`
			Message string `json:"message"`
		}{
			ID:      "abc123",
			Message: "test commit",
		},
		Repository: struct {
			Name string `json:"name"`
		}{
			Name: "test-repo",
		},
	}
	
	payloadBytes, _ := json.Marshal(payload)
	form := url.Values{}
	form.Add("payload", string(payloadBytes))
	
	req := httptest.NewRequest("POST", "/wh", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	
	result, err := h.githubPayload(req)
	if err != nil {
		t.Fatalf("githubPayload failed: %v", err)
	}
	
	if result.Name != "testuser" {
		t.Errorf("Expected Name testuser, got %s", result.Name)
	}
	
	if result.Email != "test@example.com" {
		t.Errorf("Expected Email test@example.com, got %s", result.Email)
	}
	
	if result.CommitId != "abc123" {
		t.Errorf("Expected CommitId abc123, got %s", result.CommitId)
	}
	
	if result.Message != "test commit" {
		t.Errorf("Expected Message 'test commit', got %s", result.Message)
	}
	
	if result.Repo != "test-repo" {
		t.Errorf("Expected Repo test-repo, got %s", result.Repo)
	}
}

func TestGitlabPayload(t *testing.T) {
	repos := make(map[string]config.Repo)
	puller := &mockPuller{}
	h := &handler{
		repos:  repos,
		puller: puller,
	}
	
	payload := gitlabPayload{
		Repository: struct {
			Name string `json:"name"`
		}{
			Name: "gitlab-repo",
		},
		Commits: []struct {
			ID      string `json:"id"`
			Message string `json:"message"`
			Author  struct {
				Name  string `json:"name"`
				Email string `json:"email"`
			}
		}{
			{
				ID:      "def456",
				Message: "gitlab commit",
				Author: struct {
					Name  string `json:"name"`
					Email string `json:"email"`
				}{
					Name:  "gitlabuser",
					Email: "gitlab@example.com",
				},
			},
		},
	}
	
	payloadBytes, _ := json.Marshal(payload)
	req := httptest.NewRequest("POST", "/wh", bytes.NewReader(payloadBytes))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Gitlab-Token", "gitlab-secret")
	
	result, err := h.gitlabPayload(req)
	if err != nil {
		t.Fatalf("gitlabPayload failed: %v", err)
	}
	
	if result.Name != "gitlabuser" {
		t.Errorf("Expected Name gitlabuser, got %s", result.Name)
	}
	
	if result.Email != "gitlab@example.com" {
		t.Errorf("Expected Email gitlab@example.com, got %s", result.Email)
	}
	
	if result.CommitId != "def456" {
		t.Errorf("Expected CommitId def456, got %s", result.CommitId)
	}
	
	if result.Message != "gitlab commit" {
		t.Errorf("Expected Message 'gitlab commit', got %s", result.Message)
	}
	
	if result.Repo != "gitlab-repo" {
		t.Errorf("Expected Repo gitlab-repo, got %s", result.Repo)
	}
	
	if result.Secret != "gitlab-secret" {
		t.Errorf("Expected Secret gitlab-secret, got %s", result.Secret)
	}
}

func TestGitlabPayloadNoCommits(t *testing.T) {
	repos := make(map[string]config.Repo)
	puller := &mockPuller{}
	h := &handler{
		repos:  repos,
		puller: puller,
	}
	
	payload := gitlabPayload{
		Repository: struct {
			Name string `json:"name"`
		}{
			Name: "gitlab-repo",
		},
		Commits: []struct {
			ID      string `json:"id"`
			Message string `json:"message"`
			Author  struct {
				Name  string `json:"name"`
				Email string `json:"email"`
			}
		}{},
	}
	
	payloadBytes, _ := json.Marshal(payload)
	req := httptest.NewRequest("POST", "/wh", bytes.NewReader(payloadBytes))
	req.Header.Set("Content-Type", "application/json")
	
	_, err := h.gitlabPayload(req)
	if err == nil {
		t.Error("Expected error for payload with no commits")
	}
}

func TestGetPayload(t *testing.T) {
	repos := make(map[string]config.Repo)
	puller := &mockPuller{}
	h := &handler{
		repos:  repos,
		puller: puller,
	}
	
	req := httptest.NewRequest("POST", "/wh", bytes.NewReader([]byte{}))
	req.Header.Set("Content-Type", "application/json")
	
	_, err := h.getPayload(req)
	if err == nil {
		t.Error("Expected error for invalid JSON payload")
	}
}

func TestHandleValidRequest(t *testing.T) {
	repos := make(map[string]config.Repo)
	repos["test-repo"] = config.Repo{
		Secret:  "",
		Folders: []string{"/path/to/repo"},
	}
	
	puller := &mockPuller{}
	handler := New(repos, 1, puller)
	
	payload := githubPayload{
		Pusher: struct {
			Name  string `json:"name"`
			Email string `json:"email"`
		}{
			Name:  "testuser",
			Email: "test@example.com",
		},
		Commit: struct {
			ID      string `json:"id"`
			Message string `json:"message"`
		}{
			ID:      "abc123",
			Message: "test commit",
		},
		Repository: struct {
			Name string `json:"name"`
		}{
			Name: "test-repo",
		},
	}
	
	payloadBytes, _ := json.Marshal(payload)
	form := url.Values{}
	form.Add("payload", string(payloadBytes))
	
	req := httptest.NewRequest("POST", "/wh", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	
	handler.ServeHTTP(w, req)
	
	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}
}

func TestHandleUnsupportedRepo(t *testing.T) {
	repos := make(map[string]config.Repo)
	puller := &mockPuller{}
	handler := New(repos, 1, puller)
	
	payload := githubPayload{
		Repository: struct {
			Name string `json:"name"`
		}{
			Name: "unsupported-repo",
		},
	}
	
	payloadBytes, _ := json.Marshal(payload)
	form := url.Values{}
	form.Add("payload", string(payloadBytes))
	
	req := httptest.NewRequest("POST", "/wh", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	
	handler.ServeHTTP(w, req)
	
	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestHandleInvalidSecret(t *testing.T) {
	repos := make(map[string]config.Repo)
	repos["test-repo"] = config.Repo{
		Secret:  "correct-secret",
		Folders: []string{"/path/to/repo"},
	}
	
	puller := &mockPuller{}
	handler := New(repos, 1, puller)
	
	payload := gitlabPayload{
		Repository: struct {
			Name string `json:"name"`
		}{
			Name: "test-repo",
		},
		Commits: []struct {
			ID      string `json:"id"`
			Message string `json:"message"`
			Author  struct {
				Name  string `json:"name"`
				Email string `json:"email"`
			}
		}{
			{
				ID:      "def456",
				Message: "gitlab commit",
				Author: struct {
					Name  string `json:"name"`
					Email string `json:"email"`
				}{
					Name:  "gitlabuser",
					Email: "gitlab@example.com",
				},
			},
		},
	}
	
	payloadBytes, _ := json.Marshal(payload)
	req := httptest.NewRequest("POST", "/wh", bytes.NewReader(payloadBytes))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Gitlab-Token", "wrong-secret")
	w := httptest.NewRecorder()
	
	handler.ServeHTTP(w, req)
	
	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}