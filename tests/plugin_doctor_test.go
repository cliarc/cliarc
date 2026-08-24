package tests

import (
	"path/filepath"
	"testing"

	"github.com/cliarc/cliarc/internal/packaging"
	"github.com/cliarc/cliarc/internal/scaffold"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPluginScaffoldAndDoctor(t *testing.T) {
	tempDir := t.TempDir()
	pluginName := "sample-plugin"
	pluginDir := filepath.Join(tempDir, pluginName)

	created, err := scaffold.ScaffoldPlugin(scaffold.ScaffoldOptions{
		Name:      pluginName,
		Language:  "go",
		OutputDir: pluginDir,
	})
	require.NoError(t, err)
	assert.NotEmpty(t, created)

	// Check files created
	assert.FileExists(t, filepath.Join(pluginDir, "cliarc.plugin.yaml"))
	assert.FileExists(t, filepath.Join(pluginDir, "README.md"))
	assert.FileExists(t, filepath.Join(pluginDir, "LICENSE"))
	assert.FileExists(t, filepath.Join(pluginDir, "CHANGELOG.md"))
	assert.FileExists(t, filepath.Join(pluginDir, ".gitignore"))
	assert.FileExists(t, filepath.Join(pluginDir, ".cliarcignore"))
	assert.FileExists(t, filepath.Join(pluginDir, ".github", "workflows", "release.yml"))
	assert.FileExists(t, filepath.Join(pluginDir, "src", "main.go"))
	assert.FileExists(t, filepath.Join(pluginDir, "tests", "plugin_test.go"))

	// Validate with Doctor
	report, err := packaging.ValidatePlugin(pluginDir)
	require.NoError(t, err)
	assert.True(t, report.Passed)
	assert.Equal(t, pluginName, report.PluginName)
	assert.Equal(t, 0, report.FailCount)

	// Test Clean
	cleanRes, err := packaging.Clean(pluginDir)
	require.NoError(t, err)
	assert.NotNil(t, cleanRes)
}
