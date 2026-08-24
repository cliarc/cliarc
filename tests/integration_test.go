package tests

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/cliarc/cliarc/core/events"
	manager "github.com/cliarc/cliarc/core/plugin-manager"
	"github.com/cliarc/cliarc/core/permissions"
	"github.com/cliarc/cliarc/core/registry"
	"github.com/cliarc/cliarc/internal/packaging"
	"github.com/cliarc/cliarc/internal/scaffold"
	pb "github.com/cliarc/cliarc/protocol/generated/go/cliarc/protocol"
	"github.com/stretchr/testify/require"
)

func TestCoreToNativePluginCommunication(t *testing.T) {
	if os.Getenv("CI") == "true" {
		t.Skip("Skipping integration test in CI")
	}

	tempDir := t.TempDir()
	pluginName := "test-agent"
	pluginSourceDir := filepath.Join(tempDir, pluginName)

	// Scaffold a native Go plugin
	_, err := scaffold.ScaffoldPlugin(scaffold.ScaffoldOptions{
		Name:      pluginName,
		Language:  "go",
		OutputDir: pluginSourceDir,
	})
	require.NoError(t, err)

	absRoot, err := filepath.Abs("..")
	require.NoError(t, err)
	absRootSlash := filepath.ToSlash(absRoot)

	goModContent := fmt.Sprintf(`module cliarc-plugin-test-agent

go 1.25.0

require (
	github.com/cliarc/cliarc v0.0.0
	google.golang.org/grpc v1.62.1
)

replace github.com/cliarc/cliarc => %s
`, absRootSlash)
	require.NoError(t, os.WriteFile(filepath.Join(pluginSourceDir, "go.mod"), []byte(goModContent), 0644))

	tidyCmd := exec.Command("go", "mod", "tidy")
	tidyCmd.Dir = pluginSourceDir
	tidyOut, err := tidyCmd.CombinedOutput()
	require.NoError(t, err, "tidy error: %s", string(tidyOut))

	// Build the plugin binary
	buildRes, err := packaging.Build(packaging.BuildOptions{
		SourceDir: pluginSourceDir,
	})
	require.NoError(t, err)
	require.NotEmpty(t, buildRes.Binaries)

	// Link/install into a temporary registry directory
	registryDir := filepath.Join(tempDir, "registry")
	installRes, err := packaging.Install(packaging.InstallOptions{
		SourcePath:  pluginSourceDir,
		RegistryDir: registryDir,
	})
	require.NoError(t, err)
	require.Equal(t, pluginName, installRes.PluginName)

	// Setup manager
	reg := registry.New()
	bus := events.NewBus()
	perm := permissions.NewValidator()
	mgr := manager.NewManager(reg, perm, bus, registryDir)

	manifests, err := mgr.Discover(registryDir)
	require.NoError(t, err)
	require.Len(t, manifests, 1)

	require.NoError(t, mgr.Load(manifests[0]))

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	require.NoError(t, mgr.Start(ctx, pluginName))

	// Wait for plugin process initialization
	time.Sleep(500 * time.Millisecond)

	// Execute plugin action
	payload, _ := json.Marshal(map[string]interface{}{
		"query": "ping",
	})
	resp, err := mgr.Execute(ctx, pluginName, pluginName+".run", payload)
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.Equal(t, pb.Status_STATUS_OK, resp.Status)

	// Health check
	health, err := mgr.Health(ctx, pluginName)
	require.NoError(t, err)
	require.NotNil(t, health)
	require.Equal(t, pb.Status_STATUS_OK, health.Status)

	// Stop
	require.NoError(t, mgr.Stop(ctx, pluginName))
}
