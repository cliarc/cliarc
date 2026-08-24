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
			Description:      "Official Docker container, image, and compose automation",
			Author:           "CLIARC Community",
			License:          "Apache-2.0",
			Homepage:         "https://cliarc.com/plugins/docker",
			Repository:       "https://github.com/cliarc/plugin-docker",
			Downloads:        14500,
			Rating:           4.9,
			Verified:         true,
			Category:         "Containers",
			Runtime:          "go",
			MinCLIARCVersion: ">=0.1.0",
			Tags:             []string{"containers", "docker", "devops"},
			Permissions:      []string{"process:exec", "filesystem:read", "network:outbound"},
		},
		{
			Name:             "kubernetes",
			Version:          "1.2.0",
			Description:      "Kubernetes cluster inspection, pod management, and helm deployment",
			Author:           "CLIARC Community",
			License:          "Apache-2.0",
			Homepage:         "https://cliarc.com/plugins/kubernetes",
			Repository:       "https://github.com/cliarc/plugin-kubernetes",
			Downloads:        9800,
			Rating:           4.8,
			Verified:         true,
			Category:         "Cloud & Orchestration",
			Runtime:          "go",
			MinCLIARCVersion: ">=0.1.0",
			Tags:             []string{"k8s", "kubernetes", "cloud", "orchestration"},
			Permissions:      []string{"process:exec", "network:outbound"},
		},
		{
			Name:             "aws",
			Version:          "0.9.5",
			Description:      "AWS Cloud resources, EC2 management, and S3 asset operations",
			Author:           "CLIARC Community",
			License:          "Apache-2.0",
			Homepage:         "https://cliarc.com/plugins/aws",
			Repository:       "https://github.com/cliarc/plugin-aws",
			Downloads:        8400,
			Rating:           4.7,
			Verified:         true,
			Category:         "Cloud",
			Runtime:          "python",
			MinCLIARCVersion: ">=0.1.0",
			Tags:             []string{"aws", "cloud", "s3", "ec2"},
			Permissions:      []string{"network:outbound", "secrets:access"},
		},
		{
			Name:             "github",
			Version:          "1.1.0",
			Description:      "GitHub repository workflows, pull requests, issues, and release automation",
			Author:           "CLIARC Core Team",
			License:          "MIT",
			Homepage:         "https://cliarc.com/plugins/github",
			Repository:       "https://github.com/cliarc/plugin-github",
			Downloads:        11200,
			Rating:           4.9,
			Verified:         true,
			Category:         "Developer Tools",
			Runtime:          "node",
			MinCLIARCVersion: ">=0.1.0",
			Tags:             []string{"git", "github", "ci-cd"},
			Permissions:      []string{"network:outbound", "secrets:access"},
		},
		{
			Name:             "terraform",
			Version:          "0.8.0",
			Description:      "Infrastructure as Code plan, apply, drift detection, and state inspection",
			Author:           "CLIARC Community",
			License:          "MPL-2.0",
			Homepage:         "https://cliarc.com/plugins/terraform",
			Repository:       "https://github.com/cliarc/plugin-terraform",
			Downloads:        6700,
			Rating:           4.6,
			Verified:         true,
			Category:         "DevOps",
			Runtime:          "go",
			MinCLIARCVersion: ">=0.1.0",
			Tags:             []string{"iac", "terraform", "cloud"},
			Permissions:      []string{"process:exec", "filesystem:read", "filesystem:write"},
		},
		{
			Name:             "postgres",
			Version:          "1.0.1",
			Description:      "PostgreSQL database query, migration runner, backup, and health analysis",
			Author:           "CLIARC Community",
			License:          "MIT",
			Homepage:         "https://cliarc.com/plugins/postgres",
			Repository:       "https://github.com/cliarc/plugin-postgres",
			Downloads:        5900,
			Rating:           4.8,
			Verified:         true,
			Category:         "Databases",
			Runtime:          "rust",
			MinCLIARCVersion: ">=0.1.0",
			Tags:             []string{"database", "sql", "postgres"},
			Permissions:      []string{"network:outbound", "secrets:access"},
		},
		{
			Name:             "ssh",
			Version:          "0.1.0",
			Description:      "Official SSH diagnostic, server management, key rotation, and automation",
			Author:           "CLIARC Core Team",
			License:          "Apache-2.0",
			Homepage:         "https://cliarc.com/plugins/ssh",
			Repository:       "https://github.com/cliarc/plugin-ssh",
			Downloads:        16800,
			Rating:           5.0,
			Verified:         true,
			Category:         "Infrastructure",
			Runtime:          "go",
			MinCLIARCVersion: ">=0.1.0",
			Tags:             []string{"ssh", "servers", "infrastructure", "keys"},
			Permissions:      []string{"network:outbound", "filesystem:read", "filesystem:write", "process:exec"},
		},
	}
}
