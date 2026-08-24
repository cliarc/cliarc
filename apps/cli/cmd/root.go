package cmd

import (
	"fmt"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "cliarc",
	Short: "CLIARC — Extensible Developer Platform",
	Long: `CLIARC is a SaaS-level, production-grade, extensible plugin platform for managing
infrastructure, developer tools, cloud services, and diagnostics through a unified CLI.`,
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	// 1. CLIARC Core Commands
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(doctorCmd)
	rootCmd.AddCommand(configCmd)
	rootCmd.AddCommand(updateCmd)
	rootCmd.AddCommand(completionCmd)

	// 2. Plugin Management Layer
	rootCmd.AddCommand(pluginCmd)

	// 3. Plugin Development Layer
	rootCmd.AddCommand(devCmd)

	// 4. Register Dynamic Plugin Commands (docker, ssh, kubernetes, aws, etc.)
	RegisterDynamicPluginCommands(rootCmd)

	// Custom Stylized Help Template
	rootCmd.SetHelpTemplate(fmt.Sprintf(`%s

%s
  {{.UseLine}}

%s
  %s    CLIARC version and build info
  %s     Complete environment & toolchain check
  %s     Configuration management
  %s     CLIARC self-update and release checks
  %s completion Shell autocompletion script
  %s       Display help for any command

%s
  %s  Search available plugins in registry catalog
  %s Install a plugin from registry or local archive
  %s Uninstall and remove an installed plugin
  %s  Update installed plugin(s) to latest version
  %s    List all installed plugins and their status
  %s    Show detailed plugin metadata and binary specs
  %s  Enable an installed plugin
  %s Disable an installed plugin

%s
  %s     Scaffold a new plugin project template
  %s      Execute plugin locally in development mode
  %s     Run plugin unit and integration tests in tests/
  %s    Compile native executable binary/binaries
  %s  Create a distributable .tar.gz package
  %s  Validate and publish plugin package to registry
  %s     Link local plugin into ~/.cliarc/plugins for live testing
  %s   Remove linked development plugin
  %s Validate project structure, manifest, and binaries
  %s    Clean build artifacts and temporary files

%s
  {{.Flags.FlagUsages | trimTrailingWhitespaces}}

Use "{{.CommandPath}} [command] --help" for more information about a command.
`,
		color.CyanString("CLIARC Developer Platform — One Core. Unlimited Possibilities."),
		color.YellowString("USAGE:"),
		color.GreenString("CORE COMMANDS:"),
		color.CyanString("cliarc version"),
		color.CyanString("cliarc doctor "),
		color.CyanString("cliarc config "),
		color.CyanString("cliarc update "),
		color.CyanString("cliarc"),
		color.CyanString("cliarc help   "),
		color.GreenString("PLUGIN MANAGEMENT:"),
		color.CyanString("cliarc plugin search   "),
		color.CyanString("cliarc plugin install  "),
		color.CyanString("cliarc plugin uninstall"),
		color.CyanString("cliarc plugin update   "),
		color.CyanString("cliarc plugin list     "),
		color.CyanString("cliarc plugin info     "),
		color.CyanString("cliarc plugin enable   "),
		color.CyanString("cliarc plugin disable  "),
		color.GreenString("PLUGIN DEVELOPMENT:"),
		color.CyanString("cliarc dev init     "),
		color.CyanString("cliarc dev run      "),
		color.CyanString("cliarc dev test     "),
		color.CyanString("cliarc dev build    "),
		color.CyanString("cliarc dev package  "),
		color.CyanString("cliarc dev publish  "),
		color.CyanString("cliarc dev link     "),
		color.CyanString("cliarc dev unlink   "),
		color.CyanString("cliarc dev validate "),
		color.CyanString("cliarc dev clean    "),
		color.YellowString("FLAGS:"),
	))
}
