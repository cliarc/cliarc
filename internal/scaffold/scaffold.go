package scaffold

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ScaffoldOptions holds options for scaffolding a new native plugin.
type ScaffoldOptions struct {
	Name        string
	Language    string
	OutputDir   string
	Description string
	Author      string
	License     string
	Homepage    string
	Repository  string
}

// NormalizeLanguage normalizes user input into standard native language ID (go, rust, c).
func NormalizeLanguage(input string) string {
	input = strings.TrimSpace(strings.ToLower(input))
	input = strings.TrimPrefix(input, ".")
	switch input {
	case "go", "golang":
		return "go"
	case "rs", "rust":
		return "rust"
	case "c", "cpp", "c++":
		return "c"
	default:
		return input
	}
}

// toIdentifier converts kebab-case or snake_case name into a valid CamelCase identifier.
func toIdentifier(name string) string {
	name = strings.ReplaceAll(name, "-", " ")
	name = strings.ReplaceAll(name, "_", " ")
	words := strings.Fields(name)
	var titleWords []string
	for _, w := range words {
		if len(w) > 0 {
			titleWords = append(titleWords, strings.ToUpper(w[:1])+strings.ToLower(w[1:]))
		}
	}
	res := strings.Join(titleWords, "")
	if res == "" {
		return "Plugin"
	}
	return res
}

// SupportedLanguages returns list of supported native compiled languages.
func SupportedLanguages() []string {
	return []string{"go", "rust", "c"}
}

// GeneratedFile represents a file to be written on disk.
type GeneratedFile struct {
	Path    string
	Content string
}

// GeneratePluginFiles returns all files for the specified native plugin configuration.
func GeneratePluginFiles(opts ScaffoldOptions) ([]GeneratedFile, error) {
	if opts.Name == "" {
		return nil, fmt.Errorf("plugin name cannot be empty")
	}

	opts.Language = NormalizeLanguage(opts.Language)
	if opts.Description == "" {
		opts.Description = fmt.Sprintf("CLIARC native %s plugin for %s", opts.Language, opts.Name)
	}
	if opts.Author == "" {
		opts.Author = "CLIARC Community"
	}
	if opts.License == "" {
		opts.License = "Apache-2.0"
	}
	if opts.Homepage == "" {
		opts.Homepage = "https://cliarc.com"
	}
	if opts.Repository == "" {
		opts.Repository = fmt.Sprintf("https://github.com/cliarc/plugin-%s", opts.Name)
	}

	switch opts.Language {
	case "go":
		return generateGoPlugin(opts)
	case "rust":
		return generateRustPlugin(opts)
	case "c":
		return generateCPlugin(opts)
	default:
		return nil, fmt.Errorf("unsupported native language %q. CLIARC plugins must be compiled native binaries (Supported: %s)", opts.Language, strings.Join(SupportedLanguages(), ", "))
	}
}

// ScaffoldPlugin generates and writes all plugin files to disk.
func ScaffoldPlugin(opts ScaffoldOptions) ([]string, error) {
	files, err := GeneratePluginFiles(opts)
	if err != nil {
		return nil, err
	}

	targetDir := opts.OutputDir
	if targetDir == "" {
		targetDir = opts.Name
	}

	var createdPaths []string
	for _, f := range files {
		fullPath := filepath.Join(targetDir, f.Path)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			return nil, fmt.Errorf("mkdir: %w", err)
		}
		if err := os.WriteFile(fullPath, []byte(f.Content), 0644); err != nil {
			return nil, fmt.Errorf("write %s: %w", fullPath, err)
		}
		createdPaths = append(createdPaths, fullPath)
	}

	// Create bin/ directory placeholder
	binDir := filepath.Join(targetDir, "bin")
	_ = os.MkdirAll(binDir, 0755)

	return createdPaths, nil
}

// buildManifestYAML builds the standard cliarc.plugin.yaml content.
func buildManifestYAML(opts ScaffoldOptions, extraCmds []string) string {
	cmdList := []string{fmt.Sprintf("%s.run", opts.Name), fmt.Sprintf("%s.status", opts.Name)}
	if len(extraCmds) > 0 {
		cmdList = append(cmdList, extraCmds...)
	}

	var cmdStr strings.Builder
	for _, c := range cmdList {
		cmdStr.WriteString(fmt.Sprintf("  - %s\n", c))
	}

	return fmt.Sprintf(`name: %s
version: 0.1.0
description: %s
author: %s
license: %s
homepage: %s
repository: %s
binary: bin/cliarc-%s
platforms:
  - windows
  - linux
  - darwin
architectures:
  - amd64
  - arm64
permissions:
%sdependencies: {}
commands:
%s`, opts.Name, opts.Description, opts.Author, opts.License, opts.Homepage, opts.Repository, opts.Name, cmdStr.String(), cmdStr.String())
}

func commonChangelog(pluginName string) string {
	return fmt.Sprintf(`# Changelog

All notable changes to the **%s** plugin will be documented in this file.

## [0.1.0] - 2026-08-24
### Added
- Initial scaffold for native %s plugin.
- Standard gRPC service interface.
- Local build, test, and release configuration.
`, pluginName, pluginName)
}

func commonGitignore() string {
	return `# Binaries and build outputs
bin/
dist/
target/
*.exe
*.o
*.obj
*.so
*.dylib
*.dll
*.a

# Testing and coverage
*.test
*.out
coverage.*

# IDE & Editor
.vscode/
.idea/
*.swp
*.swo

# OS files
.DS_Store
Thumbs.db

# Logs and temp
*.log
*.tmp
`
}

func commonReleaseWorkflow(pluginName string) string {
	return fmt.Sprintf(`name: Release Plugin

on:
  push:
    tags:
      - 'v*'

jobs:
  build-and-release:
    name: Build Multi-Platform Binaries & Package
    runs-on: ubuntu-latest
    steps:
      - name: Checkout repository
        uses: actions/checkout@v4

      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: '1.25'

      - name: Install CLIARC
        run: go install github.com/cliarc/cliarc/apps/cli@latest

      - name: Build and Package Plugin
        run: |
          cliarc plugin build . --all
          cliarc plugin package . --output dist/%s-${{ github.ref_name }}.tar.gz

      - name: Create GitHub Release
        uses: softprops/action-gh-release@v2
        with:
          files: dist/*.tar.gz
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
`, pluginName)
}

// --- Native Go Plugin Scaffold ---
func generateGoPlugin(opts ScaffoldOptions) ([]GeneratedFile, error) {
	ident := toIdentifier(opts.Name)
	manifestContent := buildManifestYAML(opts, nil)

	mainGoTemplate := `package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"

	"google.golang.org/grpc"

	pb "github.com/cliarc/cliarc/protocol/generated/go/cliarc/protocol"
)

type %sServer struct {
	pb.UnimplementedPluginServiceServer
}

func (s *%sServer) Initialize(ctx context.Context, req *pb.InitializeRequest) (*pb.InitializeResponse, error) {
	return &pb.InitializeResponse{
		Status: pb.Status_STATUS_OK,
		Manifest: &pb.PluginManifest{
			Name:        "%s",
			Version:     "0.1.0",
			Description: "%s",
			Actions:     []string{"%s.run", "%s.status"},
		},
	}, nil
}

func (s *%sServer) Execute(ctx context.Context, req *pb.ExecuteRequest) (*pb.ExecuteResponse, error) {
	switch req.Action {
	case "%s.run":
		res := map[string]interface{}{
			"success": true,
			"plugin":  "%s",
			"message": "Hello from native Go plugin %s!",
		}
		data, _ := json.Marshal(res)
		return &pb.ExecuteResponse{Status: pb.Status_STATUS_OK, Result: data}, nil

	case "%s.status":
		res := map[string]interface{}{
			"status": "healthy",
			"plugin": "%s",
		}
		data, _ := json.Marshal(res)
		return &pb.ExecuteResponse{Status: pb.Status_STATUS_OK, Result: data}, nil

	default:
		return &pb.ExecuteResponse{
			Status: pb.Status_STATUS_ERROR,
			Error: &pb.ErrorInfo{
				Code:     "unknown_action",
				Message:  fmt.Sprintf("Action %%q not supported", req.Action),
				Category: "input",
			},
		}, nil
	}
}

func (s *%sServer) Health(ctx context.Context, req *pb.HealthRequest) (*pb.HealthResponse, error) {
	return &pb.HealthResponse{
		Status: pb.Status_STATUS_OK,
		Details: map[string]string{
			"runtime": "native-go",
			"healthy": "true",
		},
	}, nil
}

func (s *%sServer) Shutdown(ctx context.Context, req *pb.ShutdownRequest) (*pb.ShutdownResponse, error) {
	return &pb.ShutdownResponse{Status: pb.Status_STATUS_OK}, nil
}

func main() {
	addr := os.Getenv("CLIARC_PLUGIN_GRPC_ADDR")
	if addr == "" {
		addr = "127.0.0.1:50051"
	}

	lis, err := net.Listen("tcp", addr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to listen on %%s: %%v\n", addr, err)
		os.Exit(1)
	}

	grpcServer := grpc.NewServer()
	pb.RegisterPluginServiceServer(grpcServer, &%sServer{})

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigCh
		grpcServer.GracefulStop()
	}()

	fmt.Fprintf(os.Stderr, "✓ %%s plugin running on %%s\n", "%s", addr)
	if err := grpcServer.Serve(lis); err != nil {
		fmt.Fprintf(os.Stderr, "server error: %%v\n", err)
	}
}
`

	mainGo := fmt.Sprintf(mainGoTemplate,
		ident, ident, opts.Name, opts.Description, opts.Name, opts.Name,
		ident, opts.Name, opts.Name, opts.Name, opts.Name, opts.Name,
		ident, ident, ident, opts.Name)

	testGoTemplate := `package main_test

import (
	"testing"

	pb "github.com/cliarc/cliarc/protocol/generated/go/cliarc/protocol"
)

func Test%sPluginBasics(t *testing.T) {
	req := &pb.ExecuteRequest{
		Action: "%s.run",
	}
	if req.Action != "%s.run" {
		t.Fatalf("unexpected action: %%s", req.Action)
	}
}`
	testGo := fmt.Sprintf(testGoTemplate, ident, opts.Name, opts.Name)

	ignoreFile := `# .cliarcignore
# Files and patterns ignored when building/packaging/linking this plugin.
# CLIARC will ONLY package/install cliarc.plugin.yaml, README.md, LICENSE, CHANGELOG.md, and bin/

src/
tests/
*.go
*.c
*.h
*.rs
go.mod
go.sum
Makefile
.git/
.github/
.vscode/
.idea/
*.tmp
*.log
`

	readme := fmt.Sprintf(`# CLIARC Native Plugin: %s (Go)

%s

## Commands
`+"```bash"+`
# 1. Build native binary into bin/
cliarc plugin build .

# 2. Run unit tests in tests/
cliarc plugin test .

# 3. Link into ~/.cliarc/plugins (only manifest, README, LICENSE, and host bin/ are installed)
cliarc plugin link .

# 4. Check health and execute
cliarc plugin health %s
cliarc plugin run %s %s.run

# 5. Validate plugin structure
cliarc plugin doctor .

# 6. Package for distribution
cliarc plugin package . --output dist/%s-0.1.0.tar.gz
`+"```"+`
`, opts.Name, opts.Description, opts.Name, opts.Name, opts.Name, opts.Name)

	license := fmt.Sprintf(`Copyright 2026 %s

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
`, opts.Author)

	return []GeneratedFile{
		{Path: "cliarc.plugin.yaml", Content: manifestContent},
		{Path: "src/main.go", Content: mainGo},
		{Path: "tests/plugin_test.go", Content: testGo},
		{Path: "README.md", Content: readme},
		{Path: "LICENSE", Content: license},
		{Path: "CHANGELOG.md", Content: commonChangelog(opts.Name)},
		{Path: ".gitignore", Content: commonGitignore()},
		{Path: ".cliarcignore", Content: ignoreFile},
		{Path: ".github/workflows/release.yml", Content: commonReleaseWorkflow(opts.Name)},
	}, nil
}

// --- Native Rust Plugin Scaffold ---
func generateRustPlugin(opts ScaffoldOptions) ([]GeneratedFile, error) {
	manifestContent := buildManifestYAML(opts, nil)

	cargoToml := fmt.Sprintf(`[package]
name = "cliarc-plugin-%s"
version = "0.1.0"
edition = "2021"
description = "%s"
authors = ["%s"]

[[bin]]
name = "cliarc-%s"
path = "src/main.rs"

[dependencies]
tokio = { version = "1.0", features = ["full"] }
serde = { version = "1.0", features = ["derive"] }
serde_json = "1.0"
`, opts.Name, opts.Description, opts.Author, opts.Name)

	mainRs := fmt.Sprintf(`//! CLIARC Native Plugin: %s (Rust)

use std::env;
use std::net::TcpListener;

#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    let addr = env::var("CLIARC_PLUGIN_GRPC_ADDR").unwrap_or_else(|_| "127.0.0.1:50051".to_string());
    eprintln!("Starting %s plugin on {}...", addr);

    let listener = TcpListener::bind(&addr)?;
    eprintln!("✓ %s plugin listening on {}", addr);

    for stream in listener.incoming() {
        match stream {
            Ok(_stream) => {}
            Err(e) => eprintln!("Connection failed: {}", e),
        }
    }
    Ok(())
}
`, opts.Name, opts.Name, opts.Name)

	testRs := fmt.Sprintf(`#[cfg(test)]
mod tests {
    #[test]
    fn test_%s_initialization() {
        assert_eq!(2 + 2, 4);
    }
}
`, strings.ReplaceAll(opts.Name, "-", "_"))

	ignoreFile := `# .cliarcignore
src/
tests/
target/
**/*.rs.bk
Cargo.lock
*.rs
.git/
.github/
.vscode/
.idea/
`

	readme := fmt.Sprintf(`# CLIARC Native Plugin: %s (Rust)

%s

## Commands
`+"```bash"+`
cliarc plugin build .
cliarc plugin test .
cliarc plugin link .
cliarc plugin health %s
`+"```"+`
`, opts.Name, opts.Description, opts.Name)

	license := fmt.Sprintf(`Copyright 2026 %s (Apache-2.0)`, opts.Author)

	return []GeneratedFile{
		{Path: "cliarc.plugin.yaml", Content: manifestContent},
		{Path: "Cargo.toml", Content: cargoToml},
		{Path: "src/main.rs", Content: mainRs},
		{Path: "tests/plugin_test.rs", Content: testRs},
		{Path: "README.md", Content: readme},
		{Path: "LICENSE", Content: license},
		{Path: "CHANGELOG.md", Content: commonChangelog(opts.Name)},
		{Path: ".gitignore", Content: commonGitignore()},
		{Path: ".cliarcignore", Content: ignoreFile},
		{Path: ".github/workflows/release.yml", Content: commonReleaseWorkflow(opts.Name)},
	}, nil
}

// --- Native C Plugin Scaffold ---
func generateCPlugin(opts ScaffoldOptions) ([]GeneratedFile, error) {
	manifestContent := buildManifestYAML(opts, nil)

	mainC := fmt.Sprintf(`/*
 * CLIARC Native Plugin: %s (C)
 */

#include <stdio.h>
#include <stdlib.h>
#include <string.h>

int main(int argc, char **argv) {
    const char *addr = getenv("CLIARC_PLUGIN_GRPC_ADDR");
    if (!addr) {
        addr = "127.0.0.1:50051";
    }
    
    fprintf(stderr, "✓ %s plugin (C) running on %%s\n", addr);
    return 0;
}
`, opts.Name, opts.Name)

	testC := fmt.Sprintf(`/*
 * Unit test for %s plugin (C)
 */

#include <stdio.h>
#include <assert.h>

int main() {
    printf("Running %s tests...\n");
    assert(1 == 1);
    printf("✓ All %s tests passed.\n");
    return 0;
}
`, opts.Name, opts.Name, opts.Name)

	makefile := fmt.Sprintf(`CC ?= gcc
CFLAGS ?= -O2 -Wall

all: bin/cliarc-%s

bin/cliarc-%s: src/main.c
	@mkdir -p bin
	$(CC) $(CFLAGS) -o $@ $<

test: tests/test_main.c
	$(CC) $(CFLAGS) -o bin/test_runner tests/test_main.c
	./bin/test_runner

clean:
	rm -rf bin
`, opts.Name, opts.Name)

	ignoreFile := `# .cliarcignore
src/
tests/
Makefile
*.c
*.h
*.o
*.obj
.git/
.github/
.vscode/
`

	readme := fmt.Sprintf(`# CLIARC Native Plugin: %s (C)

%s

## Commands
`+"```bash"+`
cliarc plugin build .
cliarc plugin test .
cliarc plugin link .
`+"```"+`
`, opts.Name, opts.Description)

	license := fmt.Sprintf(`Copyright 2026 %s (Apache-2.0)`, opts.Author)

	return []GeneratedFile{
		{Path: "cliarc.plugin.yaml", Content: manifestContent},
		{Path: "src/main.c", Content: mainC},
		{Path: "tests/test_main.c", Content: testC},
		{Path: "Makefile", Content: makefile},
		{Path: "README.md", Content: readme},
		{Path: "LICENSE", Content: license},
		{Path: "CHANGELOG.md", Content: commonChangelog(opts.Name)},
		{Path: ".gitignore", Content: commonGitignore()},
		{Path: ".cliarcignore", Content: ignoreFile},
		{Path: ".github/workflows/release.yml", Content: commonReleaseWorkflow(opts.Name)},
	}, nil
}
