package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/cliarc/cliarc/internal/models"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Manage servers in the CLIARC inventory",
}

var serverAddCmd = &cobra.Command{
	Use:   "add [name]",
	Short: "Add a server to the inventory",
	Long: `Add a server interactively or via flags:
  cliarc server add server1 --host 157.173.220.167 --username root --key-path ~/.ssh/id_ed25519
  cliarc server add (starts interactive wizard)`,
	RunE: func(cmd *cobra.Command, args []string) error {
		servers, err := loadServers()
		if err != nil {
			return err
		}

		nameFlag, _ := cmd.Flags().GetString("name")
		hostFlag, _ := cmd.Flags().GetString("host")
		portFlag, _ := cmd.Flags().GetInt("port")
		userFlag, _ := cmd.Flags().GetString("username")
		keyFlag, _ := cmd.Flags().GetString("key-path")
		passFlag, _ := cmd.Flags().GetString("password")

		var s models.Server

		// Check if positional name was provided
		if len(args) > 0 {
			s.Name = args[0]
		} else if nameFlag != "" {
			s.Name = nameFlag
		}

		// If required flags are provided, use non-interactive mode
		if hostFlag != "" {
			s.Host = hostFlag
			s.Port = portFlag
			if s.Port == 0 {
				s.Port = 22
			}
			s.Username = userFlag
			s.SSHKeyID = expandPath(keyFlag)
			s.Password = passFlag
			if s.Name == "" {
				s.Name = s.Host
			}
		} else {
			// Interactive mode
			reader := bufio.NewReader(os.Stdin)

			if s.Name == "" {
				fmt.Print("Server Name: ")
				input, _ := reader.ReadString('\n')
				s.Name = strings.TrimSpace(input)
			}

			fmt.Print("Host / IP Address: ")
			input, _ := reader.ReadString('\n')
			s.Host = strings.TrimSpace(input)
			if s.Host == "" {
				return fmt.Errorf("host cannot be empty")
			}

			fmt.Print("Port [22]: ")
			input, _ = reader.ReadString('\n')
			input = strings.TrimSpace(input)
			if input == "" {
				s.Port = 22
			} else {
				s.Port, _ = strconv.Atoi(input)
				if s.Port == 0 {
					s.Port = 22
				}
			}

			fmt.Print("Username [root]: ")
			input, _ = reader.ReadString('\n')
			input = strings.TrimSpace(input)
			if input == "" {
				s.Username = "root"
			} else {
				s.Username = input
			}

			fmt.Print("SSH Private Key Path [~/.ssh/id_ed25519 or ~/.ssh/id_rsa]: ")
			input, _ = reader.ReadString('\n')
			input = strings.TrimSpace(input)
			if input != "" {
				s.SSHKeyID = expandPath(input)
			} else {
				defaultKeys := findDefaultSSHKeys()
				if len(defaultKeys) > 0 {
					s.SSHKeyID = defaultKeys[0]
					fmt.Printf("Using detected default key: %s\n", s.SSHKeyID)
				}
			}

			if s.SSHKeyID == "" {
				fmt.Print("Password (optional): ")
				input, _ = reader.ReadString('\n')
				s.Password = strings.TrimSpace(input)
			}
		}

		if s.SSHKeyID != "" {
			s.SSHKeyID = expandPath(s.SSHKeyID)
		}

		// Check if server with this name already exists
		for i, existing := range servers {
			if strings.EqualFold(existing.Name, s.Name) {
				s.ID = existing.ID
				servers[i] = s
				if err := saveServers(servers); err != nil {
					return err
				}
				fmt.Println(color.GreenString("✓ Updated existing server %q (%s@%s:%d)", s.Name, s.Username, s.Host, s.Port))
				return nil
			}
		}

		s.ID = fmt.Sprintf("srv-%d", len(servers)+1)
		servers = append(servers, s)

		if err := saveServers(servers); err != nil {
			return err
		}

		// If this is the only server, set as active automatically
		if len(servers) == 1 {
			_ = setActiveServerName(s.Name)
			fmt.Println(color.GreenString("✓ Added server %q (%s@%s:%d) [set as active]", s.Name, s.Username, s.Host, s.Port))
		} else {
			fmt.Println(color.GreenString("✓ Added server %q (%s@%s:%d)", s.Name, s.Username, s.Host, s.Port))
		}

		return nil
	},
}

var serverListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all configured servers",
	RunE: func(cmd *cobra.Command, args []string) error {
		servers, err := loadServers()
		if err != nil {
			return err
		}

		if len(servers) == 0 {
			fmt.Println("No servers configured in CLIARC inventory.")
			// Check if ~/.ssh/config has servers to suggest importing
			sshConfigPath := expandPath("~/.ssh/config")
			if sshServers, err := parseSSHConfigFile(sshConfigPath); err == nil && len(sshServers) > 0 {
				fmt.Printf("Found %d servers in %s. Run 'cliarc server import' to import them.\n", len(sshServers), sshConfigPath)
			} else {
				fmt.Println("Add a server with 'cliarc server add'.")
			}
			return nil
		}

		active, _ := getActiveServerName()

		fmt.Println(color.CyanString("Configured Servers:"))
		for _, s := range servers {
			isActive := strings.EqualFold(s.Name, active) || strings.EqualFold(s.ID, active)
			marker := "  "
			status := ""
			if isActive {
				marker = color.GreenString("► ")
				status = color.GreenString(" (active)")
			}
			keyInfo := ""
			if s.SSHKeyID != "" {
				keyInfo = fmt.Sprintf(" [key: %s]", s.SSHKeyID)
			}
			fmt.Printf("%s%-14s %s@%s:%d%s%s\n", marker, s.Name, s.Username, s.Host, s.Port, keyInfo, status)
		}
		return nil
	},
}

var serverImportCmd = &cobra.Command{
	Use:     "import [config-path]",
	Aliases: []string{"import-ssh"},
	Short:   "Import servers from OpenSSH ~/.ssh/config",
	Long: `Import servers defined in OpenSSH config (default: ~/.ssh/config).
Examples:
  cliarc server import
  cliarc server import ~/.ssh/config`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		configPath := "~/.ssh/config"
		if len(args) > 0 {
			configPath = args[0]
		}
		expandedPath := expandPath(configPath)

		sshServers, err := parseSSHConfigFile(expandedPath)
		if err != nil {
			return fmt.Errorf("failed to parse SSH config %q: %w", expandedPath, err)
		}

		if len(sshServers) == 0 {
			fmt.Println(color.YellowString("No host definitions found in %s", expandedPath))
			return nil
		}

		existing, _ := loadServers()
		existingMap := make(map[string]bool)
		for _, s := range existing {
			existingMap[strings.ToLower(s.Name)] = true
		}

		importedCount := 0
		for _, s := range sshServers {
			if existingMap[strings.ToLower(s.Name)] {
				// Update existing
				for i, ex := range existing {
					if strings.EqualFold(ex.Name, s.Name) {
						s.ID = ex.ID
						existing[i] = s
						break
					}
				}
			} else {
				s.ID = fmt.Sprintf("srv-%d", len(existing)+1)
				existing = append(existing, s)
				existingMap[strings.ToLower(s.Name)] = true
			}
			importedCount++
		}

		if err := saveServers(existing); err != nil {
			return fmt.Errorf("failed to save imported servers: %w", err)
		}

		// If no active server is set, set first imported as active
		active, _ := getActiveServerName()
		if active == "" && len(sshServers) > 0 {
			_ = setActiveServerName(sshServers[0].Name)
		}

		fmt.Println(color.GreenString("✓ Successfully imported %d servers from %s:", importedCount, expandedPath))
		for _, s := range sshServers {
			keyInfo := ""
			if s.SSHKeyID != "" {
				keyInfo = fmt.Sprintf(" [key: %s]", s.SSHKeyID)
			}
			fmt.Printf("  • %-12s %s@%s:%d%s\n", color.CyanString(s.Name), s.Username, s.Host, s.Port, keyInfo)
		}

		return nil
	},
}

var serverRemoveCmd = &cobra.Command{
	Use:     "remove [name]",
	Aliases: []string{"rm", "delete"},
	Short:   "Remove a server from the inventory",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		servers, err := loadServers()
		if err != nil {
			return err
		}

		var updated []models.Server
		found := false
		for _, s := range servers {
			if strings.EqualFold(s.Name, name) || strings.EqualFold(s.ID, name) {
				found = true
				continue
			}
			updated = append(updated, s)
		}

		if !found {
			return fmt.Errorf("server %q not found", name)
		}

		if err := saveServers(updated); err != nil {
			return err
		}

		active, _ := getActiveServerName()
		if strings.EqualFold(active, name) {
			_ = clearActiveServerName()
		}

		fmt.Println(color.GreenString("✓ Removed server %q", name))
		return nil
	},
}

var serverUseCmd = &cobra.Command{
	Use:   "use [name]",
	Short: "Set the active server (alias to 'cliarc use')",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return useCmd.RunE(cmd, args)
	},
}

func init() {
	serverCmd.AddCommand(serverAddCmd)
	serverCmd.AddCommand(serverListCmd)
	serverCmd.AddCommand(serverImportCmd)
	serverCmd.AddCommand(serverRemoveCmd)
	serverCmd.AddCommand(serverUseCmd)

	serverAddCmd.Flags().StringP("name", "n", "", "Server name")
	serverAddCmd.Flags().StringP("host", "H", "", "Host or IP address")
	serverAddCmd.Flags().IntP("port", "p", 22, "SSH port")
	serverAddCmd.Flags().StringP("username", "u", "root", "SSH username")
	serverAddCmd.Flags().StringP("key-path", "k", "", "Path to SSH private key (supports ~)")
	serverAddCmd.Flags().StringP("password", "P", "", "SSH password")
}
