package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefault(t *testing.T) {
	cfg := Default()
	
	if cfg.BufferSize != defaultBufferSize {
		t.Errorf("Expected BufferSize %d, got %d", defaultBufferSize, cfg.BufferSize)
	}
	
	if cfg.Timeout != defaultTimeout {
		t.Errorf("Expected Timeout %d, got %d", defaultTimeout, cfg.Timeout)
	}
	
	if cfg.Listen != ":8080" {
		t.Errorf("Expected Listen :8080, got %s", cfg.Listen)
	}
	
	if cfg.Repos != nil {
		t.Error("Expected Repos to be nil for default config")
	}
}

func TestFromFileJSON(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "test.json")
	
	jsonContent := `{
		"listen": ":9090",
		"buffer_size": 5,
		"timeout": 15,
		"repos": {
			"test-repo": {
				"secret": "test-secret",
				"folders": ["/path/to/repo"]
			}
		}
	}`
	
	err := os.WriteFile(configFile, []byte(jsonContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test config file: %v", err)
	}
	
	cfg, err := FromFile(configFile)
	if err != nil {
		t.Fatalf("FromFile failed: %v", err)
	}
	
	if cfg.Listen != ":9090" {
		t.Errorf("Expected Listen :9090, got %s", cfg.Listen)
	}
	
	if cfg.BufferSize != 5 {
		t.Errorf("Expected BufferSize 5, got %d", cfg.BufferSize)
	}
	
	if cfg.Timeout != 15 {
		t.Errorf("Expected Timeout 15, got %d", cfg.Timeout)
	}
	
	repo, exists := cfg.Repos["test-repo"]
	if !exists {
		t.Error("Expected test-repo to exist in config")
	}
	
	if repo.Secret != "test-secret" {
		t.Errorf("Expected Secret test-secret, got %s", repo.Secret)
	}
	
	if len(repo.Folders) != 1 || repo.Folders[0] != "/path/to/repo" {
		t.Errorf("Expected Folders [/path/to/repo], got %v", repo.Folders)
	}
}

func TestFromFileYAML(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "test.yaml")
	
	yamlContent := `listen: ":8081"
buffer_size: 10
timeout: 20
repos:
  my-repo:
    secret: "yaml-secret"
    folders:
      - "/repo1"
      - "/repo2"
`
	
	err := os.WriteFile(configFile, []byte(yamlContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test config file: %v", err)
	}
	
	cfg, err := FromFile(configFile)
	if err != nil {
		t.Fatalf("FromFile failed: %v", err)
	}
	
	if cfg.Listen != ":8081" {
		t.Errorf("Expected Listen :8081, got %s", cfg.Listen)
	}
	
	if cfg.BufferSize != 10 {
		t.Errorf("Expected BufferSize 10, got %d", cfg.BufferSize)
	}
	
	if cfg.Timeout != 20 {
		t.Errorf("Expected Timeout 20, got %d", cfg.Timeout)
	}
	
	repo, exists := cfg.Repos["my-repo"]
	if !exists {
		t.Error("Expected my-repo to exist in config")
	}
	
	if repo.Secret != "yaml-secret" {
		t.Errorf("Expected Secret yaml-secret, got %s", repo.Secret)
	}
	
	expectedFolders := []string{"/repo1", "/repo2"}
	if len(repo.Folders) != 2 || repo.Folders[0] != expectedFolders[0] || repo.Folders[1] != expectedFolders[1] {
		t.Errorf("Expected Folders %v, got %v", expectedFolders, repo.Folders)
	}
}

func TestFromFileNonExistent(t *testing.T) {
	_, err := FromFile("/non/existent/file.json")
	if err == nil {
		t.Error("Expected error for non-existent file")
	}
}

func TestFromFileUnknownExtension(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "test.txt")
	
	err := os.WriteFile(configFile, []byte("content"), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	
	_, err = FromFile(configFile)
	if err == nil {
		t.Error("Expected error for unknown file extension")
	}
}

func TestFromFileInvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "invalid.json")
	
	err := os.WriteFile(configFile, []byte("invalid json"), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	
	_, err = FromFile(configFile)
	if err == nil {
		t.Error("Expected error for invalid JSON")
	}
}

func TestFromFileInvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "invalid.yaml")
	
	err := os.WriteFile(configFile, []byte("invalid: yaml: content: ["), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	
	_, err = FromFile(configFile)
	if err == nil {
		t.Error("Expected error for invalid YAML")
	}
}