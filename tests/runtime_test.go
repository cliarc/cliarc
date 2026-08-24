package tests

import (
	"testing"

	runtime "github.com/cliarc/cliarc/core/plugin-runtime"
	"github.com/cliarc/cliarc/internal/manifest"
	"github.com/stretchr/testify/assert"
)

func TestPluginRuntimeNotStarted(t *testing.T) {
	mf := &manifest.Manifest{
		Name: "dummy", Version: "1.0.0", ProtocolVersion: "1",
		Runtime: manifest.RuntimeSpec{Type: "executable", Command: "echo"},
		Actions: []string{"test"},
	}
	rt := runtime.NewPluginRuntime(mf)
	assert.False(t, rt.IsRunning())
	assert.Nil(t, rt.Client())
}
