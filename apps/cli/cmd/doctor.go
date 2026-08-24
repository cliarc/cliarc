package cmd

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/cliarc/cliarc/core/config"
	"github.com/cliarc/cliarc/internal/packaging"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var doctorCmd = &cobra.Command{
	Use:   "doctor [plugin]",
	Short: "Check system environment, compilers, networking, and CLIARC configuration",
	Long: `Run complete diagnostic checks across system environment or specific plugins:
  • Development toolchain & compilers (Go, C/C++, Rust)
  • CLIARC directories & permissions (~/.cliarc, plugins, config.json)
  • Loopback TCP/gRPC networking connectivity
  • Plugin-specific health and diagnostics (when plugin name is provided)`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) > 0 {
			return runPluginDoctor(args[0])
		}

		fmt.Println(color.CyanString("🩺 CLIARC Environment & System Diagnostic"))
		fmt.Printf("OS: %s | Arch: %s | Host: %s\n\n", runtime.GOOS, runtime.GOARCH, getHostname())

		passCount := 0
		warnCount := 0
		failCount := 0

		reportCheck := func(name string, status string, msg string, hint string) {
			switch status {
			case "pass":
				passCount++
				fmt.Printf("  %s %-30s %s\n", color.GreenString("✓"), name, color.HiBlackString(msg))
			case "warn":
				warnCount++
				fmt.Printf("  %s %-30s %s\n", color.YellowString("!"), name, color.YellowString(msg))
				if hint != "" {
					fmt.Printf("    %s\n", color.HiBlackString("→ %s", hint))
				}
			case "fail":
				failCount++
				fmt.Printf("  %s %-30s %s\n", color.RedString("✗"), name, color.RedString(msg))
				if hint != "" {
					fmt.Printf("    %s\n", color.HiBlackString("→ %s", hint))
				}
			}
		}

		// 1. Go Compiler
		if goPath, err := exec.LookPath("go"); err == nil {
			out, err := exec.Command(goPath, "version").Output()
			if err == nil {
				reportCheck("Go Toolchain", "pass", strings.TrimSpace(string(out)), "")
			} else {
				reportCheck("Go Toolchain", "pass", "Found at "+goPath, "")
			}
		} else {
			reportCheck("Go Toolchain", "warn", "go compiler not found on PATH", "Install Go 1.22+ from https://golang.org")
		}

		// 2. C/C++ Compiler
		var cCompiler string
		for _, cc := range []string{"gcc", "clang", "cl"} {
			if p, err := exec.LookPath(cc); err == nil {
				cCompiler = p
				break
			}
		}
		if cCompiler != "" {
			reportCheck("C/C++ Compiler", "pass", "Found at "+cCompiler, "")
		} else {
			reportCheck("C/C++ Compiler", "warn", "gcc/clang not found on PATH", "Required only if building native C/C++ plugins.")
		}

		// 3. Rust Toolchain
		if cargoPath, err := exec.LookPath("cargo"); err == nil {
			out, err := exec.Command(cargoPath, "--version").Output()
			if err == nil {
				reportCheck("Rust Toolchain", "pass", strings.TrimSpace(string(out)), "")
			} else {
				reportCheck("Rust Toolchain", "pass", "Found at "+cargoPath, "")
			}
		} else {
			reportCheck("Rust Toolchain", "warn", "cargo not found on PATH", "Optional: Install Rust from https://rustup.rs if building Rust plugins.")
		}

		// 4. CLIARC Base Directory (~/.cliarc)
		home, err := os.UserHomeDir()
		if err != nil {
			reportCheck("CLIARC Home (~/.cliarc)", "fail", "Could not resolve user home directory", "")
		} else {
			cliarcHome := filepath.Join(home, ".cliarc")
			if stat, err := os.Stat(cliarcHome); err == nil && stat.IsDir() {
				reportCheck("CLIARC Home (~/.cliarc)", "pass", cliarcHome, "")
			} else {
				_ = os.MkdirAll(cliarcHome, 0750)
				reportCheck("CLIARC Home (~/.cliarc)", "pass", "Created "+cliarcHome, "")
			}
		}

		// 5. Config File
		cfg := config.Default()
		configPath := filepath.Join(home, ".cliarc", "config.json")
		if _, err := os.Stat(configPath); err == nil {
			reportCheck("Config File (config.json)", "pass", configPath, "")
		} else {
			_ = cfg.Save(configPath)
			reportCheck("Config File (config.json)", "pass", "Initialized "+configPath, "")
		}

		// 6. Plugins Directory
		pluginsDir := filepath.Join(home, ".cliarc", "plugins")
		_ = os.MkdirAll(pluginsDir, 0755)
		entries, err := os.ReadDir(pluginsDir)
		if err == nil {
			pCount := 0
			for _, e := range entries {
				if e.IsDir() {
					pCount++
				}
			}
			reportCheck("Plugins Dir (~/.cliarc/plugins)", "pass", fmt.Sprintf("%s (%d plugin(s) installed)", pluginsDir, pCount), "")
		} else {
			reportCheck("Plugins Dir (~/.cliarc/plugins)", "fail", "Could not read plugins directory", "")
		}

		// 7. Loopback Networking / Socket Binding (for plugin gRPC)
		lis, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			reportCheck("Loopback Networking", "fail", "Failed to bind loopback TCP socket: "+err.Error(), "CLIARC plugins communicate via loopback gRPC sockets (127.0.0.1).")
		} else {
			addr := lis.Addr().String()
			_ = lis.Close()
			reportCheck("Loopback Networking", "pass", fmt.Sprintf("TCP loopback socket test passed (%s)", addr), "")
		}

		// 8. SSH Client
		if sshPath, err := exec.LookPath("ssh"); err == nil {
			reportCheck("SSH Client", "pass", "Found at "+sshPath, "")
		} else {
			reportCheck("SSH Client", "warn", "OpenSSH client (ssh) not found on PATH", "Recommended for remote SSH operations.")
		}

		fmt.Println()
		if failCount == 0 {
			fmt.Println(color.GreenString("✓ CLIARC environment is healthy! (%d checks passed, %d warning(s))", passCount, warnCount))
		} else {
			fmt.Println(color.RedString("✗ Environment issues detected: %d check(s) failed, %d warning(s)", failCount, warnCount))
		}

		return nil
	},
}

func runPluginDoctor(pluginName string) error {
	home, _ := os.UserHomeDir()
	candidatePaths := []string{
		filepath.Join(home, ".cliarc", "plugins", pluginName),
		filepath.Join("..", "plugins", pluginName),
		filepath.Join("plugins", pluginName),
		pluginName,
	}

	var targetPath string
	for _, p := range candidatePaths {
		if stat, err := os.Stat(p); err == nil && stat.IsDir() {
			targetPath = p
			break
		}
	}

	if targetPath == "" {
		return fmt.Errorf("plugin %q not found in ~/.cliarc/plugins or local paths", pluginName)
	}

	report, err := packaging.ValidatePlugin(targetPath)
	if err != nil {
		return err
	}

	fmt.Println(color.CyanString("🩺 CLIARC Plugin Diagnostic: %s", targetPath))
	fmt.Printf("Plugin: %s (v%s)\n\n", color.HiWhiteString(report.PluginName), report.PluginVersion)

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
		fmt.Println(color.GreenString("✓ Plugin %q is healthy and valid! (%d checks passed, %d warning(s))",
			report.PluginName, report.PassCount, report.WarnCount))
	} else {
		fmt.Println(color.RedString("✗ Plugin issues detected: %d check(s) failed", report.FailCount))
	}
	return nil
}

func getHostname() string {
	h, err := os.Hostname()
	if err != nil {
		return "localhost"
	}
	return h
}
