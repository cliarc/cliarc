package manager

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// PluginRecord stores persistent state metadata for an installed plugin.
type PluginRecord struct {
	Name          string    `json:"name"`
	Version       string    `json:"version"`
	Description   string    `json:"description,omitempty"`
	Author        string    `json:"author,omitempty"`
	Enabled       bool      `json:"enabled"`
	InstallSource string    `json:"install_source"` // "registry", "local", "git", "linked"
	InstalledAt   time.Time `json:"installed_at"`
	UpdatedAt     time.Time `json:"updated_at"`
	Checksum      string    `json:"checksum,omitempty"`
	Path          string    `json:"path"`
	ConfigPath    string    `json:"config_path,omitempty"`
}

// PluginStateStore manages ~/.cliarc/plugins.json.
type PluginStateStore struct {
	mu       sync.RWMutex
	filePath string
	Plugins  map[string]*PluginRecord `json:"plugins"`
}

// NewPluginStateStore creates a state store targeting the given state path or default ~/.cliarc/plugins.json.
func NewPluginStateStore(customPath string) (*PluginStateStore, error) {
	target := customPath
	if target == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("state: resolve home dir: %w", err)
		}
		target = filepath.Join(home, ".cliarc", "plugins.json")
	}

	store := &PluginStateStore{
		filePath: target,
		Plugins:  make(map[string]*PluginRecord),
	}

	if err := store.Load(); err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	return store, nil
}

// Load reads plugins.json from disk.
func (s *PluginStateStore) Load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := os.ReadFile(s.filePath)
	if err != nil {
		return err
	}

	var raw struct {
		Plugins map[string]*PluginRecord `json:"plugins"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("state: parse json: %w", err)
	}

	if raw.Plugins != nil {
		s.Plugins = raw.Plugins
	} else {
		s.Plugins = make(map[string]*PluginRecord)
	}

	return nil
}

// Save writes plugins.json to disk.
func (s *PluginStateStore) Save() error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if err := os.MkdirAll(filepath.Dir(s.filePath), 0750); err != nil {
		return fmt.Errorf("state: mkdir: %w", err)
	}

	raw := struct {
		Plugins map[string]*PluginRecord `json:"plugins"`
	}{
		Plugins: s.Plugins,
	}

	data, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return fmt.Errorf("state: marshal: %w", err)
	}

	return os.WriteFile(s.filePath, data, 0600)
}

// Get returns the plugin record by name.
func (s *PluginStateStore) Get(name string) (*PluginRecord, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	rec, ok := s.Plugins[name]
	return rec, ok
}

// List returns all recorded plugins.
func (s *PluginStateStore) List() []*PluginRecord {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*PluginRecord, 0, len(s.Plugins))
	for _, rec := range s.Plugins {
		out = append(out, rec)
	}
	return out
}

// Set adds or updates a plugin record and saves.
func (s *PluginStateStore) Set(rec *PluginRecord) error {
	s.mu.Lock()
	s.Plugins[rec.Name] = rec
	s.mu.Unlock()
	return s.Save()
}

// Remove deletes a plugin record and saves.
func (s *PluginStateStore) Remove(name string) error {
	s.mu.Lock()
	delete(s.Plugins, name)
	s.mu.Unlock()
	return s.Save()
}

// SetEnabled toggles the enabled state of a plugin.
func (s *PluginStateStore) SetEnabled(name string, enabled bool) error {
	s.mu.Lock()
	rec, ok := s.Plugins[name]
	if !ok {
		s.mu.Unlock()
		return fmt.Errorf("plugin %q not found in state store", name)
	}
	rec.Enabled = enabled
	rec.UpdatedAt = time.Now()
	s.mu.Unlock()
	return s.Save()
}
