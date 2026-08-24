package packaging

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/cliarc/cliarc/internal/manifest"
)

// CheckStatus represents the status of a single diagnostic check.
type CheckStatus string

const (
	StatusPass CheckStatus = "pass"
	StatusWarn CheckStatus = "warn"
	StatusFail CheckStatus = "fail"
)

// DiagnosticItem represents an individual diagnostic result.
type DiagnosticItem struct {
	Name    string
	Status  CheckStatus
	Message string
	Detail  string
}

// PluginDoctorReport contains all diagnostics for a plugin.
type PluginDoctorReport struct {
	PluginName    string
	PluginVersion string
	Directory     string
	Passed        bool
	PassCount     int
	WarnCount     int
	FailCount     int
	Items         []DiagnosticItem
}

// ValidatePlugin runs comprehensive validation on a plugin directory.
func ValidatePlugin(sourceDir string) (*PluginDoctorReport, error) {
	if sourceDir == "" {
		sourceDir = "."
	}
	absDir, err := filepath.Abs(sourceDir)
	if err != nil {
		absDir = sourceDir
	}

	report := &PluginDoctorReport{
		Directory: absDir,
		Passed:    true,
	}

	addItem := func(name string, status CheckStatus, message, detail string) {
		report.Items = append(report.Items, DiagnosticItem{
			Name:    name,
			Status:  status,
			Message: message,
			Detail:  detail,
		})
		switch status {
		case StatusPass:
			report.PassCount++
		case StatusWarn:
			report.WarnCount++
		case StatusFail:
			report.FailCount++
			report.Passed = false
		}
	}

	// 1. Manifest Existence
	manifestPath, ok := manifest.FindManifestInDir(absDir)
	if !ok {
		addItem("Manifest File", StatusFail,
			fmt.Sprintf("Missing %s (or manifest.yaml)", manifest.ManifestFileName),
			"A plugin manifest is required for CLIARC to discover and run this plugin.")
		return report, nil
	}

	manifestRel, _ := filepath.Rel(absDir, manifestPath)
	baseName := filepath.Base(manifestPath)
	if baseName == "plugin.yaml" || baseName == "plugin.yml" || baseName == "cliarc.plugin.yaml" || baseName == "cliarc.plugin.yml" || baseName == "plugin.json" {
		addItem("Manifest File", StatusPass,
			fmt.Sprintf("Found %s", manifestRel),
			manifestPath)
	} else {
		addItem("Manifest File", StatusPass,
			fmt.Sprintf("Found %s", manifestRel),
			manifestPath)
	}

	// 2. Manifest Schema & Fields
	mf, err := manifest.Load(manifestPath)
	if err != nil {
		addItem("Manifest Syntax", StatusFail,
			fmt.Sprintf("Invalid manifest YAML: %v", err),
			"Fix the YAML syntax or required fields in "+manifestRel)
		return report, nil
	}
	addItem("Manifest Syntax", StatusPass, "Manifest YAML is valid", "")
	report.PluginName = mf.Name
	report.PluginVersion = mf.Version

	// Check name
	if mf.Name == "" {
		addItem("Plugin Name", StatusFail, "Plugin name is empty", "Set 'name' in "+manifestRel)
	} else {
		addItem("Plugin Name", StatusPass, fmt.Sprintf("Plugin name: %q", mf.Name), "")
	}

	// Check version
	if mf.Version == "" {
		addItem("Plugin Version", StatusFail, "Plugin version is empty", "Set 'version' (e.g. 0.1.0) in "+manifestRel)
	} else {
		addItem("Plugin Version", StatusPass, fmt.Sprintf("Version: v%s", mf.Version), "")
	}

	// Check metadata fields
	if mf.Description != "" {
		addItem("Description", StatusPass, "Description provided", mf.Description)
	} else {
		addItem("Description", StatusWarn, "No description field in manifest", "Add 'description' to explain what this plugin does.")
	}

	if mf.Author != "" {
		addItem("Author", StatusPass, fmt.Sprintf("Author: %s", mf.Author), "")
	} else {
		addItem("Author", StatusWarn, "No author field in manifest", "Add 'author' to identify the plugin creator.")
	}

	if mf.License != "" {
		addItem("License", StatusPass, fmt.Sprintf("License: %s", mf.License), "")
	} else {
		addItem("License", StatusWarn, "No license specified in manifest", "Add 'license' (e.g. MIT or Apache-2.0).")
	}

	// Check commands/actions
	cmds := mf.Commands
	if len(cmds) == 0 {
		cmds = mf.Actions
	}
	if len(cmds) == 0 {
		addItem("Commands/Actions", StatusFail, "No commands or actions defined in manifest", "Define at least one action or command.")
	} else {
		addItem("Commands/Actions", StatusPass, fmt.Sprintf("%d command(s) defined (%s)", len(cmds), strings.Join(cmds, ", ")), "")
	}

	// 3. Native Source Files & Language
	lang := DetectLanguage(absDir)
	if lang == "unknown" {
		addItem("Native Source", StatusFail, "Unable to detect native source (Go, Rust, or C)", "Plugins must have Go (src/main.go), Rust (src/main.rs), or C (src/main.c) source code.")
	} else {
		addItem("Native Source", StatusPass, fmt.Sprintf("Detected %s source implementation", strings.ToUpper(lang)), "")
	}

	// Check src/ directory
	srcPath := filepath.Join(absDir, "src")
	if stat, err := os.Stat(srcPath); err == nil && stat.IsDir() {
		addItem("Source Directory", StatusPass, "src/ directory exists", "")
	} else {
		addItem("Source Directory", StatusWarn, "No src/ directory found (source files at root)", "Best practice is to keep source code inside src/ directory.")
	}

	// Check tests/ directory
	testsPath := filepath.Join(absDir, "tests")
	if stat, err := os.Stat(testsPath); err == nil && stat.IsDir() {
		addItem("Test Suite", StatusPass, "tests/ directory exists", "")
	} else {
		addItem("Test Suite", StatusWarn, "No tests/ directory found", "Consider adding unit tests under tests/ directory.")
	}

	// 4. Standard Project Documentation & Config Files
	checkFile := func(filename string, required bool, desc string) {
		p := filepath.Join(absDir, filename)
		if stat, err := os.Stat(p); err == nil && !stat.IsDir() {
			addItem(filename, StatusPass, fmt.Sprintf("%s exists", filename), "")
		} else if required {
			addItem(filename, StatusFail, fmt.Sprintf("Missing %s", filename), desc)
		} else {
			addItem(filename, StatusWarn, fmt.Sprintf("Missing %s", filename), desc)
		}
	}

	checkFile("README.md", false, "Recommended: Add README.md with usage documentation.")
	checkFile("LICENSE", false, "Recommended: Add LICENSE file.")
	checkFile("CHANGELOG.md", false, "Recommended: Add CHANGELOG.md to track plugin versions.")
	checkFile(".gitignore", false, "Recommended: Add .gitignore to exclude build artifacts.")
	checkFile(".cliarcignore", false, "Recommended: Add .cliarcignore to filter distribution files.")

	// Check CI workflow
	workflowPath := filepath.Join(absDir, ".github", "workflows", "release.yml")
	if stat, err := os.Stat(workflowPath); err == nil && !stat.IsDir() {
		addItem(".github/workflows/release.yml", StatusPass, "GitHub release workflow exists", "")
	} else {
		addItem(".github/workflows/release.yml", StatusWarn, "Missing .github/workflows/release.yml", "Recommended for automated plugin builds and releases.")
	}

	// 5. Binary Executable in bin/
	binDir := filepath.Join(absDir, "bin")
	if entries, err := os.ReadDir(binDir); err == nil && len(entries) > 0 {
		var binNames []string
		for _, e := range entries {
			if !e.IsDir() {
				binNames = append(binNames, e.Name())
			}
		}
		if len(binNames) > 0 {
			addItem("Compiled Binaries", StatusPass, fmt.Sprintf("Found %d binary/binaries in bin/ (%s)", len(binNames), strings.Join(binNames, ", ")), "")
		} else {
			addItem("Compiled Binaries", StatusWarn, "bin/ directory is empty", "Run 'cliarc plugin build .' to compile native binary.")
		}
	} else {
		addItem("Compiled Binaries", StatusWarn, "bin/ directory not found or empty", "Run 'cliarc plugin build .' to compile native binary.")
	}

	return report, nil
}
