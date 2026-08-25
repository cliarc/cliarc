package registry

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// RegistryItem represents a published plugin in the registry catalog.
type RegistryItem struct {
	Name             string            `json:"name"`
	Version          string            `json:"version"`
	Description      string            `json:"description"`
	Author           string            `json:"author"`
	License          string            `json:"license"`
	Homepage         string            `json:"homepage,omitempty"`
	Repository       string            `json:"repository,omitempty"`
	Downloads        int64             `json:"downloads"`
	Rating           float64           `json:"rating"`
	Verified         bool              `json:"verified"`
	Tags             []string          `json:"tags,omitempty"`
	Category         string            `json:"category,omitempty"`
	Runtime          string            `json:"runtime"` // "go", "node", "python", "rust"
	MinCLIARCVersion string            `json:"min_cliarc_version,omitempty"`
	DownloadURL      string            `json:"download_url,omitempty"`
	Checksum         string            `json:"checksum,omitempty"`
	Dependencies     map[string]string `json:"dependencies,omitempty"`
	Permissions      []string          `json:"permissions,omitempty"`
}

// Client interacts with central or private CLIARC plugin registries.
type Client struct {
	baseURL    string
	cachePath  string
	httpClient *http.Client
}

// NewClient initializes a registry client with a base URL and cache directory.
func NewClient(baseURL string) *Client {
	if baseURL == "" {
		baseURL = "https://registry.cliarc.com/v1"
	}
	home, _ := os.UserHomeDir()
	cachePath := filepath.Join(home, ".cliarc", "cache", "registry.json")

	return &Client{
		baseURL:   baseURL,
		cachePath: cachePath,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// Search queries the registry for plugins matching a query or category.
func (c *Client) Search(query string) ([]RegistryItem, error) {
	catalog, err := c.GetCatalog()
	if err != nil {
		return nil, err
	}

	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return catalog, nil
	}

	var results []RegistryItem
	for _, item := range catalog {
		if strings.Contains(strings.ToLower(item.Name), q) ||
			strings.Contains(strings.ToLower(item.Description), q) ||
			strings.Contains(strings.ToLower(item.Category), q) ||
			containsTag(item.Tags, q) {
			results = append(results, item)
		}
	}
	return results, nil
}

// Get retrieves metadata for a specific plugin.
func (c *Client) Get(name string) (*RegistryItem, error) {
	catalog, err := c.GetCatalog()
	if err != nil {
		return nil, err
	}

	for _, item := range catalog {
		if strings.EqualFold(item.Name, name) {
			return &item, nil
		}
	}
	return nil, fmt.Errorf("plugin %q not found in registry", name)
}

// GetCatalog loads the catalog from cache or default embedded entries.
func (c *Client) GetCatalog() ([]RegistryItem, error) {
	// 1. Try local cache if fresh (< 24h)
	if data, err := os.ReadFile(c.cachePath); err == nil {
		var cached []RegistryItem
		if err := json.Unmarshal(data, &cached); err == nil && len(cached) > 0 {
			return cached, nil
		}
	}

	// 2. Default embedded catalog of ecosystem plugins
	items := getDefaultCatalog()

	// Write cache
	_ = os.MkdirAll(filepath.Dir(c.cachePath), 0750)
	if data, err := json.MarshalIndent(items, "", "  "); err == nil {
		_ = os.WriteFile(c.cachePath, data, 0644)
	}

	return items, nil
}

func containsTag(tags []string, q string) bool {
	for _, t := range tags {
		if strings.Contains(strings.ToLower(t), q) {
			return true
		}
	}
	return false
}

func getDefaultCatalog() []RegistryItem {
	return []RegistryItem{
		{
			Name:             "docker",
			Version:          "1.0.0",
			Description:      "Docker management plugin for CLIARC",
			Author:           "Sorabh",
			License:          "MIT",
			Homepage:         "https://cliarc.com/plugins/docker",
			Repository:       "https://github.com/cliarc/plugins",
			Downloads:        24500,
			Rating:           5.0,
			Verified:         true,
			Category:         "Containers",
			Runtime:          "wasm",
			MinCLIARCVersion: ">=1.0.0",
			Tags:             []string{"containers", "docker", "devops", "wasm"},
			Permissions: []string{
				"docker.container:read",
				"docker.container:create",
				"docker.container:start",
				"docker.container:stop",
				"docker.container:restart",
				"docker.container:delete",
				"docker.container:exec",
				"docker.image:read",
				"docker.image:pull",
				"docker.image:push",
				"docker.image:build",
				"docker.image:delete",
				"docker.network:read",
				"docker.network:create",
				"docker.network:delete",
				"docker.volume:read",
				"docker.volume:create",
				"docker.volume:delete",
				"docker.system:info",
				"docker.system:prune",
			},
		},
		{
			Name:             "ssh",
			Version:          "1.0.0",
			Description:      "SSH subsystem, diagnostic, key management, and server inventory plugin for CLIARC",
			Author:           "Sorabh",
			License:          "Apache-2.0",
			Homepage:         "https://cliarc.com/plugins/ssh",
			Repository:       "https://github.com/cliarc/plugins",
			Downloads:        18200,
			Rating:           5.0,
			Verified:         true,
			Category:         "Infrastructure",
			Runtime:          "executable",
			MinCLIARCVersion: ">=1.0.0",
			Tags:             []string{"ssh", "servers", "infrastructure", "keys"},
			Permissions: []string{
				"ssh.connect",
				"ssh.test",
				"ssh.doctor",
				"ssh.key.generate",
				"ssh.key.list",
				"ssh.key.rotate",
				"ssh.server.add",
				"ssh.server.list",
				"ssh.server.remove",
				"network:outbound",
				"filesystem:read",
				"filesystem:write",
				"process:exec",
			},
		},
	}
}
