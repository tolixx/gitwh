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
	Secret  string   `json:"secret" yaml:"secret"`
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
		dec = json.NewDecoder(f)
	case ".yaml", ".yml":
		dec = yaml.NewDecoder(f)
	}

	if dec == nil {
		return nil, fmt.Errorf("unknown config file extension: %s", ext)
	}

	if err := dec.Decode(cfg); err != nil {
		return nil, fmt.Errorf("failed to decode config file: %v", err)
	}
	return cfg, nil
}
