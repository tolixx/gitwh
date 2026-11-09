package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"time"

	"gitwh/config"
	"gitwh/handlers"
	"gitwh/puller/git"
)

func main() {

	configPath := flag.String("config", "/etc/gitwh.yaml", "Configuration file path")
	flag.Parse()

	cfg, err := config.FromFile(*configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	fmt.Printf("Webhook Server, config - %s\n", *configPath)
	fmt.Printf("%d repo(s), BufferSize: %d, Timeout: %d\n", len(cfg.Repos), cfg.BufferSize, cfg.Timeout)

	srv := &http.Server{
		Addr:              cfg.Listen,
		Handler:           newHandler(cfg),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	log.Fatalf("Failed to ListenAndServe: %v", srv.ListenAndServe())
}

func newHandler(cfg *config.Config) http.Handler {
	return handlers.New(cfg.Repos, cfg.BufferSize, git.New(cfg.Timeout))
}
