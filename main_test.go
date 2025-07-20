package main

import (
	"gitwh/config"
	"gitwh/puller/git"
	"net/http"
	"testing"
)

func TestNewHandler(t *testing.T) {
	cfg := &config.Config{
		Listen:     ":8080",
		BufferSize: 5,
		Timeout:    10,
		Repos: map[string]config.Repo{
			"test-repo": {
				Secret:  "secret",
				Folders: []string{"/path/to/repo"},
			},
		},
	}
	
	handler := newHandler(cfg)
	
	if handler == nil {
		t.Error("Expected handler to be created")
	}
	
	if _, ok := handler.(http.Handler); !ok {
		t.Error("Expected handler to implement http.Handler interface")
	}
}

func TestNewHandlerWithEmptyRepos(t *testing.T) {
	cfg := &config.Config{
		Listen:     ":8080",
		BufferSize: 3,
		Timeout:    15,
		Repos:      make(map[string]config.Repo),
	}
	
	handler := newHandler(cfg)
	
	if handler == nil {
		t.Error("Expected handler to be created even with empty repos")
	}
}

func TestNewHandlerIntegration(t *testing.T) {
	cfg := &config.Config{
		Listen:     ":9999",
		BufferSize: 1,
		Timeout:    5,
		Repos: map[string]config.Repo{
			"integration-repo": {
				Secret:  "integration-secret",
				Folders: []string{"/tmp/integration"},
			},
		},
	}
	
	handler := newHandler(cfg)
	
	gitPuller := git.New(cfg.Timeout)
	if gitPuller == nil {
		t.Error("Expected git puller to be created")
	}
	
	if handler == nil {
		t.Error("Expected handler to be created with git puller")
	}
}