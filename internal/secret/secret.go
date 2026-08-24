package secret

import (
	"fmt"
	"os"
	"strings"
	"sync"
)

// Provider is the interface for secret storage backends.
// Future implementations: macOS Keychain, Windows Credential Manager, Linux Secret Service.
type Provider interface {
	Get(name string) (string, error)
	Set(name string, value string) error
	Delete(name string) error
	List() ([]string, error)
}

// SecretRef represents a reference to a secret without containing its value.
type SecretRef struct {
	Name     string            `json:"name"`
	Provider string            `json:"provider"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

// Manager coordinates secret providers.
type Manager struct {
	providers map[string]Provider
	mu        sync.RWMutex
}

// NewManager creates a secret manager with default providers.
func NewManager() *Manager {
	m := &Manager{
		providers: make(map[string]Provider),
	}
	// Register default providers
	m.Register("env", &EnvProvider{})
	m.Register("file", &FileProvider{})
	m.Register("memory", NewMemoryProvider())
	return m
}

// Register adds a secret provider.
func (m *Manager) Register(name string, p Provider) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.providers[name] = p
}

// Resolve retrieves a secret value by reference.
func (m *Manager) Resolve(ref *SecretRef) (string, error) {
	if ref == nil {
		return "", fmt.Errorf("secret: nil reference")
	}
	m.mu.RLock()
	p, ok := m.providers[ref.Provider]
	m.mu.RUnlock()
	if !ok {
		return "", fmt.Errorf("secret: unknown provider %q", ref.Provider)
	}
	return p.Get(ref.Name)
}

// Mask returns a masked representation of a secret for logging/display.
func Mask(value string) string {
	if len(value) <= 4 {
		return "****"
	}
	return value[:2] + strings.Repeat("*", len(value)-4) + value[len(value)-2:]
}

// --- Providers ---

// EnvProvider reads secrets from environment variables.
type EnvProvider struct{}

func (e *EnvProvider) Get(name string) (string, error) {
	v := os.Getenv(name)
	if v == "" {
		return "", fmt.Errorf("secret: env var %q not set", name)
	}
	return v, nil
}

func (e *EnvProvider) Set(name string, value string) error {
	return os.Setenv(name, value)
}

func (e *EnvProvider) Delete(name string) error {
	return os.Unsetenv(name)
}

func (e *EnvProvider) List() ([]string, error) {
	return nil, fmt.Errorf("env provider does not support listing")
}

// FileProvider reads secrets from files (useful for Docker secrets).
type FileProvider struct{}

func (f *FileProvider) Get(name string) (string, error) {
	data, err := os.ReadFile(name)
	if err != nil {
		return "", fmt.Errorf("secret: read file %q: %w", name, err)
	}
	return strings.TrimSpace(string(data)), nil
}

func (f *FileProvider) Set(name string, value string) error {
	return os.WriteFile(name, []byte(value), 0600)
}

func (f *FileProvider) Delete(name string) error {
	return os.Remove(name)
}

func (f *FileProvider) List() ([]string, error) {
	return nil, fmt.Errorf("file provider does not support listing")
}

// MemoryProvider is an in-memory provider for testing.
type MemoryProvider struct {
	mu      sync.RWMutex
	secrets map[string]string
}

func NewMemoryProvider() *MemoryProvider {
	return &MemoryProvider{secrets: make(map[string]string)}
}

func (m *MemoryProvider) Get(name string) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	v, ok := m.secrets[name]
	if !ok {
		return "", fmt.Errorf("secret: %q not found", name)
	}
	return v, nil
}

func (m *MemoryProvider) Set(name string, value string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.secrets[name] = value
	return nil
}

func (m *MemoryProvider) Delete(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.secrets, name)
	return nil
}

func (m *MemoryProvider) List() ([]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	keys := make([]string, 0, len(m.secrets))
	for k := range m.secrets {
		keys = append(keys, k)
	}
	return keys, nil
}
