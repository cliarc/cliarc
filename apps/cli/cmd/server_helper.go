package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/cliarc/cliarc/core/config"
	"github.com/cliarc/cliarc/internal/models"
	"github.com/spf13/cobra"
)

// ServerTarget represents resolved connection parameters.
type ServerTarget struct {
	ServerName     string
	Host           string
	Port           int
	Username       string
	SSHKeyPath     string
	SSHKeyPassword string
	Password       string
	Timeout        int
}

// expandPath expands ~ and environment variables in a file path.
func expandPath(p string) string {
	if p == "" {
		return ""
	}
	if strings.HasPrefix(p, "~") {
		home, err := os.UserHomeDir()
		if err == nil {
			if p == "~" {
				return home
			}
			if strings.HasPrefix(p, "~/") || strings.HasPrefix(p, "~\\") {
				return filepath.Join(home, p[2:])
			}
			return filepath.Join(home, p[1:])
		}
	}
	return os.ExpandEnv(p)
}

// findDefaultSSHKeys returns available standard SSH private keys in ~/.ssh.
func findDefaultSSHKeys() []string {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	sshDir := filepath.Join(home, ".ssh")
	standardNames := []string{"id_ed25519", "id_rsa", "id_ecdsa", "id_dsa"}
	var found []string
	for _, name := range standardNames {
		p := filepath.Join(sshDir, name)
		if stat, err := os.Stat(p); err == nil && !stat.IsDir() {
			found = append(found, p)
		}
	}
	return found
}

// loadServers reads the server catalog from disk.
func loadServers() ([]models.Server, error) {
	cfg := config.Default()
	data, err := os.ReadFile(cfg.ServerStorePath)
	if err != nil {
		if os.IsNotExist(err) {
			return []models.Server{}, nil
		}
		return nil, fmt.Errorf("read servers: %w", err)
	}

	var servers []models.Server
	if err := json.Unmarshal(data, &servers); err != nil {
		return nil, fmt.Errorf("unmarshal servers: %w", err)
	}
	return servers, nil
}

// saveServers persists the server catalog to disk.
func saveServers(servers []models.Server) error {
	cfg := config.Default()
	if err := os.MkdirAll(filepath.Dir(cfg.ServerStorePath), 0750); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}
	data, err := json.MarshalIndent(servers, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal servers: %w", err)
	}
	return os.WriteFile(cfg.ServerStorePath, data, 0600)
}

// parseSSHConfigFile parses an OpenSSH config file (e.g. ~/.ssh/config).
func parseSSHConfigFile(configPath string) ([]models.Server, error) {
	configPath = expandPath(configPath)
	file, err := os.Open(configPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var servers []models.Server
	var currentServer *models.Server
	globalKey := ""

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}

		key := strings.ToLower(parts[0])
		val := parts[1]

		switch key {
		case "host":
			if currentServer != nil && currentServer.Host != "" {
				if currentServer.SSHKeyID == "" && globalKey != "" {
					currentServer.SSHKeyID = globalKey
				}
				if currentServer.Port == 0 {
					currentServer.Port = 22
				}
				if currentServer.Username == "" {
					currentServer.Username = "root"
				}
				servers = append(servers, *currentServer)
			}

			// If wildcard Host *, capture global defaults
			if val == "*" {
				currentServer = nil
				continue
			}

			currentServer = &models.Server{
				ID:   fmt.Sprintf("ssh-%s", val),
				Name: val,
				Port: 22,
			}

		case "hostname":
			if currentServer != nil {
				currentServer.Host = val
			}
		case "user":
			if currentServer != nil {
				currentServer.Username = val
			}
		case "port":
			if currentServer != nil {
				p, _ := strconv.Atoi(val)
				if p > 0 {
					currentServer.Port = p
				}
			}
		case "identityfile":
			expandedKey := expandPath(val)
			if currentServer != nil {
				currentServer.SSHKeyID = expandedKey
			} else {
				globalKey = expandedKey
			}
		}
	}

	if currentServer != nil && currentServer.Host != "" {
		if currentServer.SSHKeyID == "" && globalKey != "" {
			currentServer.SSHKeyID = globalKey
		}
		if currentServer.Port == 0 {
			currentServer.Port = 22
		}
		if currentServer.Username == "" {
			currentServer.Username = "root"
		}
		servers = append(servers, *currentServer)
	}

	return servers, scanner.Err()
}

// findServer finds a server by name, ID, or host from servers.json, then falls back to ~/.ssh/config.
func findServer(query string) (*models.Server, error) {
	// 1. Check ~/.cliarc/servers.json
	servers, err := loadServers()
	if err == nil {
		// Exact match on Name or ID
		for _, s := range servers {
			if strings.EqualFold(s.Name, query) || strings.EqualFold(s.ID, query) {
				return &s, nil
			}
		}
		// Match on Host
		for _, s := range servers {
			if strings.EqualFold(s.Host, query) {
				return &s, nil
			}
		}
	}

	// 2. Check ~/.ssh/config as fallback
	sshConfigPath := expandPath("~/.ssh/config")
	if sshServers, err := parseSSHConfigFile(sshConfigPath); err == nil {
		for _, s := range sshServers {
			if strings.EqualFold(s.Name, query) || strings.EqualFold(s.Host, query) {
				return &s, nil
			}
		}
	}

	return nil, fmt.Errorf("server %q not found in CLIARC inventory or ~/.ssh/config", query)
}

// getConfigFile returns the path to ~/.cliarc/config.json.
func getConfigFile() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".cliarc", "config.json")
}

// loadConfig reads ~/.cliarc/config.json or returns default.
func loadConfig() (*config.Config, error) {
	return config.Load(getConfigFile())
}

// saveConfig saves ~/.cliarc/config.json.
func saveConfig(cfg *config.Config) error {
	return cfg.Save(getConfigFile())
}

// getActiveServerName returns the currently active server name from config.
func getActiveServerName() (string, error) {
	cfg, err := loadConfig()
	if err != nil {
		return "", err
	}
	return cfg.ActiveServer, nil
}

// setActiveServerName sets the active server name in config.
func setActiveServerName(name string) error {
	cfg, err := loadConfig()
	if err != nil {
		cfg = config.Default()
	}
	cfg.ActiveServer = name
	return saveConfig(cfg)
}

// clearActiveServerName removes the active server from config.
func clearActiveServerName() error {
	return setActiveServerName("")
}

// resolveServerTarget combines server catalog lookup, active server, and CLI flags.
func resolveServerTarget(cmd *cobra.Command, args []string, positionalIndex int) (ServerTarget, error) {
	target := ServerTarget{
		Port:    22,
		Timeout: 10,
	}

	var matchedServer *models.Server

	// 1. Check if a positional server argument was passed
	if positionalIndex >= 0 && len(args) > positionalIndex && args[positionalIndex] != "" {
		argVal := args[positionalIndex]
		srv, err := findServer(argVal)
		if err == nil {
			matchedServer = srv
		} else {
			// If not found in servers.json / ssh config, use argument as host
			target.Host = argVal
		}
	}

	// 2. If no positional match, check if active server is set
	if matchedServer == nil && target.Host == "" {
		active, _ := getActiveServerName()
		if active != "" {
			srv, err := findServer(active)
			if err == nil {
				matchedServer = srv
			}
		}
	}

	// 3. Apply matched server attributes
	if matchedServer != nil {
		target.ServerName = matchedServer.Name
		target.Host = matchedServer.Host
		if matchedServer.Port > 0 {
			target.Port = matchedServer.Port
		}
		target.Username = matchedServer.Username
		target.SSHKeyPath = expandPath(matchedServer.SSHKeyID)
		target.Password = matchedServer.Password
	}

	// 4. Override with explicit CLI flags if set
	if cmd != nil {
		if cmd.Flags().Changed("host") {
			target.Host, _ = cmd.Flags().GetString("host")
		}
		if cmd.Flags().Changed("port") {
			target.Port, _ = cmd.Flags().GetInt("port")
		}
		if cmd.Flags().Changed("username") {
			target.Username, _ = cmd.Flags().GetString("username")
		}
		if cmd.Flags().Changed("key-path") {
			rawKey, _ := cmd.Flags().GetString("key-path")
			target.SSHKeyPath = expandPath(rawKey)
		}
		if cmd.Flags().Changed("password") {
			target.Password, _ = cmd.Flags().GetString("password")
		}
		if cmd.Flags().Changed("timeout") {
			target.Timeout, _ = cmd.Flags().GetInt("timeout")
		}
	}

	// 5. If no SSH key or password was specified, auto-detect default SSH key
	if target.SSHKeyPath == "" && target.Password == "" {
		defaultKeys := findDefaultSSHKeys()
		if len(defaultKeys) > 0 {
			target.SSHKeyPath = defaultKeys[0]
		}
	} else if target.SSHKeyPath != "" {
		target.SSHKeyPath = expandPath(target.SSHKeyPath)
	}

	// Default username to "root" if empty
	if target.Username == "" {
		target.Username = "root"
	}

	// 6. Validation
	if target.Host == "" {
		return target, fmt.Errorf("no target host or server specified.\nUse: cliarc ssh <command> <server-name>, or set an active server with 'cliarc use <server-name>', or provide '--host <hostname>'")
	}

	return target, nil
}
