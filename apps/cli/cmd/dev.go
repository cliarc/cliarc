package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/cliarc/cliarc/core/events"
	"github.com/cliarc/cliarc/core/permissions"
	manager "github.com/cliarc/cliarc/core/plugin-manager"
	"github.com/cliarc/cliarc/core/registry"
	"github.com/cliarc/cliarc/internal/manifest"
	"github.com/cliarc/cliarc/internal/packaging"
	"github.com/cliarc/cliarc/internal/scaffold"
	"github.com/cliarc/cliarc/internal/security"
)

var devCmd = &cobra.Command{
	Use:   "dev",
	Short: "Plugin development, building, testing, packaging, and publishing",
	Long: `Dedicated toolchain for plugin authors to scaffold, test, build, package, and publish CLIARC plugins.

Available Commands:
  init       Scaffold a new native or scripted plugin project
  run        Execute plugin locally in development mode
  test       Run plugin unit and integration tests in tests/
  build      Compile native executable binary/binaries
  package    Create a distributable .tar.gz package with SHA256 checksum
  publish    Validate and publish plugin package to registry
  link       Link current plugin directory into ~/.cliarc/plugins for live testing
  unlink     Remove linked development plugin
  validate   Deep structural, manifest, and security validation (doctor)
  clean      Clean build artifacts, temporary files, and logs`,
}

// 1. init
var devInitCmd = &cobra.Command{
	Use:   "init <name>",
	Short: "Scaffold a new plugin project template",
	Long: `Create a plugin project with plugin.yaml, src/, tests/, README.md, LICENSE, CHANGELOG.md, and CI workflows.
Supported Languages: Go, Rust, Python, Node.js, C/C++`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := strings.TrimSpace(args[0])
		langFlag, _ := cmd.Flags().GetString("lang")
		dirFlag, _ := cmd.Flags().GetString("dir")
		descFlag, _ := cmd.Flags().GetString("description")
		authorFlag, _ := cmd.Flags().GetString("author")

		if langFlag == "" {
			langFlag = "go"
		}
		targetDir := dirFlag
		if targetDir == "" {
			targetDir = filepath.Join(".", name)
		}

		normLang := scaffold.NormalizeLanguage(langFlag)
		opts := scaffold.ScaffoldOptions{
			Name:        name,
			Language:    normLang,
			OutputDir:   targetDir,
			Description: descFlag,
			Author:      authorFlag,
		}

		createdFiles, err := scaffold.ScaffoldPlugin(opts)
		if err != nil {
			return fmt.Errorf("failed to create plugin: %w", err)
		}

		fmt.Println(color.GreenString("✓ Created %s plugin %q:", strings.ToUpper(normLang), name))
		for _, f := range createdFiles {
			relPath, err := filepath.Rel(targetDir, f)
			if err != nil {
				relPath = f
			}
			fmt.Printf("  • %-28s %s\n", color.CyanString(relPath), color.HiBlackString("(%s)", f))
		}

		fmt.Println(color.HiCyanString("\nNext steps for development:"))
		fmt.Printf("  1. cd %s\n", targetDir)
		fmt.Println("  2. cliarc dev validate .   # Validate project structure & manifest")
		fmt.Println("  3. cliarc dev build .      # Compile binary executable")
		fmt.Println("  4. cliarc dev test .       # Run test suite")
		fmt.Println("  5. cliarc dev link .       # Link to local CLIARC for testing")
		return nil
	},
}

// 2. run
var devRunCmd = &cobra.Command{
	Use:   "run [path] [action]",
	Short: "Execute plugin locally in development mode",
	Args:  cobra.MaximumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		targetDir := "."
		action := ""
		if len(args) == 1 {
			if _, err := os.Stat(args[0]); err == nil {
				targetDir = args[0]
			} else {
				action = args[0]
			}
		} else if len(args) >= 2 {
			targetDir = args[0]
			action = args[1]
		}

		mfPath, ok := manifest.FindManifestInDir(targetDir)
		if !ok {
			return fmt.Errorf("no plugin.yaml or manifest found in %s", targetDir)
		}

		mf, err := manifest.Load(mfPath)
		if err != nil {
			return fmt.Errorf("invalid manifest: %w", err)
		}

		if action == "" {
			if len(mf.Actions) > 0 {
				action = mf.Actions[0]
			} else {
				action = mf.Name + ".run"
			}
		}

		fmt.Println(color.CyanString("Running development action %q on plugin %q...", action, mf.Name))

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		reg := registry.New()
		validator := permissions.NewValidator()
		bus := events.NewBus()
		mgr := manager.NewManager(reg, validator, bus, mf.Dir)

		if err := mgr.Load(mf); err != nil {
			return fmt.Errorf("load failed: %w", err)
		}

		if err := mgr.Start(ctx, mf.Name); err != nil {
			return fmt.Errorf("start failed: %w", err)
		}
		defer mgr.Stop(context.Background(), mf.Name)

		payloadBytes, _ := json.Marshal(map[string]interface{}{"mode": "dev"})
		resp, err := mgr.Execute(ctx, mf.Name, action, payloadBytes)
		if err != nil {
			return fmt.Errorf("execution error: %w", err)
		}

		if len(resp.Result) > 0 {
			fmt.Println(string(resp.Result))
		} else {
			fmt.Println(color.GreenString("✓ Action completed successfully"))
		}
		return nil
	},
}

// 3. test
var devTestCmd = &cobra.Command{
	Use:   "test [path]",
	Short: "Run plugin tests in tests/",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		targetDir := "."
		if len(args) > 0 {
			targetDir = args[0]
		}

		fmt.Printf("Running plugin tests in %s...\n", color.CyanString(targetDir))
		res, err := packaging.Test(packaging.TestOptions{
			SourceDir: targetDir,
		})
		if err != nil {
			if res != nil && len(res.Output) > 0 {
				fmt.Fprintln(os.Stderr, string(res.Output))
			}
			return fmt.Errorf("tests failed: %w", err)
		}

		if len(res.Output) > 0 {
			fmt.Println(string(res.Output))
		}
		fmt.Println(color.GreenString("✓ Plugin test suite passed (%s)", res.Duration))
		return nil
	},
}

// 4. build
var devBuildCmd = &cobra.Command{
	Use:   "build [path]",
	Short: "Compile native binary executable(s) into bin/",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		targetDir := "."
		if len(args) > 0 {
			targetDir = args[0]
		}

		allFlag, _ := cmd.Flags().GetBool("all")
		osFlag, _ := cmd.Flags().GetString("os")
		archFlag, _ := cmd.Flags().GetString("arch")

		fmt.Printf("Building plugin in %s...\n", color.CyanString(targetDir))
		res, err := packaging.Build(packaging.BuildOptions{
			SourceDir: targetDir,
			TargetOS:  osFlag,
			TargetArch: archFlag,
			BuildAll:  allFlag,
		})
		if err != nil {
			return fmt.Errorf("build failed: %w", err)
		}

		fmt.Println(color.GreenString("✓ Successfully built %s plugin %q (%d binary/binaries in %s):",
			strings.ToUpper(res.Language), res.PluginName, len(res.Binaries), res.BuildTime))
		for _, b := range res.Binaries {
			fmt.Printf("  • %-16s %s (%s)\n",
				color.CyanString(b.OS+"/"+b.Arch),
				b.BinaryPath,
				color.YellowString("%.1f MB", float64(b.Size)/(1024*1024)))
		}
		return nil
	},
}

// 5. package
var devPackageCmd = &cobra.Command{
	Use:     "package [path]",
	Aliases: []string{"pack"},
	Short:   "Create a distributable .tar.gz package with checksums",
	Args:    cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		targetDir := "."
		if len(args) > 0 {
			targetDir = args[0]
		}

		outFlag, _ := cmd.Flags().GetString("out")

		fmt.Printf("Packaging plugin in %s...\n", color.CyanString(targetDir))
		res, err := packaging.Package(packaging.PackageOptions{
			SourceDir:  targetDir,
			OutputPath: outFlag,
		})
		if err != nil {
			return fmt.Errorf("package failed: %w", err)
		}

		fmt.Println(color.GreenString("✓ Package created successfully:"))
		fmt.Printf("  • Archive:  %s (%s)\n", color.CyanString(res.ArchivePath), color.YellowString("%.2f MB", float64(res.Size)/(1024*1024)))
		fmt.Printf("  • SHA256:   %s\n", color.HiBlackString(res.Checksum))
		fmt.Printf("  • Files:    %d files packaged\n", res.FilesCount)
		return nil
	},
}

// 6. publish
var devPublishCmd = &cobra.Command{
	Use:   "publish [path]",
	Short: "Validate and publish plugin package to registry",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		targetDir := "."
		if len(args) > 0 {
			targetDir = args[0]
		}

		// First validate
		fmt.Println(color.CyanString("1. Validating plugin for publication..."))
		report, err := packaging.ValidatePlugin(targetDir)
		if err != nil || !report.Passed {
			return fmt.Errorf("validation failed with %d error(s). Run 'cliarc dev validate' for details", report.FailCount)
		}

		// Package
		fmt.Println(color.CyanString("2. Building distribution bundle..."))
		pkgRes, err := packaging.Package(packaging.PackageOptions{SourceDir: targetDir})
		if err != nil {
			return fmt.Errorf("failed to package plugin: %w", err)
		}

		hash, _ := security.CalculateSHA256(pkgRes.ArchivePath)

		fmt.Println(color.GreenString("\n✓ Plugin %q (v%s) is ready for registry release!", report.PluginName, report.PluginVersion))
		fmt.Printf("  • Package:  %s\n", pkgRes.ArchivePath)
		fmt.Printf("  • Checksum: %s\n", hash)
		fmt.Printf("  • Registry: https://registry.cliarc.com/v1/publish\n")
		return nil
	},
}

// 7. link
var devLinkCmd = &cobra.Command{
	Use:   "link [path]",
	Short: "Link local plugin directory into ~/.cliarc/plugins for live testing",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		targetDir := "."
		if len(args) > 0 {
			targetDir = args[0]
		}

		home, _ := os.UserHomeDir()
		pluginsDir := filepath.Join(home, ".cliarc", "plugins")

		res, err := packaging.Install(packaging.InstallOptions{
			SourcePath:  targetDir,
			RegistryDir: pluginsDir,
			ForceBuild:  true,
		})
		if err != nil {
			return fmt.Errorf("link failed: %w", err)
		}

		stateStore, _ := manager.NewPluginStateStore("")
		if stateStore != nil {
			mf, _ := manifest.Load(res.InstallDir)
			rec := &manager.PluginRecord{
				Name:          res.PluginName,
				Version:       res.Version,
				Description:   mf.Description,
				Author:        mf.Author,
				Enabled:       true,
				InstallSource: "linked",
				InstalledAt:   time.Now(),
				UpdatedAt:     time.Now(),
				Path:          res.InstallDir,
				ConfigPath:    filepath.Join(res.InstallDir, "config.json"),
			}
			_ = stateStore.Set(rec)
		}

		fmt.Println(color.GreenString("✓ Plugin %q linked successfully to %s", res.PluginName, res.InstallDir))
		fmt.Printf("  Test with: %s\n", color.CyanString("cliarc %s --help", res.PluginName))
		return nil
	},
}

// 8. unlink
var devUnlinkCmd = &cobra.Command{
	Use:   "unlink <name>",
	Short: "Remove a linked development plugin",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := strings.ToLower(strings.TrimSpace(args[0]))
		home, _ := os.UserHomeDir()
		targetDir := filepath.Join(home, ".cliarc", "plugins", name)

		stateStore, _ := manager.NewPluginStateStore("")
		if stat, err := os.Stat(targetDir); err == nil && stat.IsDir() {
			if err := os.RemoveAll(targetDir); err != nil {
				return fmt.Errorf("failed to remove linked plugin %s: %w", targetDir, err)
			}
		}

		if stateStore != nil {
			_ = stateStore.Remove(name)
		}

		fmt.Println(color.GreenString("✓ Linked plugin %q removed successfully", name))
		return nil
	},
}

// 9. validate
var devValidateCmd = &cobra.Command{
	Use:     "validate [path]",
	Aliases: []string{"doctor", "check"},
	Short:   "Deep structural, manifest, and binary validation",
	Args:    cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		targetDir := "."
		if len(args) > 0 {
			targetDir = args[0]
		}

		report, err := packaging.ValidatePlugin(targetDir)
		if err != nil {
			return err
		}

		fmt.Println(color.CyanString("🩺 CLIARC Plugin Validation: %s", report.Directory))
		if report.PluginName != "" {
			fmt.Printf("Plugin: %s\n\n", color.HiWhiteString(report.PluginName))
		}

		for _, check := range report.Items {
			icon := color.GreenString("✓")
			if check.Status == packaging.StatusFail {
				icon = color.RedString("✗")
			} else if check.Status == packaging.StatusWarn {
				icon = color.YellowString("!")
			}

			fmt.Printf("  %s %-30s %s\n", icon, check.Name, check.Message)
			if check.Detail != "" && check.Status != packaging.StatusPass {
				fmt.Printf("    → %s\n", color.HiBlackString(check.Detail))
			}
		}

		fmt.Println()
		if report.Passed {
			fmt.Println(color.GreenString("✓ All core plugin validation checks passed (%d passed, %d warning(s))",
				report.PassCount, report.WarnCount))
		} else {
			fmt.Println(color.RedString("✗ Validation failed with %d error(s)", report.FailCount))
			return fmt.Errorf("validation failed")
		}
		return nil
	},
}

// 10. clean
var devCleanCmd = &cobra.Command{
	Use:   "clean [path]",
	Short: "Remove build artifacts, binaries, and temporary files",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		targetDir := "."
		if len(args) > 0 {
			targetDir = args[0]
		}

		res, err := packaging.Clean(targetDir)
		if err != nil {
			return err
		}

		if len(res.RemovedPaths) == 0 {
			fmt.Println(color.GreenString("✓ Plugin directory is already clean (0 files removed)"))
			return nil
		}

		fmt.Println(color.GreenString("✓ Cleaned plugin directory (freed %.2f MB):", float64(res.FreedBytes)/(1024*1024)))
		for _, p := range res.RemovedPaths {
			fmt.Printf("  • Removed %s\n", color.CyanString(p))
		}
		return nil
	},
}

func init() {
	devInitCmd.Flags().StringP("lang", "l", "go", "Plugin implementation language (go, rust, python, node, c)")
	devInitCmd.Flags().StringP("dir", "d", "", "Target output directory")
	devInitCmd.Flags().String("description", "High-performance developer tool", "Short description")
	devInitCmd.Flags().String("author", "CLIARC Community", "Author name")

	devBuildCmd.Flags().BoolP("all", "a", false, "Cross-compile for all supported OS/Arch targets")
	devBuildCmd.Flags().String("os", "", "Target operating system (linux, darwin, windows)")
	devBuildCmd.Flags().String("arch", "", "Target architecture (amd64, arm64)")

	devPackageCmd.Flags().StringP("out", "o", "", "Custom output archive path")

	devCmd.AddCommand(devInitCmd)
	devCmd.AddCommand(devRunCmd)
	devCmd.AddCommand(devTestCmd)
	devCmd.AddCommand(devBuildCmd)
	devCmd.AddCommand(devPackageCmd)
	devCmd.AddCommand(devPublishCmd)
	devCmd.AddCommand(devLinkCmd)
	devCmd.AddCommand(devUnlinkCmd)
	devCmd.AddCommand(devValidateCmd)
	devCmd.AddCommand(devCleanCmd)
}
