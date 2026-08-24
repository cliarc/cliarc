package registry

import (
	"fmt"
	"sync"

	"github.com/cliarc/cliarc/internal/manifest"
	"github.com/cliarc/cliarc/internal/models"
)

// Registry maintains the catalog of discovered and loaded plugins.
type Registry struct {
	mu      sync.RWMutex
	plugins map[string]*models.PluginInfo // key: plugin name
}

// New creates a new plugin registry.
func New() *Registry {
	return &Registry{
		plugins: make(map[string]*models.PluginInfo),
	}
}

// Register adds a plugin to the registry.
func (r *Registry) Register(info *models.PluginInfo) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.plugins[info.Name]; exists {
		return fmt.Errorf("registry: plugin %q already registered", info.Name)
	}
	r.plugins[info.Name] = info
	return nil
}

// Unregister removes a plugin from the registry.
func (r *Registry) Unregister(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.plugins, name)
}

// Get retrieves a plugin by name.
func (r *Registry) Get(name string) (*models.PluginInfo, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.plugins[name]
	return p, ok
}

// List returns all registered plugins.
func (r *Registry) List() []*models.PluginInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]*models.PluginInfo, 0, len(r.plugins))
	for _, p := range r.plugins {
		out = append(out, p)
	}
	return out
}

// UpdateState updates the state of a registered plugin.
func (r *Registry) UpdateState(name string, state models.PluginState) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	p, ok := r.plugins[name]
	if !ok {
		return fmt.Errorf("registry: plugin %q not found", name)
	}
	p.State = state
	return nil
}

// Discover scans a directory for plugin manifests and returns them.
func Discover(dir string) ([]*manifest.Manifest, error) {
	// Placeholder: in production, walk the directory looking for manifest.yaml files.
	return nil, fmt.Errorf("registry.Discover not yet implemented")
}
