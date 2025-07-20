package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"

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

	http.Handle("/", newHandler(cfg))

	if err := http.ListenAndServe(cfg.Listen, nil); err != nil {
		fmt.Printf("Failed to ListenAndServe : %v", err)
	}
}

func newHandler(cfg *config.Config) http.Handler {
	return handlers.New(cfg.Repos, cfg.BufferSize, git.New(cfg.Timeout))
}
