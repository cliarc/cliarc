package manifest

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// ManifestFileName is the primary default manifest file name.
const ManifestFileName = "plugin.yaml"

// CandidateManifestFiles lists candidate filenames in order of preference.
var CandidateManifestFiles = []string{
	"plugin.yaml",
	"plugin.yml",
	"cliarc.plugin.yaml",
	"cliarc.plugin.yml",
	"plugin.json",
	"manifest.yaml",
	"manifest.yml",
}

// FindManifestInDir finds the first matching manifest file in a directory.
func FindManifestInDir(dir string) (string, bool) {
	for _, filename := range CandidateManifestFiles {
		p := filepath.Join(dir, filename)
		if stat, err := os.Stat(p); err == nil && !stat.IsDir() {
			return p, true
		}
	}
	return "", false
}

// AuthorInfo holds structured author contact metadata.
type AuthorInfo struct {
	Name     string `yaml:"name" json:"name"`
	Email    string `yaml:"email,omitempty" json:"email,omitempty"`
	Homepage string `yaml:"homepage,omitempty" json:"homepage,omitempty"`
}

// PluginBlock defines the 'plugin' section in Schema v1.
type PluginBlock struct {
	ID          string      `yaml:"id" json:"id"`
	Name        string      `yaml:"name" json:"name"`
	Version     string      `yaml:"version" json:"version"`
	Description string      `yaml:"description,omitempty" json:"description,omitempty"`
	Author      interface{} `yaml:"author,omitempty" json:"author,omitempty"` // string or AuthorInfo
	License     string      `yaml:"license,omitempty" json:"license,omitempty"`
	Homepage    string      `yaml:"homepage,omitempty" json:"homepage,omitempty"`
	Repository  string      `yaml:"repository,omitempty" json:"repository,omitempty"`
}

// CLISpec defines CLI compatibility versions.
type CLISpec struct {
	MinVersion string `yaml:"minVersion,omitempty" json:"minVersion,omitempty"`
	MaxVersion string `yaml:"maxVersion,omitempty" json:"maxVersion,omitempty"`
}

// VerificationSpec defines cryptographic package integrity verification.
type VerificationSpec struct {
	Algorithm string `yaml:"algorithm,omitempty" json:"algorithm,omitempty"` // e.g. "Ed25519"
	KeyID     string `yaml:"keyId,omitempty" json:"keyId,omitempty"`
	SHA256    string `yaml:"sha256,omitempty" json:"sha256,omitempty"`
	Signature string `yaml:"signature,omitempty" json:"signature,omitempty"`
}

// PackageSpec defines package format and entry artifact.
type PackageSpec struct {
	Format string `yaml:"format,omitempty" json:"format,omitempty"` // e.g. "wasm", "binary", "tar.gz"
	Entry  string `yaml:"entry,omitempty" json:"entry,omitempty"`
}

// FlagDefinition defines a command-line flag supported by a command.
type FlagDefinition struct {
	Name         string      `yaml:"name" json:"name"`
	Shorthand    string      `yaml:"shorthand,omitempty" json:"shorthand,omitempty"`
	Type         string      `yaml:"type,omitempty" json:"type,omitempty"` // string, bool, int
	DefaultValue interface{} `yaml:"default,omitempty" json:"default,omitempty"`
	Description  string      `yaml:"description,omitempty" json:"description,omitempty"`
	Required     bool        `yaml:"required,omitempty" json:"required,omitempty"`
}

// ArgDefinition defines a positional argument.
type ArgDefinition struct {
	Name        string `yaml:"name" json:"name"`
	Description string `yaml:"description,omitempty" json:"description,omitempty"`
	Required    bool   `yaml:"required,omitempty" json:"required,omitempty"`
	Variadic    bool   `yaml:"variadic,omitempty" json:"variadic,omitempty"`
}

// CommandDefinition defines a command or nested subcommand in the plugin command hierarchy.
type CommandDefinition struct {
	Name        string              `yaml:"name" json:"name"`
	Aliases     []string            `yaml:"aliases,omitempty" json:"aliases,omitempty"`
	Action      string              `yaml:"action,omitempty" json:"action,omitempty"`
	Handler     string              `yaml:"handler,omitempty" json:"handler,omitempty"`
	Usage       string              `yaml:"usage,omitempty" json:"usage,omitempty"`
	Description string              `yaml:"description,omitempty" json:"description,omitempty"`
	Permissions []string            `yaml:"permissions,omitempty" json:"permissions,omitempty"`
	Args        []ArgDefinition     `yaml:"args,omitempty" json:"args,omitempty"`
	Flags       []FlagDefinition    `yaml:"flags,omitempty" json:"flags,omitempty"`
	Subcommands []CommandDefinition `yaml:"subcommands,omitempty" json:"subcommands,omitempty"`
	Examples    []string            `yaml:"examples,omitempty" json:"examples,omitempty"`
}

// Manifest defines the complete plugin manifest structure supporting Schema v1 and legacy formats.
type Manifest struct {
	SchemaVersion    int                    `yaml:"schemaVersion,omitempty" json:"schemaVersion,omitempty"`
	Plugin           *PluginBlock           `yaml:"plugin,omitempty" json:"plugin,omitempty"`
	ID               string                 `yaml:"id,omitempty" json:"id,omitempty"`
	Name             string                 `yaml:"name,omitempty" json:"name,omitempty"`
	Version          string                 `yaml:"version,omitempty" json:"version,omitempty"`
	ProtocolVersion  string                 `yaml:"protocol_version,omitempty" json:"protocol_version,omitempty"`
	Description      string                 `yaml:"description,omitempty" json:"description,omitempty"`
	Author           string                 `yaml:"author,omitempty" json:"author,omitempty"`
	AuthorInfo       *AuthorInfo            `yaml:"-" json:"-"`
	License          string                 `yaml:"license,omitempty" json:"license,omitempty"`
	Homepage         string                 `yaml:"homepage,omitempty" json:"homepage,omitempty"`
	Repository       string                 `yaml:"repository,omitempty" json:"repository,omitempty"`
	Binary           string                 `yaml:"binary,omitempty" json:"binary,omitempty"`
	CLI              *CLISpec               `yaml:"cli,omitempty" json:"cli,omitempty"`
	MinCLIARCVersion string                 `yaml:"min_cliarc_version,omitempty" json:"min_cliarc_version,omitempty"`
	MaxCLIARCVersion string                 `yaml:"max_cliarc_version,omitempty" json:"max_cliarc_version,omitempty"`
	Platforms        []string               `yaml:"platforms,omitempty" json:"platforms,omitempty"`
	Architectures    []string               `yaml:"architectures,omitempty" json:"architectures,omitempty"`
	Permissions      []string               `yaml:"permissions,omitempty" json:"permissions,omitempty"`
	Verification     *VerificationSpec      `yaml:"verification,omitempty" json:"verification,omitempty"`
	Package          *PackageSpec           `yaml:"package,omitempty" json:"package,omitempty"`
	Dependencies     map[string]string      `yaml:"dependencies,omitempty" json:"dependencies,omitempty"`
	Commands         []string               `yaml:"-" json:"-"` // Flat list of actions for gRPC registration
	CommandTree      []CommandDefinition    `yaml:"command_tree,omitempty" json:"command_tree,omitempty"`
	RawCommands      yaml.Node              `yaml:"commands,omitempty" json:"-"`
	Actions          []string               `yaml:"actions,omitempty" json:"actions,omitempty"`
	Runtime          RuntimeSpec            `yaml:"runtime,omitempty" json:"runtime,omitempty"`
	Dir              string                 `yaml:"-" json:"-"`
	Marketplace      *MarketplaceSpec       `yaml:"marketplace,omitempty" json:"marketplace,omitempty"`
	ConfigSchema     map[string]interface{} `yaml:"config_schema,omitempty" json:"config_schema,omitempty"`
}

// RuntimeSpec defines how the plugin process/bridge is executed.
type RuntimeSpec struct {
	Type    string            `yaml:"type,omitempty" json:"type,omitempty"` // "wasm", "executable", "grpc", "node", "python", "go"
	Entry   string            `yaml:"entry,omitempty" json:"entry,omitempty"`
	Command string            `yaml:"command,omitempty" json:"command,omitempty"`
	WASI    bool              `yaml:"wasi,omitempty" json:"wasi,omitempty"`
	Args    []string          `yaml:"args,omitempty" json:"args,omitempty"`
	Env     map[string]string `yaml:"env,omitempty" json:"env,omitempty"`
}

// MarketplaceSpec stores registry listing metadata.
type MarketplaceSpec struct {
	Category    string   `yaml:"category,omitempty" json:"category,omitempty"`
	Tags        []string `yaml:"tags,omitempty" json:"tags,omitempty"`
	IconURL     string   `yaml:"icon_url,omitempty" json:"icon_url,omitempty"`
	Repository  string   `yaml:"repository,omitempty" json:"repository,omitempty"`
	License     string   `yaml:"license,omitempty" json:"license,omitempty"`
	Verified    bool     `yaml:"verified,omitempty" json:"verified,omitempty"`
	Downloads   int64    `yaml:"downloads,omitempty" json:"downloads,omitempty"`
	Rating      float64  `yaml:"rating,omitempty" json:"rating,omitempty"`
	DownloadURL string   `yaml:"download_url,omitempty" json:"download_url,omitempty"`
	Checksum    string   `yaml:"checksum,omitempty" json:"checksum,omitempty"`
}

// Validate verifies manifest rules and normalizes fields across Schema v1 and legacy formats.
func (m *Manifest) Validate() error {
	// Normalize from Plugin block (Schema v1)
	if m.Plugin != nil {
		if m.Plugin.ID != "" && m.ID == "" {
			m.ID = m.Plugin.ID
		}
		if m.Plugin.Name != "" && m.Name == "" {
			m.Name = m.Plugin.Name
		}
		if m.Name == "" && m.ID != "" {
			m.Name = m.ID
		}
		if m.Plugin.Version != "" && m.Version == "" {
			m.Version = m.Plugin.Version
		}
		if m.Plugin.Description != "" && m.Description == "" {
			m.Description = m.Plugin.Description
		}
		if m.Plugin.License != "" && m.License == "" {
			m.License = m.Plugin.License
		}
		if m.Plugin.Homepage != "" && m.Homepage == "" {
			m.Homepage = m.Plugin.Homepage
		}
		if m.Plugin.Repository != "" && m.Repository == "" {
			m.Repository = m.Plugin.Repository
		}
		if m.Plugin.Author != nil {
			switch v := m.Plugin.Author.(type) {
			case string:
				m.Author = v
			case map[string]interface{}:
				if n, ok := v["name"].(string); ok {
					m.Author = n
				}
				m.AuthorInfo = &AuthorInfo{
					Name:     fmt.Sprint(v["name"]),
					Email:    fmt.Sprint(v["email"]),
					Homepage: fmt.Sprint(v["homepage"]),
				}
			}
		}
	}

	if m.Name == "" && m.ID != "" {
		m.Name = m.ID
	}
	m.Name = strings.TrimSpace(m.Name)
	if m.Name == "" {
		return fmt.Errorf("manifest: plugin name or id is required")
	}

	m.Version = strings.TrimSpace(m.Version)
	if m.Version == "" {
		return fmt.Errorf("manifest: version is required")
	}

	if m.ProtocolVersion == "" {
		m.ProtocolVersion = "1"
	}

	// Normalize CLI block
	if m.CLI != nil {
		if m.CLI.MinVersion != "" && m.MinCLIARCVersion == "" {
			m.MinCLIARCVersion = m.CLI.MinVersion
		}
		if m.CLI.MaxVersion != "" && m.MaxCLIARCVersion == "" {
			m.MaxCLIARCVersion = m.CLI.MaxVersion
		}
	}

	// Parse RawCommands (map or list)
	m.parseRawCommands()

	// Unify Actions and flat Commands
	if len(m.Commands) > 0 && len(m.Actions) == 0 {
		m.Actions = append([]string{}, m.Commands...)
	} else if len(m.Actions) > 0 && len(m.Commands) == 0 {
		m.Commands = append([]string{}, m.Actions...)
	}

	// Ensure CommandTree has entries if flat actions exist
	if len(m.CommandTree) == 0 && len(m.Actions) > 0 {
		for _, action := range m.Actions {
			parts := strings.Split(action, ".")
			cmdName := action
			if len(parts) > 1 {
				cmdName = strings.Join(parts[1:], " ")
			}
			m.CommandTree = append(m.CommandTree, CommandDefinition{
				Name:        cmdName,
				Action:      action,
				Handler:     action,
				Description: fmt.Sprintf("Execute %s action", action),
			})
		}
	}

	if len(m.Actions) == 0 && len(m.Commands) == 0 && len(m.CommandTree) == 0 {
		return fmt.Errorf("manifest: at least one command or action is required")
	}

	// Normalize runtime & entry
	if m.Package != nil {
		if m.Package.Entry != "" && m.Runtime.Entry == "" {
			m.Runtime.Entry = m.Package.Entry
		}
	}
	if m.Runtime.Entry != "" && m.Runtime.Command == "" {
		m.Runtime.Command = m.Runtime.Entry
	}
	if m.Binary != "" && m.Runtime.Command == "" {
		m.Runtime.Command = m.Binary
	}
	if m.Runtime.Command != "" && m.Binary == "" {
		m.Binary = m.Runtime.Command
	}
	if m.Runtime.Type == "" {
		if strings.HasSuffix(m.Runtime.Entry, ".wasm") {
			m.Runtime.Type = "wasm"
		} else {
			m.Runtime.Type = "executable"
		}
	}
	if m.Runtime.Command == "" && m.Binary == "" && m.Runtime.Entry == "" {
		m.Binary = filepath.Join("bin", "cliarc-"+strings.ToLower(m.Name))
		m.Runtime.Command = m.Binary
	}

	// Validate permissions
	for _, perm := range m.Permissions {
		if strings.TrimSpace(perm) == "" {
			return fmt.Errorf("manifest: empty permission string")
		}
	}

	return nil
}

func (m *Manifest) parseRawCommands() {
	if m.RawCommands.Kind == 0 {
		return
	}

	// 1. MappingNode: commands: { ps: { description: "...", handler: "..." } }
	if m.RawCommands.Kind == yaml.MappingNode {
		for i := 0; i < len(m.RawCommands.Content); i += 2 {
			keyNode := m.RawCommands.Content[i]
			valNode := m.RawCommands.Content[i+1]

			cmdName := keyNode.Value
			var item struct {
				Description string              `yaml:"description"`
				Handler     string              `yaml:"handler"`
				Action      string              `yaml:"action"`
				Usage       string              `yaml:"usage"`
				Permissions []string            `yaml:"permissions"`
				Aliases     []string            `yaml:"aliases"`
				Flags       []FlagDefinition    `yaml:"flags"`
				Args        []ArgDefinition     `yaml:"args"`
				Subcommands []CommandDefinition `yaml:"subcommands"`
			}
			_ = valNode.Decode(&item)

			action := item.Handler
			if action == "" {
				action = item.Action
			}
			if action == "" {
				action = strings.ToLower(m.Name) + "." + cmdName
			}

			cmdDef := CommandDefinition{
				Name:        cmdName,
				Description: item.Description,
				Handler:     action,
				Action:      action,
				Usage:       item.Usage,
				Permissions: item.Permissions,
				Aliases:     item.Aliases,
				Flags:       item.Flags,
				Args:        item.Args,
				Subcommands: item.Subcommands,
			}

			m.CommandTree = append(m.CommandTree, cmdDef)
			m.collectActions(cmdDef)
		}
		return
	}

	// 2. SequenceNode: list of strings or list of CommandDefinition objects
	if m.RawCommands.Kind == yaml.SequenceNode {
		var strList []string
		if err := m.RawCommands.Decode(&strList); err == nil && len(strList) > 0 {
			m.Commands = strList
			return
		}

		var cmdList []CommandDefinition
		if err := m.RawCommands.Decode(&cmdList); err == nil && len(cmdList) > 0 {
			m.CommandTree = cmdList
			for _, c := range cmdList {
				m.collectActions(c)
			}
		}
	}
}

func (m *Manifest) collectActions(cmd CommandDefinition) {
	action := cmd.Handler
	if action == "" {
		action = cmd.Action
	}
	if action != "" {
		m.Commands = append(m.Commands, action)
	} else if len(cmd.Subcommands) == 0 {
		m.Commands = append(m.Commands, strings.ToLower(m.Name)+"."+cmd.Name)
	}
	for _, sub := range cmd.Subcommands {
		m.collectActions(sub)
	}
}

// Load reads and validates a manifest from a file path or directory.
func Load(path string) (*Manifest, error) {
	targetPath := path
	stat, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("manifest: read file: %w", err)
	}

	if stat.IsDir() {
		found, ok := FindManifestInDir(path)
		if !ok {
			return nil, fmt.Errorf("manifest: no plugin.yaml or cliarc.plugin.yaml found in %s", path)
		}
		targetPath = found
	}

	data, err := os.ReadFile(targetPath)
	if err != nil {
		return nil, fmt.Errorf("manifest: read file: %w", err)
	}

	var m Manifest
	if strings.HasSuffix(targetPath, ".json") {
		if err := json.Unmarshal(data, &m); err != nil {
			return nil, fmt.Errorf("manifest: json unmarshal: %w", err)
		}
	} else {
		if err := yaml.Unmarshal(data, &m); err != nil {
			return nil, fmt.Errorf("manifest: yaml unmarshal: %w", err)
		}
	}

	if err := m.Validate(); err != nil {
		return nil, err
	}

	absPath, err := filepath.Abs(targetPath)
	if err == nil {
		m.Dir = filepath.Dir(absPath)
	} else {
		m.Dir = filepath.Dir(targetPath)
	}

	return &m, nil
}
