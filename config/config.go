package config

import (
	"encoding/json"
	"fmt"
	"gopkg.in/yaml.v3"
	"log"
	"os"
	"path/filepath"
)

const defaultBufferSize = 3
const defaultTimeout = 10

// Repo represents repository
type Repo struct {
	Secret string `json:"secret" yaml:"secret"`
	// Branch, when set, limits pulls to pushes on this branch
	Branch  string   `json:"branch" yaml:"branch"`
	Folders []string `json:"folders" yaml:"folders"`
}

// Config represents configuration for Webhook
type Config struct {
	Listen     string          `json:"listen" yaml:"listen"`
	Repos      map[string]Repo `json:"repos"  yaml:"repos"`
	BufferSize int             `json:"buffer_size" yaml:"buffer_size"`
	Timeout    int             `json:"timeout" yaml:"timeout"`
}

type Decoder interface {
	Decode(interface{}) error
}

// Default returns default config without any repos, listen on port 8080
func Default() *Config {
	return &Config{BufferSize: defaultBufferSize, Timeout: defaultTimeout, Listen: ":8080"}
}

// FromFile reads configurations from file
func FromFile(path string) (*Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open config file: %v", err)
	}

	defer func() {
		if err := f.Close(); err != nil {
			log.Printf("failed to close config file: %v", err)
		}
	}()

	cfg := Default()
	var dec Decoder
	ext := filepath.Ext(path)
	switch ext {
	case ".json", ".conf":
		d := json.NewDecoder(f)
		d.DisallowUnknownFields()
		dec = d
	case ".yaml", ".yml":
		d := yaml.NewDecoder(f)
		d.KnownFields(true)
		dec = d
	}

	if dec == nil {
		return nil, fmt.Errorf("unknown config file extension: %s", ext)
	}

	if err := dec.Decode(cfg); err != nil {
		return nil, fmt.Errorf("failed to decode config file: %v", err)
	}
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

func (c *Config) validate() error {
	if c.Timeout <= 0 {
		return fmt.Errorf("timeout must be a positive number of seconds, got %d", c.Timeout)
	}
	if c.BufferSize < 0 {
		return fmt.Errorf("buffer_size must not be negative, got %d", c.BufferSize)
	}
	for name, repo := range c.Repos {
		if len(repo.Folders) == 0 {
			return fmt.Errorf("repo %q has no folders", name)
		}
	}
	return nil
}
