# CLIARC Protocol

## Overview

CLIARC uses gRPC with Protocol Buffers for Core <-> Plugin communication.

## Files

- `protocol/proto/common.proto` — Shared types (Status, ErrorInfo, Server, SecretRef)
- `protocol/proto/plugin.proto` — PluginService definition

## Generating Code

### Go
```bash
make proto
```

### Python
```bash
python -m grpc_tools.protoc \
  -I protocol/proto \
  --python_out=./sdk/python \
  --grpc_python_out=./sdk/python \
  protocol/proto/common.proto protocol/proto/plugin.proto
```

### TypeScript/Node.js
```bash
npx grpc_tools_node_protoc \
  -I protocol/proto \
  --js_out=import_style=commonjs,binary:./sdk/ts \
  --grpc_out=grpc_js:./sdk/ts \
  --ts_out=grpc_js:./sdk/ts \
  protocol/proto/common.proto protocol/proto/plugin.proto
```

### Rust
Use `tonic-build` with the proto files.

## Service Methods

| Method | Direction | Purpose |
|--------|-----------|---------|
| Initialize | Core -> Plugin | Plugin startup handshake |
| Execute | Core -> Plugin | Invoke an action |
| Health | Core -> Plugin | Health check |
| Shutdown | Core -> Plugin | Graceful termination |

## Versioning

- `protocol_version` in manifest defines the contract version
- Future versions will be backward compatible where possible
