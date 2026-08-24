# CLIARC Architecture

## Overview

CLIARC is a plugin-based developer platform. The Core is written in Go, but plugins can be written in any language that supports gRPC.

## Core Components

### Plugin Runtime (`core/plugin-runtime/`)
- Launches plugin processes
- Establishes gRPC communication
- Monitors health and detects crashes
- Isolates plugin failures from Core

### Plugin Manager (`core/plugin-manager/`)
- Discovers plugins from directories
- Loads manifests
- Orchestrates lifecycle (start/stop)
- Routes Execute requests
- Validates permissions before routing

### Registry (`core/registry/`)
- Catalog of discovered and loaded plugins
- Thread-safe access to plugin metadata

### Permissions (`core/permissions/`)
- Validates plugin capabilities
- Supports exact and wildcard permissions (`server.*`)
- Prevents unauthorized action execution

### Events (`core/events/`)
- In-memory pub/sub for system events
- Used for health failures, plugin state changes

### Config (`core/config/`)
- JSON-based configuration
- Extension points for future features

## Plugin Protocol

Plugins communicate with Core via gRPC using Protocol Buffers defined in `protocol/proto/`.

The contract is language-independent. To add a Python plugin:
1. Generate Python gRPC stubs from the same `.proto` files
2. Implement the `PluginService` interface
3. Read `CLIARC_PLUGIN_GRPC_ADDR` from environment
4. Start a gRPC server and call `Initialize`

## Security

- Secrets are referenced, never embedded
- Secret providers: env, file, memory (extensible to OS keychains)
- SSH keys are never logged
- Permission system prevents unauthorized actions
