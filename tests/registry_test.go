package tests

import (
	"testing"

	"github.com/cliarc/cliarc/core/registry"
	"github.com/cliarc/cliarc/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRegistryRegisterAndGet(t *testing.T) {
	reg := registry.New()
	info := &models.PluginInfo{Name: "ssh", Version: "0.1.0", State: models.PluginStateDiscovered}

	err := reg.Register(info)
	require.NoError(t, err)

	got, ok := reg.Get("ssh")
	require.True(t, ok)
	assert.Equal(t, "0.1.0", got.Version)
}

func TestRegistryDuplicate(t *testing.T) {
	reg := registry.New()
	info := &models.PluginInfo{Name: "ssh", Version: "0.1.0"}
	require.NoError(t, reg.Register(info))
	err := reg.Register(info)
	assert.Error(t, err)
}

func TestRegistryList(t *testing.T) {
	reg := registry.New()
	reg.Register(&models.PluginInfo{Name: "a", Version: "1.0"})
	reg.Register(&models.PluginInfo{Name: "b", Version: "2.0"})
	assert.Len(t, reg.List(), 2)
}
