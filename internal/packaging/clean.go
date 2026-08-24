package packaging

import (
	"os"
	"path/filepath"
)

// CleanResult contains summary of cleaned items.
type CleanResult struct {
	RemovedPaths []string
	FreedBytes   int64
}

// Clean removes build artifacts, binaries, temporary directories, and logs from a plugin directory.
func Clean(sourceDir string) (*CleanResult, error) {
	if sourceDir == "" {
		sourceDir = "."
	}
	absDir, err := filepath.Abs(sourceDir)
	if err != nil {
		absDir = sourceDir
	}

	targets := []string{
		"bin",
		"dist",
		"target",
		".tmp",
	}

	var removed []string
	var totalFreed int64

	for _, target := range targets {
		p := filepath.Join(absDir, target)
		if _, err := os.Stat(p); err == nil {
			size := getDirSize(p)
			if err := os.RemoveAll(p); err == nil {
				removed = append(removed, target+"/")
				totalFreed += size
			}
		}
	}

	// Also look for top-level binaries / test binaries / logs
	entries, err := os.ReadDir(absDir)
	if err == nil {
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			name := e.Name()
			if filepath.Ext(name) == ".exe" || filepath.Ext(name) == ".log" || filepath.Ext(name) == ".tmp" || name == "test_runner" {
				p := filepath.Join(absDir, name)
				if info, err := e.Info(); err == nil {
					totalFreed += info.Size()
				}
				if err := os.Remove(p); err == nil {
					removed = append(removed, name)
				}
			}
		}
	}

	return &CleanResult{
		RemovedPaths: removed,
		FreedBytes:   totalFreed,
	}, nil
}

func getDirSize(path string) int64 {
	var size int64
	_ = filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			size += info.Size()
		}
		return nil
	})
	return size
}
