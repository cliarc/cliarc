package ignore

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

// DefaultIgnores contains files and directories that must never be installed or packaged.
var DefaultIgnores = []string{
	// Version control & IDEs
	".git",
	".github",
	".vscode",
	".idea",
	".DS_Store",
	"Thumbs.db",

	// Source files (must not be copied into distribution/registry)
	"*.go",
	"*.rs",
	"*.c",
	"*.cpp",
	"*.cc",
	"*.cxx",
	"*.h",
	"*.hpp",
	"go.mod",
	"go.sum",
	"Cargo.toml",
	"Cargo.lock",
	"Makefile",
	"CMakeLists.txt",

	// Build artifacts & intermediate objects
	"target/debug",
	"target/tmp",
	"target/release/build",
	"target/release/deps",
	"target/release/incremental",
	"*.o",
	"*.obj",
	"*.a",
	"*.lib",
	"*.pdb",
	"*.ilk",
	"*.tmp",
	"*.log",

	// Python & Node legacy ignores
	"node_modules",
	"__pycache__",
	"*.pyc",
	".venv",
	"venv",
}

// PatternRule represents a parsed ignore pattern.
type PatternRule struct {
	Pattern   string
	IsNegated bool
	IsDirOnly bool
}

// Matcher checks if a given relative path matches ignore patterns.
type Matcher struct {
	rules []PatternRule
}

// New creates a Matcher by loading .cliarcignore and .gitignore from a directory, plus default rules.
func New(dir string) *Matcher {
	m := &Matcher{}

	// Load defaults first
	for _, p := range DefaultIgnores {
		m.addRule(p)
	}

	// Read .cliarcignore if present
	m.loadFromFile(filepath.Join(dir, ".cliarcignore"))

	// Read .gitignore as secondary fallback
	m.loadFromFile(filepath.Join(dir, ".gitignore"))

	return m
}

// NewEmpty creates a Matcher with only the provided custom rules.
func NewEmpty() *Matcher {
	return &Matcher{}
}

// AddPattern adds a pattern rule to the matcher.
func (m *Matcher) AddPattern(pattern string) {
	m.addRule(pattern)
}

func (m *Matcher) addRule(raw string) {
	line := strings.TrimSpace(raw)
	if line == "" || strings.HasPrefix(line, "#") {
		return
	}

	isNegated := false
	if strings.HasPrefix(line, "!") {
		isNegated = true
		line = strings.TrimPrefix(line, "!")
	}

	isDirOnly := false
	if strings.HasSuffix(line, "/") {
		isDirOnly = true
		line = strings.TrimSuffix(line, "/")
	}

	pat := filepath.ToSlash(line)
	pat = strings.TrimPrefix(pat, "./")
	pat = strings.TrimPrefix(pat, "/")

	m.rules = append(m.rules, PatternRule{
		Pattern:   pat,
		IsNegated: isNegated,
		IsDirOnly: isDirOnly,
	})
}

func (m *Matcher) loadFromFile(path string) {
	file, err := os.Open(path)
	if err != nil {
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		m.addRule(scanner.Text())
	}
}

// ShouldIgnore tests if a path (relative to root) matches any ignore pattern.
func (m *Matcher) ShouldIgnore(relPath string, isDir bool) bool {
	if relPath == "." || relPath == "" {
		return false
	}

	cleanRel := filepath.ToSlash(filepath.Clean(relPath))
	parts := strings.Split(cleanRel, "/")

	ignored := false

	for _, rule := range m.rules {
		if rule.IsDirOnly && !isDir {
			continue
		}

		matched := false

		// Check component-level matching (e.g. "node_modules", "*.go")
		for _, part := range parts {
			if m.matchGlob(rule.Pattern, part) {
				matched = true
				break
			}
		}

		// Check full path match
		if !matched && m.matchGlob(rule.Pattern, cleanRel) {
			matched = true
		}

		// Check prefix folder match (e.g. "target" matches "target/foo/bar")
		if !matched && strings.HasPrefix(cleanRel, rule.Pattern+"/") {
			matched = true
		}

		if matched {
			if rule.IsNegated {
				ignored = false
			} else {
				ignored = true
			}
		}
	}

	return ignored
}

func (m *Matcher) matchGlob(pattern, str string) bool {
	matched, err := filepath.Match(pattern, str)
	if err == nil && matched {
		return true
	}
	return false
}
