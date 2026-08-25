package tests

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/cliarc/cliarc/internal/manifest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestManifestValidation(t *testing.T) {
	tests := []struct {
		name     string
		manifest manifest.Manifest
		wantErr  bool
	}{
		{
			name: "valid manifest with actions",
			manifest: manifest.Manifest{
				Name: "test", Version: "1.0.0", ProtocolVersion: "1",
				Runtime: manifest.RuntimeSpec{Type: "executable", Command: "test-cmd"},
				Actions: []string{"action.one"},
			},
			wantErr: false,
		},
		{
			name: "valid manifest with commands",
			manifest: manifest.Manifest{
				Name: "test", Version: "1.0.0",
				Binary:   "bin/cliarc-test",
				Commands: []string{"test.run"},
			},
			wantErr: false,
		},
		{
			name: "missing name",
			manifest: manifest.Manifest{
				Version: "1.0.0", ProtocolVersion: "1",
				Runtime: manifest.RuntimeSpec{Type: "executable", Command: "test-cmd"},
				Actions: []string{"action.one"},
			},
			wantErr: true,
		},
		{
			name: "missing actions and commands",
			manifest: manifest.Manifest{
				Name: "test", Version: "1.0.0", ProtocolVersion: "1",
				Runtime: manifest.RuntimeSpec{Type: "executable", Command: "test-cmd"},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.manifest.Validate()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestManifestLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "manifest.yaml")
	content := `
name: ssh
version: 0.1.0
protocol_version: "1"
runtime:
  type: executable
  command: cliarc-ssh
actions:
  - connection.test
`
	require.NoError(t, os.WriteFile(path, []byte(content), 0644))

	mf, err := manifest.Load(path)
	require.NoError(t, err)
	assert.Equal(t, "ssh", mf.Name)
	assert.Equal(t, "0.1.0", mf.Version)
}

func TestCLIARCPluginYAMLLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "plugin.yaml")
	content := `
name: custom-tool
version: 0.2.0
description: A custom tool
author: CLIARC Team
license: MIT
homepage: https://cliarc.com
repository: https://github.com/cliarc/custom-tool
binary: bin/cliarc-custom-tool
platforms:
  - windows
  - linux
architectures:
  - amd64
permissions:
  - custom-tool.run
commands:
  - custom-tool.run
`
	require.NoError(t, os.WriteFile(path, []byte(content), 0644))

	mf, err := manifest.Load(dir)
	require.NoError(t, err)
	assert.Equal(t, "custom-tool", mf.Name)
	assert.Equal(t, "0.2.0", mf.Version)
	assert.Equal(t, "MIT", mf.License)
	assert.Contains(t, mf.Commands, "custom-tool.run")
	assert.Contains(t, mf.Actions, "custom-tool.run")
}

func TestSchemaV1ManifestLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "plugin.yaml")
	content := `
schemaVersion: 1

plugin:
  id: docker
  name: Docker
  version: 1.0.0
  description: Docker management plugin for CLIARC
  author:
    name: Sorabh
    homepage: https://cliarc.com
  license: MIT
  homepage: https://cliarc.com/plugins/docker
  repository: https://github.com/cliarc/plugins

runtime:
  type: wasm
  entry: plugin.wasm
  wasi: true

cli:
  minVersion: 1.0.0

commands:
  ps:
    description: List Docker containers
    handler: docker.container.read
    permissions:
      - docker.container:read
  create:
    description: Create a Docker container
    handler: docker.container.create
    permissions:
      - docker.container:create
  start:
    description: Start a Docker container
    handler: docker.container.start
    permissions:
      - docker.container:start

permissions:
  - docker.container:read
  - docker.container:create
  - docker.container:start

verification:
  algorithm: Ed25519
  keyId: publisher-key-001
  sha256: ""
  signature: ""

package:
  format: wasm
  entry: plugin.wasm
`
	require.NoError(t, os.WriteFile(path, []byte(content), 0644))

	mf, err := manifest.Load(dir)
	require.NoError(t, err)
	assert.Equal(t, 1, mf.SchemaVersion)
	assert.Equal(t, "docker", mf.ID)
	assert.Equal(t, "Docker", mf.Name)
	assert.Equal(t, "1.0.0", mf.Version)
	assert.Equal(t, "Sorabh", mf.Author)
	assert.Equal(t, "MIT", mf.License)
	assert.Equal(t, "wasm", mf.Runtime.Type)
	assert.Equal(t, "plugin.wasm", mf.Runtime.Entry)
	assert.Equal(t, "1.0.0", mf.MinCLIARCVersion)
	assert.Len(t, mf.CommandTree, 3)
	assert.Contains(t, mf.Commands, "docker.container.read")
	assert.Contains(t, mf.Commands, "docker.container.create")
	assert.Contains(t, mf.Commands, "docker.container.start")
	assert.Contains(t, mf.Permissions, "docker.container:read")
}

