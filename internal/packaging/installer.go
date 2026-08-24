package packaging

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	goRuntime "runtime"
	"strings"

	"github.com/cliarc/cliarc/internal/manifest"
)

// InstallOptions configures plugin installation or linking.
type InstallOptions struct {
	SourcePath  string
	RegistryDir string
	ForceBuild  bool
}

// InstallResult metadata for an installed plugin.
type InstallResult struct {
	PluginName     string
	Version        string
	InstallDir     string
	BinaryPath     string
	InstalledFiles []string
	Actions        []string
}

// PluginDetails represents comprehensive metadata about an installed plugin.
type PluginDetails struct {
	Name           string
	Version        string
	Description    string
	Author         string
	Path           string
	BinaryPath     string
	BinarySize     int64
	Actions        []string
	Permissions    []string
	InstalledFiles []string
}

// Install links or installs a native plugin package (directory or .tar.gz) into the registry.
func Install(opts InstallOptions) (*InstallResult, error) {
	srcPath := opts.SourcePath
	if srcPath == "" {
		srcPath = "."
	}

	stat, err := os.Stat(srcPath)
	if err != nil {
		return nil, fmt.Errorf("plugin source not found at %s: %w", srcPath, err)
	}

	if stat.IsDir() {
		return installFromDirectory(srcPath, opts.RegistryDir, opts.ForceBuild)
	}

	if strings.HasSuffix(strings.ToLower(srcPath), ".tar.gz") || strings.HasSuffix(strings.ToLower(srcPath), ".tgz") {
		return installFromTarGz(srcPath, opts.RegistryDir)
	}

	return nil, fmt.Errorf("unsupported plugin package format %s (expected directory or .tar.gz archive)", srcPath)
}

// Unlink removes a linked or installed plugin from the registry.
func Unlink(name, registryDir string) error {
	pluginDir := filepath.Join(registryDir, name)
	if _, err := os.Stat(pluginDir); os.IsNotExist(err) {
		return fmt.Errorf("plugin %q is not installed in %s", name, registryDir)
	}
	return os.RemoveAll(pluginDir)
}

// GetInfo retrieves detailed metadata about an installed plugin.
func GetInfo(name, registryDir string) (*PluginDetails, error) {
	pluginDir := filepath.Join(registryDir, name)
	if _, err := os.Stat(pluginDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("plugin %q not found in %s", name, registryDir)
	}

	mf, err := manifest.Load(pluginDir)
	if err != nil {
		return nil, fmt.Errorf("read manifest: %w", err)
	}

	var files []string
	var binPath string
	var binSize int64

	_ = filepath.Walk(pluginDir, func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			rel, _ := filepath.Rel(pluginDir, path)
			files = append(files, rel)
			if strings.HasPrefix(rel, "bin"+string(os.PathSeparator)) || strings.HasPrefix(rel, "bin/") {
				binPath = path
				binSize = info.Size()
			}
		}
		return nil
	})

	return &PluginDetails{
		Name:           mf.Name,
		Version:        mf.Version,
		Description:    mf.Description,
		Author:         mf.Author,
		Path:           pluginDir,
		BinaryPath:     binPath,
		BinarySize:     binSize,
		Actions:        mf.Actions,
		Permissions:    mf.Permissions,
		InstalledFiles: files,
	}, nil
}

func installFromDirectory(srcDir, registryDir string, forceBuild bool) (*InstallResult, error) {
	mf, err := manifest.Load(srcDir)
	if err != nil {
		return nil, fmt.Errorf("read plugin manifest: %w", err)
	}

	manifestSrcPath, _ := manifest.FindManifestInDir(srcDir)
	if manifestSrcPath == "" {
		manifestSrcPath = filepath.Join(srcDir, manifest.ManifestFileName)
	}

	binDir := filepath.Join(srcDir, "bin")
	binEntries, err := os.ReadDir(binDir)
	if err != nil || len(binEntries) == 0 || forceBuild {
		_, buildErr := Build(BuildOptions{SourceDir: srcDir})
		if buildErr != nil {
			return nil, fmt.Errorf("failed to build native plugin: %w", buildErr)
		}
		binEntries, _ = os.ReadDir(binDir)
	}

	destDir := filepath.Join(registryDir, mf.Name)
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return nil, fmt.Errorf("create registry plugin directory: %w", err)
	}

	var installed []string

	// 1. Copy manifest file (cliarc.plugin.yaml or manifest.yaml)
	destManifestName := filepath.Base(manifestSrcPath)
	destManifest := filepath.Join(destDir, destManifestName)
	if err := CopyFile(manifestSrcPath, destManifest); err != nil {
		return nil, fmt.Errorf("install manifest: %w", err)
	}
	installed = append(installed, destManifestName)

	// Also write standard cliarc.plugin.yaml if original was manifest.yaml
	if destManifestName != manifest.ManifestFileName {
		_ = CopyFile(manifestSrcPath, filepath.Join(destDir, manifest.ManifestFileName))
	}

	// 2. Copy README if present
	for _, rName := range []string{"README.md", "README", "readme.md"} {
		rPath := filepath.Join(srcDir, rName)
		if stat, err := os.Stat(rPath); err == nil && !stat.IsDir() {
			destReadme := filepath.Join(destDir, "README.md")
			_ = CopyFile(rPath, destReadme)
			installed = append(installed, "README.md")
			break
		}
	}

	// 3. Copy LICENSE if present
	for _, lName := range []string{"LICENSE", "LICENSE.md", "LICENSE.txt", "license"} {
		lPath := filepath.Join(srcDir, lName)
		if stat, err := os.Stat(lPath); err == nil && !stat.IsDir() {
			destLicense := filepath.Join(destDir, "LICENSE")
			_ = CopyFile(lPath, destLicense)
			installed = append(installed, "LICENSE")
			break
		}
	}

	// 4. Copy CHANGELOG if present
	for _, cName := range []string{"CHANGELOG.md", "CHANGELOG", "changelog.md"} {
		cPath := filepath.Join(srcDir, cName)
		if stat, err := os.Stat(cPath); err == nil && !stat.IsDir() {
			destChangelog := filepath.Join(destDir, "CHANGELOG.md")
			_ = CopyFile(cPath, destChangelog)
			installed = append(installed, "CHANGELOG.md")
			break
		}
	}

	// 4. Select and install ONLY the host OS-specific binary
	destBinDir := filepath.Join(destDir, "bin")
	if err := os.MkdirAll(destBinDir, 0755); err != nil {
		return nil, fmt.Errorf("create registry bin dir: %w", err)
	}

	targetBinary := selectHostBinary(binDir, mf.Name)
	if targetBinary == "" {
		return nil, fmt.Errorf("no matching binary found for %s/%s in %s", goRuntime.GOOS, goRuntime.GOARCH, binDir)
	}

	destBinName := "cliarc-" + mf.Name
	if goRuntime.GOOS == "windows" {
		destBinName += ".exe"
	}
	destBin := filepath.Join(destBinDir, destBinName)
	if err := CopyFile(targetBinary, destBin); err != nil {
		return nil, fmt.Errorf("install binary %s: %w", destBinName, err)
	}
	_ = os.Chmod(destBin, 0755)
	installed = append(installed, "bin/"+destBinName)

	return &InstallResult{
		PluginName:     mf.Name,
		Version:        mf.Version,
		InstallDir:     destDir,
		BinaryPath:     destBin,
		InstalledFiles: installed,
		Actions:        mf.Actions,
	}, nil
}

// selectHostBinary picks the binary that matches current host OS/Arch from candidates.
func selectHostBinary(binDir, pluginName string) string {
	ext := ""
	if goRuntime.GOOS == "windows" {
		ext = ".exe"
	}

	// Priority 1: Exact os-arch match (e.g. cliarc-plugin-windows-amd64.exe)
	exactName := fmt.Sprintf("cliarc-%s-%s-%s%s", pluginName, goRuntime.GOOS, goRuntime.GOARCH, ext)
	exactPath := filepath.Join(binDir, exactName)
	if _, err := os.Stat(exactPath); err == nil {
		return exactPath
	}

	// Priority 2: Generic host name (e.g. cliarc-plugin.exe)
	genericName := fmt.Sprintf("cliarc-%s%s", pluginName, ext)
	genericPath := filepath.Join(binDir, genericName)
	if _, err := os.Stat(genericPath); err == nil {
		return genericPath
	}

	// Priority 3: Any file in bin/ ending with .exe on Windows or executable on Unix
	entries, err := os.ReadDir(binDir)
	if err == nil {
		for _, e := range entries {
			if !e.IsDir() {
				p := filepath.Join(binDir, e.Name())
				if goRuntime.GOOS == "windows" && strings.HasSuffix(strings.ToLower(e.Name()), ".exe") {
					return p
				}
				if goRuntime.GOOS != "windows" {
					return p
				}
			}
		}
	}

	return ""
}

func installFromTarGz(archivePath, registryDir string) (*InstallResult, error) {
	file, err := os.Open(archivePath)
	if err != nil {
		return nil, fmt.Errorf("open archive: %w", err)
	}
	defer file.Close()

	gzr, err := gzip.NewReader(file)
	if err != nil {
		return nil, fmt.Errorf("read gzip archive: %w", err)
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)

	tempDir, err := os.MkdirTemp("", "cliarc-pkg-*")
	if err != nil {
		return nil, fmt.Errorf("create temp unpack dir: %w", err)
	}
	defer os.RemoveAll(tempDir)

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read archive entry: %w", err)
		}

		cleanName := filepath.Clean(header.Name)
		if strings.HasPrefix(cleanName, "..") || filepath.IsAbs(cleanName) {
			return nil, fmt.Errorf("security violation: archive contains illegal path traversal %q", header.Name)
		}

		target := filepath.Join(tempDir, cleanName)
		cleanTempDir := filepath.Clean(tempDir) + string(os.PathSeparator)
		if !strings.HasPrefix(target, cleanTempDir) && target != filepath.Clean(tempDir) {
			return nil, fmt.Errorf("security violation: path %q attempts directory escape", target)
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0755); err != nil {
				return nil, err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return nil, err
			}
			outFile, err := os.Create(target)
			if err != nil {
				return nil, err
			}
			if _, err := io.Copy(outFile, tr); err != nil {
				outFile.Close()
				return nil, err
			}
			outFile.Close()
			_ = os.Chmod(target, os.FileMode(header.Mode))
		}
	}

	return installFromDirectory(tempDir, registryDir, false)
}
