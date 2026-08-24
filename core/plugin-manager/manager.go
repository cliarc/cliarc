package manager

import (
	"context"
	"fmt"
	"time"
	"os"
	"path/filepath"
	"sync"

	pb "github.com/cliarc/cliarc/protocol/generated/go/cliarc/protocol"
	"github.com/cliarc/cliarc/core/events"
	"github.com/cliarc/cliarc/core/permissions"
	"github.com/cliarc/cliarc/core/registry"
	"github.com/cliarc/cliarc/core/plugin-runtime"
	"github.com/cliarc/cliarc/internal/manifest"
	"github.com/cliarc/cliarc/internal/models"
)

// Manager orchestrates plugin discovery, loading, and lifecycle.
type Manager struct {
	mu        sync.RWMutex
	registry  *registry.Registry
	validator *permissions.Validator
	bus       *events.Bus
	runtimes  map[string]*runtime.PluginRuntime // plugin name -> runtime
	workDir   string
}

// NewManager creates a plugin manager.
func NewManager(reg *registry.Registry, validator *permissions.Validator, bus *events.Bus, workDir string) *Manager {
	return &Manager{
		registry:  reg,
		validator: validator,
		bus:       bus,
		runtimes:  make(map[string]*runtime.PluginRuntime),
		workDir:   workDir,
	}
}

// Discover scans the plugin directory for manifests.
func (m *Manager) Discover(pluginDir string) ([]*manifest.Manifest, error) {
	if err := os.MkdirAll(pluginDir, 0755); err != nil {
		return nil, fmt.Errorf("manager: create plugin dir: %w", err)
	}

	entries, err := os.ReadDir(pluginDir)
	if err != nil {
		return nil, fmt.Errorf("manager: read plugin dir: %w", err)
	}

	var manifests []*manifest.Manifest
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		pluginSubDir := filepath.Join(pluginDir, entry.Name())
		manifestPath, ok := manifest.FindManifestInDir(pluginSubDir)
		if !ok {
			continue
		}
		mf, err := manifest.Load(manifestPath)
		if err != nil {
			continue // Skip invalid manifests
		}
		manifests = append(manifests, mf)
	}

	// If no plugins found in configured pluginDir, check local fallback directories (e.g. ../plugins, ./plugins)
	if len(manifests) == 0 {
		for _, fallbackDir := range []string{"../plugins", "plugins", "./plugins"} {
			if fbEntries, fbErr := os.ReadDir(fallbackDir); fbErr == nil {
				for _, entry := range fbEntries {
					if !entry.IsDir() {
						continue
					}
					pluginSubDir := filepath.Join(fallbackDir, entry.Name())
					if manifestPath, ok := manifest.FindManifestInDir(pluginSubDir); ok {
						if mf, err := manifest.Load(manifestPath); err == nil {
							manifests = append(manifests, mf)
						}
					}
				}
				if len(manifests) > 0 {
					break
				}
			}
		}
	}

	return manifests, nil
}

// Load registers a plugin manifest and prepares its runtime.
func (m *Manager) Load(mf *manifest.Manifest) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.runtimes[mf.Name]; exists {
		return fmt.Errorf("manager: plugin %q already loaded", mf.Name)
	}

	info := &models.PluginInfo{
		Name:            mf.Name,
		Version:         mf.Version,
		ProtocolVersion: mf.ProtocolVersion,
		Description:     mf.Description,
		Author:          mf.Author,
		Permissions:     mf.Permissions,
		Actions:         mf.Actions,
		State:           models.PluginStateDiscovered,
		ManifestPath:    mf.Name, // simplified
		RuntimeType:     mf.Runtime.Type,
		Command:         mf.Runtime.Command,
	}

	if err := m.registry.Register(info); err != nil {
		return fmt.Errorf("manager: register plugin: %w", err)
	}

	m.validator.Register(mf.Name, mf.Permissions)

	rt := runtime.NewPluginRuntime(mf)
	m.runtimes[mf.Name] = rt

	m.bus.Publish(context.Background(), events.Event{
		Type:   events.EventPluginDiscovered,
		Source: "plugin-manager",
		Payload: map[string]interface{}{
			"plugin":  mf.Name,
			"version": mf.Version,
		},
	})

	return nil
}

// Start launches a registered plugin.
func (m *Manager) Start(ctx context.Context, name string) error {
	m.mu.Lock()
	rt, ok := m.runtimes[name]
	m.mu.Unlock()
	if !ok {
		return fmt.Errorf("manager: plugin %q not loaded", name)
	}

	pluginWorkDir := m.workDir
	if rt.Manifest() != nil && rt.Manifest().Dir != "" {
		pluginWorkDir = rt.Manifest().Dir
	} else if m.workDir != "" {
		candidate := filepath.Join(m.workDir, name)
		if stat, err := os.Stat(candidate); err == nil && stat.IsDir() {
			pluginWorkDir = candidate
		}
	}

	if err := rt.Start(ctx, pluginWorkDir); err != nil {
		return fmt.Errorf("manager: start plugin %q: %w", name, err)
	}

	m.registry.UpdateState(name, models.PluginStateRunning)
	m.bus.Publish(ctx, events.Event{
		Type:   events.EventPluginStarted,
		Source: "plugin-manager",
		Payload: map[string]interface{}{
			"plugin": name,
			"pid":    rt.Info().PID,
		},
	})

	return nil
}

// Stop gracefully stops a plugin.
func (m *Manager) Stop(ctx context.Context, name string) error {
	m.mu.Lock()
	rt, ok := m.runtimes[name]
	m.mu.Unlock()
	if !ok {
		return fmt.Errorf("manager: plugin %q not loaded", name)
	}

	if err := rt.Stop(ctx); err != nil {
		return fmt.Errorf("manager: stop plugin %q: %w", name, err)
	}

	m.registry.UpdateState(name, models.PluginStateStopped)
	m.validator.Unregister(name)
	m.bus.Publish(ctx, events.Event{
		Type:   events.EventPluginStopped,
		Source: "plugin-manager",
		Payload: map[string]interface{}{
			"plugin": name,
		},
	})

	return nil
}

// Execute sends an action to a running plugin.
func (m *Manager) Execute(ctx context.Context, pluginName, action string, payload []byte) (*pb.ExecuteResponse, error) {
	if err := m.validator.ValidateAction(pluginName, action); err != nil {
		m.bus.Publish(ctx, events.Event{
			Type:   events.EventPermissionDenied,
			Source: "plugin-manager",
			Payload: map[string]interface{}{
				"plugin": pluginName,
				"action": action,
			},
		})
		return nil, err
	}

	m.mu.RLock()
	rt, ok := m.runtimes[pluginName]
	m.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("manager: plugin %q not loaded", pluginName)
	}

	client := rt.Client()
	if client == nil {
		return nil, fmt.Errorf("manager: plugin %q not connected", pluginName)
	}

	return client.Execute(ctx, &pb.ExecuteRequest{
		ExecutionId: fmt.Sprintf("%s-%d", pluginName, time.Now().UnixNano()),
		Action:      action,
		Payload:     payload,
	})
}

// GetRuntime returns the runtime for a plugin.
func (m *Manager) GetRuntime(name string) (*runtime.PluginRuntime, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	rt, ok := m.runtimes[name]
	return rt, ok
}

// Health checks a plugin's health.
func (m *Manager) Health(ctx context.Context, name string) (*pb.HealthResponse, error) {
	m.mu.RLock()
	rt, ok := m.runtimes[name]
	m.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("manager: plugin %q not loaded", name)
	}
	client := rt.Client()
	if client == nil {
		return nil, fmt.Errorf("manager: plugin %q not connected", name)
	}
	return client.Health(ctx, &pb.HealthRequest{})
}

// List returns all loaded plugins.
func (m *Manager) List() []*models.PluginInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]*models.PluginInfo, 0, len(m.runtimes))
	for _, rt := range m.runtimes {
		if info := rt.Info(); info != nil {
			out = append(out, info)
		}
	}
	return out
}

// Unload removes a plugin from the manager.
func (m *Manager) Unload(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.runtimes, name)
	m.registry.Unregister(name)
	m.validator.Unregister(name)
	return nil
}
