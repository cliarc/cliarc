package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// Config holds CLIARC Core configuration.
type Config struct {
	mu sync.RWMutex

	ActiveServer    string                 `json:"active_server,omitempty"`
	PluginDir       string                 `json:"plugin_dir"`
	ServerStorePath string                 `json:"server_store_path"`
	SecretProvider  string                 `json:"secret_provider"`
	LogLevel        string                 `json:"log_level"`
	PluginTimeout   int                    `json:"plugin_timeout_seconds"`
	Extensions      map[string]interface{} `json:"extensions,omitempty"`
}

// Default returns a default configuration.
func Default() *Config {
	home, _ := os.UserHomeDir()
	return &Config{
		PluginDir:       filepath.Join(home, ".cliarc", "plugins"),
		ServerStorePath: filepath.Join(home, ".cliarc", "servers.json"),
		SecretProvider:  "keychain",
		LogLevel:        "info",
		PluginTimeout:   30,
		Extensions:      make(map[string]interface{}),
	}
}

// Load reads configuration from a JSON file.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Default(), nil
		}
		return nil, fmt.Errorf("config: read file: %w", err)
	}
	var c Config
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("config: unmarshal: %w", err)
	}
	if c.Extensions == nil {
		c.Extensions = make(map[string]interface{})
	}
	return &c, nil
}

// Save writes configuration to a JSON file.
func (c *Config) Save(path string) error {
	c.mu.RLock()
	defer c.mu.RUnlock()
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("config: marshal: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0750); err != nil {
		return fmt.Errorf("config: mkdir: %w", err)
	}
	return os.WriteFile(path, data, 0600)
}

// GetExtension retrieves an extension value by key.
func (c *Config) GetExtension(key string) (interface{}, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	v, ok := c.Extensions[key]
	return v, ok
}

// SetExtension stores an extension value.
func (c *Config) SetExtension(key string, value interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.Extensions[key] = value
}
