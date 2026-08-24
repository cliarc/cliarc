package packaging

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/cliarc/cliarc/internal/manifest"
)

// PackageOptions configures plugin archive creation.
type PackageOptions struct {
	SourceDir  string
	OutputPath string
}

// PackageResult contains metadata for the generated package bundle.
type PackageResult struct {
	ArchivePath string
	PluginName  string
	Version     string
	Checksum    string
	FilesCount  int
	Size        int64
}

// Package creates a production-ready .tar.gz distribution archive for a native plugin.
func Package(opts PackageOptions) (*PackageResult, error) {
	srcDir := opts.SourceDir
	if srcDir == "" {
		srcDir = "."
	}

	mf, err := manifest.Load(srcDir)
	if err != nil {
		return nil, fmt.Errorf("invalid plugin manifest: %w", err)
	}

	// Ensure binary exists in bin/
	binDir := filepath.Join(srcDir, "bin")
	entries, err := os.ReadDir(binDir)
	if err != nil || len(entries) == 0 {
		// Attempt automatic build if bin/ is missing
		_, buildErr := Build(BuildOptions{SourceDir: srcDir})
		if buildErr != nil {
			return nil, fmt.Errorf("plugin binary missing in %s and automatic build failed: %w", binDir, buildErr)
		}
		entries, _ = os.ReadDir(binDir)
	}

	outPath := opts.OutputPath
	if outPath == "" {
		outPath = fmt.Sprintf("%s-%s.tar.gz", mf.Name, mf.Version)
	}
	if err := os.MkdirAll(filepath.Dir(outPath), 0755); err != nil && filepath.Dir(outPath) != "." {
		return nil, fmt.Errorf("create archive destination dir: %w", err)
	}

	tarGzFile, err := os.Create(outPath)
	if err != nil {
		return nil, fmt.Errorf("create archive file: %w", err)
	}
	defer tarGzFile.Close()

	gw := gzip.NewWriter(tarGzFile)
	defer gw.Close()
	tw := tar.NewWriter(gw)
	defer tw.Close()

	filesCount := 0
	var checksumEntries []string

	// Allowed distribution files in root: plugin.yaml, cliarc.plugin.yaml, manifest.yaml, README.md, LICENSE, CHANGELOG.md
	allowedRootFiles := []string{
		"plugin.yaml",
		"plugin.yml",
		"plugin.json",
		"cliarc.plugin.yaml",
		"cliarc.plugin.yml",
		"manifest.yaml",
		"manifest.yml",
		"README.md",
		"README",
		"LICENSE",
		"LICENSE.md",
		"LICENSE.txt",
		"CHANGELOG.md",
		"CHANGELOG",
	}

	for _, filename := range allowedRootFiles {
		filePath := filepath.Join(srcDir, filename)
		if stat, err := os.Stat(filePath); err == nil && !stat.IsDir() {
			hash, err := addFileToTar(tw, filePath, filename, stat.Mode())
			if err != nil {
				return nil, fmt.Errorf("add %s to archive: %w", filename, err)
			}
			checksumEntries = append(checksumEntries, fmt.Sprintf("%s  %s", hash, filename))
			filesCount++
		}
	}

	// Add bin/ folder contents strictly
	binEntries, _ := os.ReadDir(binDir)
	for _, entry := range binEntries {
		if entry.IsDir() {
			continue
		}
		binPath := filepath.Join(binDir, entry.Name())
		tarRelPath := "bin/" + entry.Name()
		hash, err := addFileToTar(tw, binPath, tarRelPath, 0755)
		if err != nil {
			return nil, fmt.Errorf("add binary %s: %w", entry.Name(), err)
		}
		checksumEntries = append(checksumEntries, fmt.Sprintf("%s  %s", hash, tarRelPath))
		filesCount++
	}

	// Add checksums.txt into archive
	checksumContent := strings.Join(checksumEntries, "\n") + "\n"
	hdr := &tar.Header{
		Name: "checksums.txt",
		Mode: 0644,
		Size: int64(len(checksumContent)),
	}
	if err := tw.WriteHeader(hdr); err != nil {
		return nil, fmt.Errorf("write checksums header: %w", err)
	}
	if _, err := tw.Write([]byte(checksumContent)); err != nil {
		return nil, fmt.Errorf("write checksums content: %w", err)
	}
	filesCount++

	_ = tw.Close()
	_ = gw.Close()
	_ = tarGzFile.Close()

	// Compute overall archive SHA256
	archiveHash, err := computeFileSHA256(outPath)
	if err != nil {
		return nil, err
	}

	stat, _ := os.Stat(outPath)

	return &PackageResult{
		ArchivePath: outPath,
		PluginName:  mf.Name,
		Version:     mf.Version,
		Checksum:    archiveHash,
		FilesCount:  filesCount,
		Size:        stat.Size(),
	}, nil
}

func addFileToTar(tw *tar.Writer, srcPath, tarPath string, mode os.FileMode) (string, error) {
	file, err := os.Open(srcPath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return "", err
	}

	hasher := sha256.New()
	tr := io.TeeReader(file, hasher)

	hdr := &tar.Header{
		Name:    filepath.ToSlash(tarPath),
		Mode:    int64(mode),
		Size:    stat.Size(),
		ModTime: stat.ModTime(),
	}

	if err := tw.WriteHeader(hdr); err != nil {
		return "", err
	}

	if _, err := io.Copy(tw, tr); err != nil {
		return "", err
	}

	return hex.EncodeToString(hasher.Sum(nil)), nil
}

func computeFileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// CopyFile copies a single file from src to dst.
func CopyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}
