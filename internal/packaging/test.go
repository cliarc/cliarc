package packaging

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// TestOptions configures native plugin test execution.
type TestOptions struct {
	SourceDir string
	Verbose   bool
}

// TestResult contains summary of test run.
type TestResult struct {
	PluginName string
	Language   string
	Passed     bool
	Output     string
	Duration   time.Duration
}

// Test executes the plugin test suite in tests/.
func Test(opts TestOptions) (*TestResult, error) {
	startTime := time.Now()

	srcDir := opts.SourceDir
	if srcDir == "" {
		srcDir = "."
	}
	absSrcDir, err := filepath.Abs(srcDir)
	if err != nil {
		absSrcDir = srcDir
	}

	lang := DetectLanguage(absSrcDir)
	if lang == "unknown" {
		return nil, fmt.Errorf("unable to detect native language in %s", absSrcDir)
	}

	var cmd *exec.Cmd

	switch lang {
	case "go":
		// Run go test in ./tests/... or ./...
		if _, err := os.Stat(filepath.Join(absSrcDir, "tests")); err == nil {
			cmd = exec.Command("go", "test", "-v", "./tests/...")
		} else {
			cmd = exec.Command("go", "test", "-v", "./...")
		}
	case "rust":
		cmd = exec.Command("cargo", "test")
	case "c":
		makefilePath := filepath.Join(absSrcDir, "Makefile")
		if _, err := os.Stat(makefilePath); err == nil {
			cmd = exec.Command("make", "test")
		} else {
			testC := filepath.Join(absSrcDir, "tests", "test_main.c")
			if _, err := os.Stat(testC); err == nil {
				binRunner := filepath.Join(absSrcDir, "bin", "test_runner")
				_ = os.MkdirAll(filepath.Dir(binRunner), 0755)
				compileCmd := exec.Command("gcc", "-O2", "-o", binRunner, testC)
				compileCmd.Dir = absSrcDir
				if out, err := compileCmd.CombinedOutput(); err != nil {
					return &TestResult{
						Language: lang,
						Passed:   false,
						Output:   fmt.Sprintf("Failed to compile C tests: %v\n%s", err, string(out)),
						Duration: time.Since(startTime),
					}, nil
				}
				cmd = exec.Command(binRunner)
			} else {
				return nil, fmt.Errorf("no C test suite found in %s/tests", absSrcDir)
			}
		}
	}

	cmd.Dir = absSrcDir
	cmd.Env = os.Environ()

	out, err := cmd.CombinedOutput()
	duration := time.Since(startTime)

	return &TestResult{
		Language: lang,
		Passed:   err == nil,
		Output:   string(out),
		Duration: duration,
	}, nil
}
