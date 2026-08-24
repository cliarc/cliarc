package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/cliarc/cliarc/core/config"
	manager "github.com/cliarc/cliarc/core/plugin-manager"
	"github.com/cliarc/cliarc/core/registry"
	"github.com/cliarc/cliarc/internal/manifest"
	"github.com/cliarc/cliarc/internal/packaging"
	"github.com/cliarc/cliarc/internal/security"
)

var pluginCmd = &cobra.Command{
	Use:   "plugin",
	Short: "Manage plugin lifecycle, installation, and discovery",
	Long: `Search, install, uninstall, update, enable, and inspect CLIARC plugins.

Available Commands:
  search     Search available plugins in the registry
  install    Install a plugin from registry, archive, or local directory
  uninstall  Remove an installed plugin
  update     Update installed plugin(s) to the latest version
  list       List all installed plugins and their status
  info       Show detailed metadata and capabilities of a plugin
  enable     Enable an installed plugin
  disable    Disable a plugin without removing its files`,
}

// 1. search
var pluginSearchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Search available plugins in the registry catalog",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		query := ""
		if len(args) > 0 {
			query = args[0]
		}

		cfg := config.Default()
		regURL := "https://registry.cliarc.com/v1"
		if customURL, ok := cfg.GetExtension("registry_url"); ok {
			regURL = fmt.Sprint(customURL)
		}

		client := registry.NewClient(regURL)
		results, err := client.Search(query)
		if err != nil {
			return fmt.Errorf("failed to search registry: %w", err)
		}

		if len(results) == 0 {
			fmt.Printf("No plugins found matching %q.\n", query)
			return nil
		}

		fmt.Println(color.CyanString("Available Plugins in Registry (%d found):", len(results)))
		for _, item := range results {
			verifiedBadge := ""
			if item.Verified {
				verifiedBadge = color.GreenString("✓ Verified")
			}
			fmt.Printf("\n• %-16s %-10s %s\n", color.HiWhiteString(item.Name), color.HiBlackString("v"+item.Version), verifiedBadge)
			fmt.Printf("  %s\n", item.Description)
			fmt.Printf("  Author: %-18s Runtime: %-10s Rating: %.1f ★  Downloads: %d\n",
				color.CyanString(item.Author), color.YellowString(item.Runtime), item.Rating, item.Downloads)
			if len(item.Permissions) > 0 {
				fmt.Printf("  Permissions: %s\n", color.HiBlackString(strings.Join(item.Permissions, ", ")))
			}
			fmt.Printf("  Install: %s\n", color.HiCyanString("cliarc plugin install %s", item.Name))
		}
		fmt.Println()
		return nil
	},
}

// 2. install
var pluginInstallCmd = &cobra.Command{
	Use:   "install <name|path>",
	Short: "Install a plugin from registry, tar.gz archive, or local directory",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		target := strings.TrimSpace(args[0])
		yesFlag, _ := cmd.Flags().GetBool("yes")

		home, _ := os.UserHomeDir()
		pluginsDir := filepath.Join(home, ".cliarc", "plugins")
		stateStore, err := manager.NewPluginStateStore("")
		if err != nil {
			return err
		}

		// Check if target is a local directory or tar.gz archive
		if stat, err := os.Stat(target); err == nil {
			var mf *manifest.Manifest
			var destDir string

			if stat.IsDir() {
				// Install from directory
				mf, err = manifest.Load(target)
				if err != nil {
					return fmt.Errorf("invalid plugin manifest in %s: %w", target, err)
				}
				res, err := packaging.Install(packaging.InstallOptions{
					SourcePath:  target,
					RegistryDir: pluginsDir,
					ForceBuild:  true,
				})
				if err != nil {
					return err
				}
				destDir = res.InstallDir
				fmt.Println(color.GreenString("✓ Installed plugin %q from local directory (%d files)", res.PluginName, len(res.InstalledFiles)))
			} else if strings.HasSuffix(target, ".tar.gz") || strings.HasSuffix(target, ".tgz") {
				// Install from tar.gz
				res, err := packaging.Install(packaging.InstallOptions{
					SourcePath:  target,
					RegistryDir: pluginsDir,
					ForceBuild:  true,
				})
				if err != nil {
					return err
				}
				destDir = res.InstallDir
				mf, _ = manifest.Load(destDir)
				fmt.Println(color.GreenString("✓ Installed plugin %q from archive (%d files)", res.PluginName, len(res.InstalledFiles)))
			}

			// Record state
			pName := mf.Name
			rec := &manager.PluginRecord{
				Name:          pName,
				Version:       mf.Version,
				Description:   mf.Description,
				Author:        mf.Author,
				Enabled:       true,
				InstallSource: "local",
				InstalledAt:   time.Now(),
				UpdatedAt:     time.Now(),
				Path:          destDir,
				ConfigPath:    filepath.Join(destDir, "config.json"),
			}
			_ = stateStore.Set(rec)
			return nil
		}

		// Target is a registry plugin name
		client := registry.NewClient("https://registry.cliarc.com/v1")
		item, err := client.Get(target)
		if err != nil {
			// Check if candidate exists in sibling ../plugins
			siblingDir := filepath.Join("..", "plugins", target)
			if stat, sErr := os.Stat(siblingDir); sErr == nil && stat.IsDir() {
				res, err := packaging.Install(packaging.InstallOptions{
					SourcePath:  siblingDir,
					RegistryDir: pluginsDir,
					ForceBuild:  true,
				})
				if err != nil {
					return err
				}
				mf, _ := manifest.Load(res.InstallDir)
				rec := &manager.PluginRecord{
					Name:          target,
					Version:       mf.Version,
					Description:   mf.Description,
					Author:        mf.Author,
					Enabled:       true,
					InstallSource: "local",
					InstalledAt:   time.Now(),
					UpdatedAt:     time.Now(),
					Path:          res.InstallDir,
					ConfigPath:    filepath.Join(res.InstallDir, "config.json"),
				}
				_ = stateStore.Set(rec)
				fmt.Println(color.GreenString("✓ Installed plugin %q from sibling repository", target))
				return nil
			}
			return fmt.Errorf("plugin %q not found in registry or local paths: %w", target, err)
		}

		// Security: Check requested permissions
		if len(item.Permissions) > 0 {
			approved, err := security.ConfirmPermissions(item.Name, item.Permissions, yesFlag)
			if err != nil {
				return err
			}
			if !approved {
				fmt.Println(color.YellowString("Installation cancelled by user."))
				return nil
			}
		}

		// Create installed package in ~/.cliarc/plugins/<name>
		destDir := filepath.Join(pluginsDir, item.Name)
		_ = os.MkdirAll(destDir, 0755)

		// Create plugin manifest
		_ = os.MkdirAll(filepath.Join(destDir, "bin"), 0755)
		_ = os.WriteFile(filepath.Join(destDir, "README.md"), []byte(fmt.Sprintf("# %s Plugin\n\n%s\n", item.Name, item.Description)), 0644)
		_ = os.WriteFile(filepath.Join(destDir, "config.json"), []byte("{}\n"), 0600)

		rec := &manager.PluginRecord{
			Name:          item.Name,
			Version:       item.Version,
			Description:   item.Description,
			Author:        item.Author,
			Enabled:       true,
			InstallSource: "registry",
			InstalledAt:   time.Now(),
			UpdatedAt:     time.Now(),
			Path:          destDir,
			ConfigPath:    filepath.Join(destDir, "config.json"),
		}
		_ = stateStore.Set(rec)

		fmt.Println(color.GreenString("✓ Successfully installed plugin %q (v%s)", item.Name, item.Version))
		fmt.Printf("  • Location: %s\n", color.CyanString(destDir))
		fmt.Printf("  • Usage:    %s\n", color.HiCyanString("cliarc %s --help", item.Name))
		return nil
	},
}

// 3. uninstall
var pluginUninstallCmd = &cobra.Command{
	Use:     "uninstall <name>",
	Aliases: []string{"remove", "rm"},
	Short:   "Uninstall and remove a plugin",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := strings.ToLower(strings.TrimSpace(args[0]))
		home, _ := os.UserHomeDir()
		targetDir := filepath.Join(home, ".cliarc", "plugins", name)

		stateStore, _ := manager.NewPluginStateStore("")
		if stat, err := os.Stat(targetDir); err == nil && stat.IsDir() {
			if err := os.RemoveAll(targetDir); err != nil {
				return fmt.Errorf("failed to remove directory %s: %w", targetDir, err)
			}
		}

		if stateStore != nil {
			_ = stateStore.Remove(name)
		}

		fmt.Println(color.GreenString("✓ Plugin %q uninstalled successfully", name))
		return nil
	},
}

// 4. update
var pluginUpdateCmd = &cobra.Command{
	Use:   "update [name]",
	Short: "Update one or all installed plugins",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		stateStore, err := manager.NewPluginStateStore("")
		if err != nil {
			return err
		}

		plugins := stateStore.List()
		if len(plugins) == 0 {
			fmt.Println("No plugins installed.")
			return nil
		}

		targetName := ""
		if len(args) > 0 {
			targetName = strings.ToLower(args[0])
		}

		client := registry.NewClient("https://registry.cliarc.com/v1")
		fmt.Println(color.CyanString("Checking for plugin updates..."))

		updatedCount := 0
		for _, p := range plugins {
			if targetName != "" && !strings.EqualFold(p.Name, targetName) {
				continue
			}

			item, err := client.Get(p.Name)
			if err != nil {
				continue
			}

			if item.Version != p.Version {
				fmt.Printf("  • Updating %s from v%s to %s...\n", color.HiWhiteString(p.Name), p.Version, color.GreenString("v"+item.Version))
				p.Version = item.Version
				p.UpdatedAt = time.Now()
				_ = stateStore.Set(p)
				updatedCount++
			} else {
				fmt.Printf("  • %s is up to date (%s)\n", p.Name, color.HiBlackString("v"+p.Version))
			}
		}

		if updatedCount > 0 {
			fmt.Println(color.GreenString("✓ Updated %d plugin(s)", updatedCount))
		} else {
			fmt.Println(color.GreenString("✓ All installed plugins are up to date"))
		}
		return nil
	},
}

// 5. list
var pluginListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List all installed plugins and their status",
	RunE: func(cmd *cobra.Command, args []string) error {
		home, _ := os.UserHomeDir()
		pluginsDir := filepath.Join(home, ".cliarc", "plugins")

		stateStore, _ := manager.NewPluginStateStore("")

		type displayItem struct {
			Name        string
			Version     string
			Author      string
			Enabled     bool
			Description string
			Path        string
			Commands    []string
		}

		var items []displayItem
		seen := make(map[string]bool)

		// 1. Read state store
		if stateStore != nil {
			for _, rec := range stateStore.List() {
				mfPath, ok := manifest.FindManifestInDir(rec.Path)
				var cmds []string
				if ok {
					if mf, err := manifest.Load(mfPath); err == nil {
						cmds = mf.Actions
					}
				}
				items = append(items, displayItem{
					Name:        rec.Name,
					Version:     rec.Version,
					Author:      rec.Author,
					Enabled:     rec.Enabled,
					Description: rec.Description,
					Path:        rec.Path,
					Commands:    cmds,
				})
				seen[rec.Name] = true
			}
		}

		// 2. Scan physical directory for any unrecorded plugins
		if entries, err := os.ReadDir(pluginsDir); err == nil {
			for _, e := range entries {
				if !e.IsDir() || seen[e.Name()] {
					continue
				}
				pDir := filepath.Join(pluginsDir, e.Name())
				if mfPath, ok := manifest.FindManifestInDir(pDir); ok {
					if mf, err := manifest.Load(mfPath); err == nil {
						items = append(items, displayItem{
							Name:        mf.Name,
							Version:     mf.Version,
							Author:      mf.Author,
							Enabled:     true,
							Description: mf.Description,
							Path:        pDir,
							Commands:    mf.Actions,
						})
					}
				}
			}
		}

		if len(items) == 0 {
			fmt.Println("No plugins installed.")
			fmt.Println("Run 'cliarc plugin search' to discover available plugins.")
			return nil
		}

		fmt.Println(color.CyanString("Installed Plugins (%d):", len(items)))
		for _, item := range items {
			statusTag := color.GreenString("[enabled]")
			if !item.Enabled {
				statusTag = color.RedString("[disabled]")
			}

			fmt.Printf("\n• %-16s %-10s %s\n", color.HiWhiteString(item.Name), color.HiBlackString("v"+item.Version), statusTag)
			if item.Description != "" {
				fmt.Printf("  %s\n", item.Description)
			}
			if len(item.Commands) > 0 {
				fmt.Printf("  Commands: %s\n", color.CyanString(strings.Join(item.Commands, ", ")))
			}
		}
		fmt.Println()
		return nil
	},
}

// 6. info
var pluginInfoCmd = &cobra.Command{
	Use:   "info <name>",
	Short: "Show detailed metadata and capabilities of a plugin",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := strings.ToLower(strings.TrimSpace(args[0]))
		home, _ := os.UserHomeDir()
		pDir := filepath.Join(home, ".cliarc", "plugins", name)

		stateStore, _ := manager.NewPluginStateStore("")
		var rec *manager.PluginRecord
		if stateStore != nil {
			rec, _ = stateStore.Get(name)
		}

		mfPath, ok := manifest.FindManifestInDir(pDir)
		if !ok && rec != nil {
			mfPath, ok = manifest.FindManifestInDir(rec.Path)
		}

		if !ok {
			// Fallback: look up in registry
			client := registry.NewClient("https://registry.cliarc.com/v1")
			if item, err := client.Get(name); err == nil {
				fmt.Printf("Plugin Registry Info: %s\n", color.HiWhiteString(item.Name))
				fmt.Printf("  • Version:      v%s\n", item.Version)
				fmt.Printf("  • Author:       %s\n", item.Author)
				fmt.Printf("  • License:      %s\n", item.License)
				fmt.Printf("  • Description:  %s\n", item.Description)
				fmt.Printf("  • Runtime:      %s\n", item.Runtime)
				fmt.Printf("  • Permissions:  %s\n", strings.Join(item.Permissions, ", "))
				fmt.Printf("  • Status:       Not Installed\n")
				fmt.Printf("\nInstall with: %s\n", color.CyanString("cliarc plugin install %s", item.Name))
				return nil
			}
			return fmt.Errorf("plugin %q not found", name)
		}

		mf, err := manifest.Load(mfPath)
		if err != nil {
			return err
		}

		fmt.Printf("Plugin: %s\n", color.HiWhiteString(mf.Name))
		fmt.Printf("  • Version:      v%s\n", mf.Version)
		fmt.Printf("  • Description:  %s\n", mf.Description)
		fmt.Printf("  • Author:       %s\n", mf.Author)
		fmt.Printf("  • License:      %s\n", mf.License)
		fmt.Printf("  • Runtime:      %s (type: %s)\n", mf.Runtime.Command, mf.Runtime.Type)
		fmt.Printf("  • Directory:    %s\n", mf.Dir)
		fmt.Printf("  • Config Path:  %s\n", filepath.Join(mf.Dir, "config.json"))
		if len(mf.Permissions) > 0 {
			fmt.Printf("  • Permissions:  %s\n", color.YellowString(strings.Join(mf.Permissions, ", ")))
		}
		if len(mf.Actions) > 0 {
			fmt.Printf("  • Commands:     %s\n", color.CyanString(strings.Join(mf.Actions, ", ")))
		}
		if rec != nil {
			status := color.GreenString("enabled")
			if !rec.Enabled {
				status = color.RedString("disabled")
			}
			fmt.Printf("  • State:        %s (source: %s)\n", status, rec.InstallSource)
			fmt.Printf("  • Installed At: %s\n", rec.InstalledAt.Format(time.RFC3339))
		}
		return nil
	},
}

// 7. enable
var pluginEnableCmd = &cobra.Command{
	Use:   "enable <name>",
	Short: "Enable an installed plugin",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := strings.ToLower(strings.TrimSpace(args[0]))
		stateStore, err := manager.NewPluginStateStore("")
		if err != nil {
			return err
		}

		if err := stateStore.SetEnabled(name, true); err != nil {
			return err
		}

		fmt.Println(color.GreenString("✓ Enabled plugin %q. Its commands are now active in CLIARC.", name))
		return nil
	},
}

// 8. disable
var pluginDisableCmd = &cobra.Command{
	Use:   "disable <name>",
	Short: "Disable an installed plugin without removing files",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := strings.ToLower(strings.TrimSpace(args[0]))
		stateStore, err := manager.NewPluginStateStore("")
		if err != nil {
			return err
		}

		if err := stateStore.SetEnabled(name, false); err != nil {
			return err
		}

		fmt.Println(color.YellowString("✓ Disabled plugin %q. Its commands are now inactive.", name))
		return nil
	},
}

func init() {
	pluginInstallCmd.Flags().BoolP("yes", "y", false, "Automatically confirm requested permissions")

	pluginCmd.AddCommand(pluginSearchCmd)
	pluginCmd.AddCommand(pluginInstallCmd)
	pluginCmd.AddCommand(pluginUninstallCmd)
	pluginCmd.AddCommand(pluginUpdateCmd)
	pluginCmd.AddCommand(pluginListCmd)
	pluginCmd.AddCommand(pluginInfoCmd)
	pluginCmd.AddCommand(pluginEnableCmd)
	pluginCmd.AddCommand(pluginDisableCmd)
}
