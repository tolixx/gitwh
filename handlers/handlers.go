package handlers

import (
	"encoding/json"
	"fmt"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"gitwh/config"
	"io"
	"log"
	"net/http"

	"gitwh/puller"
)

type repoMap map[string]config.Repo

type handler struct {
	event  chan []string
	repos  repoMap
	puller puller.Puller
}

type Payload struct {
	Name     string
	Email    string
	CommitId string
	Message  string
	Repo     string
	Secret   string
}

type githubPayload struct {
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
	fmt.Printf("%s %s - Not Found\n", r.RemoteAddr, r.RequestURI)
	http.Error(w, "Not Found", http.StatusNotFound)
}

func (h *handler) githubPayload(r *http.Request) (*Payload, error) {
	if err := r.ParseForm(); err != nil {
		return nil, err
	}

	payload := []byte(r.FormValue("payload"))
	pl := &githubPayload{}
	if err := json.Unmarshal(payload, pl); err != nil {
		return nil, err
	}

	p := Payload{
		Name:     pl.Pusher.Name,
		Email:    pl.Pusher.Email,
		CommitId: pl.Commit.ID,
		Message:  pl.Commit.Message,
		Repo:     pl.Repository.Name,
	}

	return &p, nil
}

func (h *handler) gitlabPayload(r *http.Request) (*Payload, error) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}

	pl := &gitlabPayload{}
	if err := json.Unmarshal(body, pl); err != nil {
		return nil, err
	}

	if len(pl.Commits) == 0 {
		return nil, fmt.Errorf("invalid commit %v", pl)
	}

	if len(pl.Commits) > 1 {
		log.Printf("Multi Commit in one hook (len: %d)", len(pl.Commits))
	}

	commit := pl.Commits[0]

	p := Payload{
		Name:     commit.Author.Name,
		Email:    commit.Author.Email,
		CommitId: commit.ID,
		Message:  commit.Message,
		Repo:     pl.Repository.Name,
		Secret:   r.Header.Get("X-Gitlab-Token"),
	}

	return &p, nil
}

func (h *handler) getPayload(r *http.Request) (*Payload, error) {
	contentType := r.Header.Get("Content-Type")
	log.Printf("Content-Type: %s", contentType)

	if contentType == "application/json" {
		return h.gitlabPayload(r)
	}

	return h.githubPayload(r)
}

func (h *handler) getRepositoryPath(r *http.Request) ([]string, error) {
	pl, err := h.getPayload(r)
	if err != nil {
		return nil, err
	}

	repo, ok := h.repos[pl.Repo]
	if !ok {
		return nil, fmt.Errorf("repository %s not supported", pl.Repo)
	}

	if repo.Secret != "" && pl.Secret != repo.Secret {
		return nil, fmt.Errorf("secret is not valid. Given value: %s", pl.Secret)
	}

	fmt.Printf("%s %s Push made by %v (%v)\n", r.RemoteAddr, pl.Repo, pl.Name, pl.Email)
	if pl.Message != "" {
		fmt.Printf("%s Commit message : %s\n", pl.CommitId, pl.Message)
	}
	return repo.Folders, nil
}

func (h *handler) handle(w http.ResponseWriter, r *http.Request) {
	fmt.Printf("Request from %s\n", r.RemoteAddr)
	repoPath, err := h.getRepositoryPath(r)
	if err != nil {
		fmt.Printf("[%s] Bad request : %v\n", r.RemoteAddr, err)
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}
	h.event <- repoPath
}

func (h *handler) pull() {
	for path := range h.event {
		if err := h.puller.Pull(path); err != nil {
			fmt.Printf("Pull error: %v", err)
		}
	}
}
