package packaging

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	goRuntime "runtime"
	"sync"
	"time"

	"github.com/cliarc/cliarc/internal/manifest"
)

// TargetPlatform represents an OS and Architecture pair.
type TargetPlatform struct {
	OS   string
	Arch string
}

// SupportedPlatforms lists standard targets supported by CLIARC compiler matrix.
var SupportedPlatforms = []TargetPlatform{
	{OS: "windows", Arch: "amd64"},
	{OS: "windows", Arch: "arm64"},
	{OS: "linux", Arch: "amd64"},
	{OS: "linux", Arch: "arm64"},
	{OS: "darwin", Arch: "arm64"},
	{OS: "darwin", Arch: "amd64"},
	{OS: "android", Arch: "arm64"},
	{OS: "android", Arch: "amd64"},
}

// BuildOptions contains configuration for building native plugins.
type BuildOptions struct {
	SourceDir  string
	OutputDir  string
	TargetOS   string
	TargetArch string
	BuildAll   bool
	Verbose    bool
}

// BuiltBinary metadata for an individual compiled artifact.
type BuiltBinary struct {
	OS         string
	Arch       string
	BinaryPath string
	Size       int64
}

// BuildResult contains metadata about the compilation run.
type BuildResult struct {
	PluginName string
	Language   string
	Binaries   []BuiltBinary
	BuildTime  time.Duration
}

// DetectLanguage inspects source directory to determine native plugin language.
func DetectLanguage(dir string) string {
	if _, err := os.Stat(filepath.Join(dir, "Cargo.toml")); err == nil {
		return "rust"
	}
	if _, err := os.Stat(filepath.Join(dir, "src", "main.go")); err == nil {
		return "go"
	}
	if _, err := os.Stat(filepath.Join(dir, "main.go")); err == nil {
		return "go"
	}
	if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
		return "go"
	}
	if _, err := os.Stat(filepath.Join(dir, "package.json")); err == nil {
		return "node"
	}
	if _, err := os.Stat(filepath.Join(dir, "requirements.txt")); err == nil || filepath.Ext(filepath.Join(dir, "main.py")) == ".py" {
		if _, err := os.Stat(filepath.Join(dir, "main.py")); err == nil {
			return "python"
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "src", "main.py")); err == nil {
		return "python"
	}
	if _, err := os.Stat(filepath.Join(dir, "src", "main.c")); err == nil {
		return "c"
	}
	if _, err := os.Stat(filepath.Join(dir, "main.c")); err == nil {
		return "c"
	}
	if _, err := os.Stat(filepath.Join(dir, "Makefile")); err == nil {
		return "c"
	}
	return "unknown"
}

// Build compiles a native plugin (Go, Rust, or C) into its bin/ directory.
func Build(opts BuildOptions) (*BuildResult, error) {
	startTime := time.Now()

	srcDir := opts.SourceDir
	if srcDir == "" {
		srcDir = "."
	}
	absSrcDir, err := filepath.Abs(srcDir)
	if err != nil {
		absSrcDir = srcDir
	}

	mf, err := manifest.Load(absSrcDir)
	if err != nil {
		return nil, fmt.Errorf("load plugin manifest: %w", err)
	}

	outDir := opts.OutputDir
	if outDir == "" {
		outDir = filepath.Join(absSrcDir, "bin")
	}
	absOutDir, err := filepath.Abs(outDir)
	if err != nil {
		absOutDir = outDir
	}
	if err := os.MkdirAll(absOutDir, 0755); err != nil {
		return nil, fmt.Errorf("create output bin dir: %w", err)
	}

	lang := DetectLanguage(absSrcDir)
	if lang == "unknown" {
		return nil, fmt.Errorf("unable to detect native language in %s (expected Go, Rust, or C)", absSrcDir)
	}

	var targets []TargetPlatform
	if opts.BuildAll {
		targets = SupportedPlatforms
	} else {
		targetOS := opts.TargetOS
		if targetOS == "" {
			targetOS = goRuntime.GOOS
		}
		targetArch := opts.TargetArch
		if targetArch == "" {
			targetArch = goRuntime.GOARCH
		}
		targets = []TargetPlatform{{OS: targetOS, Arch: targetArch}}
	}

	var (
		mu            sync.Mutex
		wg            sync.WaitGroup
		builtBinaries []BuiltBinary
		firstErr      error
	)

	for _, t := range targets {
		target := t
		wg.Add(1)
		go func() {
			defer wg.Done()

			binFileName := fmt.Sprintf("cliarc-%s-%s-%s", mf.Name, target.OS, target.Arch)
			if target.OS == "windows" {
				binFileName += ".exe"
			}
			targetBinPath := filepath.Join(absOutDir, binFileName)

			var buildErr error
			switch lang {
			case "go":
				buildErr = buildGoTarget(absSrcDir, targetBinPath, target.OS, target.Arch)
			case "rust":
				buildErr = buildRustTarget(absSrcDir, mf.Name, targetBinPath, target.OS, target.Arch)
			case "c":
				buildErr = buildCTarget(absSrcDir, targetBinPath, target.OS)
			}

			if buildErr != nil {
				mu.Lock()
				if firstErr == nil {
					firstErr = buildErr
				}
				mu.Unlock()
				return
			}

			if stat, err := os.Stat(targetBinPath); err == nil {
				if target.OS != "windows" {
					_ = os.Chmod(targetBinPath, 0755)
				}
				mu.Lock()
				builtBinaries = append(builtBinaries, BuiltBinary{
					OS:         target.OS,
					Arch:       target.Arch,
					BinaryPath: targetBinPath,
					Size:       stat.Size(),
				})

				// If this matches current host OS/Arch, also create standard bin/cliarc-<name>[.exe]
				if target.OS == goRuntime.GOOS && target.Arch == goRuntime.GOARCH {
					hostBinName := "cliarc-" + mf.Name
					if goRuntime.GOOS == "windows" {
						hostBinName += ".exe"
					}
					hostBinPath := filepath.Join(absOutDir, hostBinName)
					_ = CopyFile(targetBinPath, hostBinPath)
					if goRuntime.GOOS != "windows" {
						_ = os.Chmod(hostBinPath, 0755)
					}
				}
				mu.Unlock()
			}
		}()
	}

	wg.Wait()

	if len(builtBinaries) == 0 && firstErr != nil {
		return nil, firstErr
	}

	if len(builtBinaries) == 0 {
		return nil, fmt.Errorf("no binaries were successfully built")
	}

	return &BuildResult{
		PluginName: mf.Name,
		Language:   lang,
		Binaries:   builtBinaries,
		BuildTime:  time.Since(startTime),
	}, nil
}

func buildGoTarget(srcDir, destBinary, targetOS, targetArch string) error {
	// Find entrypoint: src/main.go, ./src, or .
	entrypoint := "."
	if _, err := os.Stat(filepath.Join(srcDir, "src", "main.go")); err == nil {
		entrypoint = "./src"
	}

	cmd := exec.Command("go", "build", "-trimpath", "-ldflags=-s -w", "-o", destBinary, entrypoint)
	cmd.Dir = srcDir
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("GOOS=%s", targetOS),
		fmt.Sprintf("GOARCH=%s", targetArch),
		"CGO_ENABLED=0",
	)

	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("go build (%s/%s) failed: %v\nOutput:\n%s", targetOS, targetArch, err, string(out))
	}
	return nil
}

func buildRustTarget(srcDir, pluginName, destBinary, targetOS, targetArch string) error {
	cmd := exec.Command("cargo", "build", "--release")
	cmd.Dir = srcDir
	cmd.Env = os.Environ()

	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("cargo build failed: %v\nOutput:\n%s", err, string(out))
	}

	builtName := "cliarc-" + pluginName
	if targetOS == "windows" {
		builtName += ".exe"
	}
	builtPath := filepath.Join(srcDir, "target", "release", builtName)
	if _, err := os.Stat(builtPath); os.IsNotExist(err) {
		builtPath = filepath.Join(srcDir, "target", "release", pluginName)
		if targetOS == "windows" {
			builtPath += ".exe"
		}
	}

	if _, err := os.Stat(builtPath); err != nil {
		return fmt.Errorf("cargo artifact not found at %s: %w", builtPath, err)
	}

	return CopyFile(builtPath, destBinary)
}

func buildCTarget(srcDir, destBinary, targetOS string) error {
	makefilePath := filepath.Join(srcDir, "Makefile")
	if _, err := os.Stat(makefilePath); err == nil {
		cmd := exec.Command("make")
		cmd.Dir = srcDir
		out, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("make failed: %v\nOutput:\n%s", err, string(out))
		}
		return nil
	}

	mainC := filepath.Join(srcDir, "src", "main.c")
	if _, err := os.Stat(mainC); os.IsNotExist(err) {
		mainC = filepath.Join(srcDir, "main.c")
	}

	cmd := exec.Command("gcc", "-O2", "-Wall", "-o", destBinary, mainC)
	cmd.Dir = srcDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("gcc compilation failed: %v\nOutput:\n%s", err, string(out))
	}
	return nil
}
