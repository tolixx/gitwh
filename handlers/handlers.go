package handlers

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"net/url"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"gitwh/config"
	"gitwh/puller"
)

// maxBodySize caps a webhook body. GitHub sends at most 25 MB, a push payload is far smaller.
const maxBodySize = 25 << 20

type repoMap map[string]config.Repo

type handler struct {
	event  chan []string
	repos  repoMap
	puller puller.Puller
}

// Payload is what gitwh needs from a GitHub or GitLab push delivery
type Payload struct {
	Name     string
	Email    string
	CommitId string
	Message  string
	Repo     string
	Ref      string
	// Event is the delivery type ("push", "ping", "Push Hook", ...), empty when the header is missing
	Event string
	// Secret is GitLab's X-Gitlab-Token
	Secret string
	// Signature is GitHub's X-Hub-Signature-256, checked against Body
	Signature string
	Body      []byte
	github    bool
}

type githubPayload struct {
	Ref    string `json:"ref"`
	Pusher struct {
		Name  string `json:"name"`
		Email string `json:"email"`
	} `json:"pusher"`
	Commit struct {
		ID      string `json:"id"`
		Message string `json:"message"`
	} `json:"head_commit"`
	Repository struct {
		Name string `json:"name"`
	} `json:"repository"`
	URL string `json:"git_url"`
}

type gitlabPayload struct {
	Ref        string `json:"ref"`
	Repository struct {
		Name string `json:"name"`
	} `json:"project"`
	Commits []struct {
		ID      string `json:"id"`
		Message string `json:"message"`
		Author  struct {
			Name  string `json:"name"`
			Email string `json:"email"`
		}
	} `json:"commits"`
}

// errIgnored marks a valid delivery that must not trigger a pull (ping, tag push, other branch)
var errIgnored = errors.New("ignored")

// New creates new handlers for Webhook Server
func New(repositories repoMap, bufferSize int, puller puller.Puller) http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.Logger)
	r.Use(middleware.RealIP)

	h := &handler{
		event:  make(chan []string, bufferSize),
		repos:  repositories,
		puller: puller,
	}

	r.HandleFunc("/", h.notFound)
	r.HandleFunc("/wh", h.handle)

	go h.pull()
	return r
}

func (h *handler) notFound(w http.ResponseWriter, r *http.Request) {
	log.Printf("%s %s - Not Found", r.RemoteAddr, r.RequestURI)
	http.Error(w, "Not Found", http.StatusNotFound)
}

func (h *handler) githubPayload(r *http.Request, body []byte) (*Payload, error) {
	raw := body
	if mediaType(r) == "application/x-www-form-urlencoded" {
		form, err := url.ParseQuery(string(body))
		if err != nil {
			return nil, err
		}
		raw = []byte(form.Get("payload"))
	}

	pl := &githubPayload{}
	if err := json.Unmarshal(raw, pl); err != nil {
		return nil, err
	}

	return &Payload{
		Name:      pl.Pusher.Name,
		Email:     pl.Pusher.Email,
		CommitId:  pl.Commit.ID,
		Message:   pl.Commit.Message,
		Repo:      pl.Repository.Name,
		Ref:       pl.Ref,
		Event:     r.Header.Get("X-GitHub-Event"),
		Signature: r.Header.Get("X-Hub-Signature-256"),
		Body:      body,
		github:    true,
	}, nil
}

func (h *handler) gitlabPayload(r *http.Request, body []byte) (*Payload, error) {
	pl := &gitlabPayload{}
	if err := json.Unmarshal(body, pl); err != nil {
		return nil, err
	}

	p := &Payload{
		Repo:   pl.Repository.Name,
		Ref:    pl.Ref,
		Event:  r.Header.Get("X-Gitlab-Event"),
		Secret: r.Header.Get("X-Gitlab-Token"),
	}

	// A push that deletes a branch has no commits, it is still a valid delivery
	if len(pl.Commits) > 0 {
		if len(pl.Commits) > 1 {
			log.Printf("Multi Commit in one hook (len: %d)", len(pl.Commits))
		}
		commit := pl.Commits[0]
		p.Name = commit.Author.Name
		p.Email = commit.Author.Email
		p.CommitId = commit.ID
		p.Message = commit.Message
	}

	return p, nil
}

func mediaType(r *http.Request) string {
	mt, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil {
		return ""
	}
	return mt
}

func (h *handler) getPayload(r *http.Request) (*Payload, error) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read body: %v", err)
	}

	switch {
	case r.Header.Get("X-GitHub-Event") != "":
		return h.githubPayload(r, body)
	case r.Header.Get("X-Gitlab-Event") != "":
		return h.gitlabPayload(r, body)
	case mediaType(r) == "application/json":
		// No event header: keep the old rule, JSON means GitLab
		return h.gitlabPayload(r, body)
	default:
		return h.githubPayload(r, body)
	}
}

// validSignature checks GitHub's X-Hub-Signature-256: HMAC-SHA256 of the raw body keyed by the secret
func validSignature(secret string, body []byte, header string) bool {
	given, ok := strings.CutPrefix(header, "sha256=")
	if !ok {
		return false
	}
	sig, err := hex.DecodeString(given)
	if err != nil {
		return false
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return hmac.Equal(sig, mac.Sum(nil))
}

func validToken(secret, given string) bool {
	return subtle.ConstantTimeCompare([]byte(secret), []byte(given)) == 1
}

func (h *handler) getRepositoryPath(r *http.Request) ([]string, error) {
	pl, err := h.getPayload(r)
	if err != nil {
		return nil, err
	}

	repo, ok := h.repos[pl.Repo]
	if !ok {
		return nil, fmt.Errorf("repository %q not supported", pl.Repo)
	}

	if repo.Secret != "" {
		if pl.github {
			if !validSignature(repo.Secret, pl.Body, pl.Signature) {
				return nil, fmt.Errorf("%s: missing or invalid X-Hub-Signature-256", pl.Repo)
			}
		} else if !validToken(repo.Secret, pl.Secret) {
			return nil, fmt.Errorf("%s: missing or invalid X-Gitlab-Token", pl.Repo)
		}
	}

	switch pl.Event {
	case "", "push", "Push Hook":
	default:
		log.Printf("%s: %s event, nothing to pull", pl.Repo, pl.Event)
		return nil, errIgnored
	}

	if strings.HasPrefix(pl.Ref, "refs/tags/") {
		log.Printf("%s: tag push %s, nothing to pull", pl.Repo, pl.Ref)
		return nil, errIgnored
	}

	if repo.Branch != "" && pl.Ref != "refs/heads/"+repo.Branch {
		log.Printf("%s: push to %s, watching %s only", pl.Repo, pl.Ref, repo.Branch)
		return nil, errIgnored
	}

	log.Printf("%s %s Push made by %v (%v)", r.RemoteAddr, pl.Repo, pl.Name, pl.Email)
	if pl.Message != "" {
		log.Printf("%s Commit message : %q", pl.CommitId, pl.Message)
	}
	return repo.Folders, nil
}

func (h *handler) handle(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxBodySize)
	repoPath, err := h.getRepositoryPath(r)
	if errors.Is(err, errIgnored) {
		w.WriteHeader(http.StatusAccepted)
		return
	}
	if err != nil {
		log.Printf("[%s] Bad request : %v", r.RemoteAddr, err)
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}
	h.event <- repoPath
	w.WriteHeader(http.StatusOK)
}

func (h *handler) pull() {
	for path := range h.event {
		if err := h.puller.Pull(path); err != nil {
			log.Printf("Pull error: %v", err)
		}
	}
}
