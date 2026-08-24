package cmd

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/cliarc/cliarc/core/config"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "View and manage CLIARC global configuration",
	Long: `Manage CLIARC configuration properties stored in ~/.cliarc/config.json.

Available Subcommands:
  • cliarc config              (view all settings)
  • cliarc config list         (view all settings)
  • cliarc config get <key>    (get specific setting)
  • cliarc config set <k> <v>  (update setting)
  • cliarc config path         (display config file path)
  • cliarc config reset        (restore defaults)

Available Keys:
  • plugin_dir                 (string, default: ~/.cliarc/plugins)
  • server_store_path          (string, default: ~/.cliarc/servers.json)
  • log_level                  (string: debug, info, warn, error)
  • plugin_timeout_seconds     (int, default: 30)
  • secret_provider            (string: keychain, env, file)
  • active_server              (string)`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return printConfig()
	},
}

var configListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"show"},
	Short:   "Display all configuration values",
	RunE: func(cmd *cobra.Command, args []string) error {
		return printConfig()
	},
}

var configGetCmd = &cobra.Command{
	Use:   "get <key>",
	Short: "Get a configuration value",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		key := strings.ToLower(strings.TrimSpace(args[0]))
		cfg, err := loadConfig()
		if err != nil {
			return err
		}

		switch key {
		case "plugin_dir", "plugindir", "plugins":
			fmt.Println(cfg.PluginDir)
		case "server_store_path", "serverstorepath", "servers":
			fmt.Println(cfg.ServerStorePath)
		case "log_level", "loglevel":
			fmt.Println(cfg.LogLevel)
		case "plugin_timeout_seconds", "plugin_timeout", "timeout":
			fmt.Println(cfg.PluginTimeout)
		case "secret_provider", "secretprovider":
			fmt.Println(cfg.SecretProvider)
		case "active_server", "activeserver":
			fmt.Println(cfg.ActiveServer)
		default:
			if extVal, ok := cfg.GetExtension(key); ok {
				fmt.Println(extVal)
				return nil
			}
			return fmt.Errorf("unknown configuration key %q", key)
		}
		return nil
	},
}

var configSetCmd = &cobra.Command{
	Use:   "set <key> <value>",
	Short: "Set a configuration value",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		key := strings.ToLower(strings.TrimSpace(args[0]))
		val := strings.TrimSpace(args[1])

		cfg, err := loadConfig()
		if err != nil {
			cfg = config.Default()
		}

		switch key {
		case "plugin_dir", "plugindir", "plugins":
			cfg.PluginDir = expandPath(val)
		case "server_store_path", "serverstorepath", "servers":
			cfg.ServerStorePath = expandPath(val)
		case "log_level", "loglevel":
			valLower := strings.ToLower(val)
			if valLower != "debug" && valLower != "info" && valLower != "warn" && valLower != "error" {
				return fmt.Errorf("invalid log_level %q (allowed: debug, info, warn, error)", val)
			}
			cfg.LogLevel = valLower
		case "plugin_timeout_seconds", "plugin_timeout", "timeout":
			sec, err := strconv.Atoi(val)
			if err != nil || sec <= 0 {
				return fmt.Errorf("plugin_timeout_seconds must be a positive integer")
			}
			cfg.PluginTimeout = sec
		case "secret_provider", "secretprovider":
			cfg.SecretProvider = val
		case "active_server", "activeserver":
			cfg.ActiveServer = val
		default:
			cfg.SetExtension(key, val)
		}

		if err := saveConfig(cfg); err != nil {
			return fmt.Errorf("failed to save configuration: %w", err)
		}

		fmt.Println(color.GreenString("✓ Updated configuration: %s = %s", key, val))
		return nil
	},
}

var configPathCmd = &cobra.Command{
	Use:   "path",
	Short: "Print path to the configuration file",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(getConfigFile())
	},
}

var configResetCmd = &cobra.Command{
	Use:   "reset",
	Short: "Reset configuration to defaults",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := config.Default()
		if err := saveConfig(cfg); err != nil {
			return fmt.Errorf("failed to reset configuration: %w", err)
		}
		fmt.Println(color.GreenString("✓ Reset CLIARC configuration to defaults in %s", getConfigFile()))
		return nil
	},
}

func printConfig() error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	fmt.Println(color.CyanString("CLIARC Configuration: (%s)", getConfigFile()))
	fmt.Printf("  • %-24s %s\n", "plugin_dir:", color.GreenString(cfg.PluginDir))
	fmt.Printf("  • %-24s %s\n", "server_store_path:", cfg.ServerStorePath)
	fmt.Printf("  • %-24s %s\n", "log_level:", cfg.LogLevel)
	fmt.Printf("  • %-24s %d seconds\n", "plugin_timeout_seconds:", cfg.PluginTimeout)
	fmt.Printf("  • %-24s %s\n", "secret_provider:", cfg.SecretProvider)
	activeServer := cfg.ActiveServer
	if activeServer == "" {
		activeServer = color.HiBlackString("(none)")
	} else {
		activeServer = color.GreenString(activeServer)
	}
	fmt.Printf("  • %-24s %s\n", "active_server:", activeServer)

	if len(cfg.Extensions) > 0 {
		fmt.Println(color.CyanString("\nExtensions:"))
		for k, v := range cfg.Extensions {
			valJSON, _ := json.Marshal(v)
			fmt.Printf("  • %-24s %s\n", k+":", string(valJSON))
		}
	}
	return nil
}

func init() {
	configCmd.AddCommand(configListCmd)
	configCmd.AddCommand(configGetCmd)
	configCmd.AddCommand(configSetCmd)
	configCmd.AddCommand(configPathCmd)
	configCmd.AddCommand(configResetCmd)
}
