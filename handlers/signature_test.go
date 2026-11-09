package handlers

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"gitwh/config"
)

type countPuller struct {
	mu sync.Mutex
	n  int
}

func (c *countPuller) Pull([]string) error {
	c.mu.Lock()
	c.n++
	c.mu.Unlock()
	return nil
}

func (c *countPuller) count() int {
	time.Sleep(50 * time.Millisecond) // pulls go through the event channel
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.n
}

func sign(secret, body string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(body))
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

const ghPush = `{"ref":"refs/heads/main","repository":{"name":"r"},"head_commit":{"id":"abc","message":"m"},"pusher":{"name":"p","email":"p@x"}}`

func send(h http.Handler, body, contentType string, headers map[string]string) int {
	req := httptest.NewRequest("POST", "/wh", strings.NewReader(body))
	req.Header.Set("Content-Type", contentType)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	return w.Code
}

func TestGithubSignature(t *testing.T) {
	form := url.Values{"payload": {ghPush}}.Encode()
	cases := []struct {
		name, body, ctype, sig string
		want, pulls            int
	}{
		{"json signed", ghPush, "application/json", sign("s3cret", ghPush), 200, 1},
		{"form signed", form, "application/x-www-form-urlencoded", sign("s3cret", form), 200, 1},
		{"wrong secret", ghPush, "application/json", sign("other", ghPush), 400, 0},
		{"no signature", ghPush, "application/json", "", 400, 0},
		{"garbage signature", ghPush, "application/json", "sha256=zz", 400, 0},
		{"signature of another body", ghPush, "application/json", sign("s3cret", ghPush+" "), 400, 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p := &countPuller{}
			h := New(map[string]config.Repo{"r": {Secret: "s3cret", Folders: []string{"/tmp"}}}, 1, p)
			hdr := map[string]string{"X-GitHub-Event": "push"}
			if c.sig != "" {
				hdr["X-Hub-Signature-256"] = c.sig
			}
			if got := send(h, c.body, c.ctype, hdr); got != c.want {
				t.Errorf("status %d, want %d", got, c.want)
			}
			if got := p.count(); got != c.pulls {
				t.Errorf("pulls %d, want %d", got, c.pulls)
			}
		})
	}
}

func TestGitlabTokenWithCharset(t *testing.T) {
	p := &countPuller{}
	h := New(map[string]config.Repo{"r": {Secret: "tok", Folders: []string{"/tmp"}}}, 1, p)
	body := `{"ref":"refs/heads/main","project":{"name":"r"},"commits":[{"id":"1","message":"m"}]}`
	hdr := map[string]string{"X-Gitlab-Event": "Push Hook", "X-Gitlab-Token": "tok"}
	if got := send(h, body, "application/json; charset=utf-8", hdr); got != 200 {
		t.Errorf("status %d, want 200", got)
	}
	hdr["X-Gitlab-Token"] = "nope"
	if got := send(h, body, "application/json", hdr); got != 400 {
		t.Errorf("wrong token: status %d, want 400", got)
	}
	if got := p.count(); got != 1 {
		t.Errorf("pulls %d, want 1", got)
	}
}

func TestIgnoredDeliveries(t *testing.T) {
	repos := map[string]config.Repo{
		"r": {Folders: []string{"/tmp"}, Branch: "main"},
	}
	cases := []struct {
		name, body string
		hdr        map[string]string
	}{
		{"github ping", `{"zen":"hi","repository":{"name":"r"}}`, map[string]string{"X-GitHub-Event": "ping"}},
		{"github issues", ghPush, map[string]string{"X-GitHub-Event": "issues"}},
		{"other branch", strings.Replace(ghPush, "refs/heads/main", "refs/heads/dev", 1), map[string]string{"X-GitHub-Event": "push"}},
		{"tag push", strings.Replace(ghPush, "refs/heads/main", "refs/tags/v1", 1), map[string]string{"X-GitHub-Event": "push"}},
		{"gitlab tag hook", `{"ref":"refs/tags/v1","project":{"name":"r"}}`, map[string]string{"X-Gitlab-Event": "Tag Push Hook"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p := &countPuller{}
			h := New(repos, 1, p)
			if got := send(h, c.body, "application/json", c.hdr); got != http.StatusAccepted {
				t.Errorf("status %d, want 202", got)
			}
			if got := p.count(); got != 0 {
				t.Errorf("pulls %d, want 0", got)
			}
		})
	}
}

func TestBranchMatch(t *testing.T) {
	p := &countPuller{}
	h := New(map[string]config.Repo{"r": {Folders: []string{"/tmp"}, Branch: "main"}}, 1, p)
	if got := send(h, ghPush, "application/json", map[string]string{"X-GitHub-Event": "push"}); got != 200 {
		t.Errorf("status %d, want 200", got)
	}
	if got := p.count(); got != 1 {
		t.Errorf("pulls %d, want 1", got)
	}
}

func TestBodyTooLarge(t *testing.T) {
	p := &countPuller{}
	h := New(map[string]config.Repo{"r": {Folders: []string{"/tmp"}}}, 1, p)
	big := `{"repository":{"name":"r"},"x":"` + strings.Repeat("a", maxBodySize) + `"}`
	if got := send(h, big, "application/json", map[string]string{"X-GitHub-Event": "push"}); got != 400 {
		t.Errorf("status %d, want 400", got)
	}
	if got := p.count(); got != 0 {
		t.Errorf("pulls %d, want 0", got)
	}
}

func TestGetNotAllowed(t *testing.T) {
	h := New(map[string]config.Repo{}, 1, &countPuller{})
	req := httptest.NewRequest("GET", "/wh", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("status %d, want 405", w.Code)
	}
}
