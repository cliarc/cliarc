# Plugin Development Guide

## Go SDK

The Go SDK (`sdk/go/`) simplifies plugin development:

```go
package main

import (
    "context"
    "github.com/cliarc/cliarc/sdk/go"
    pb "github.com/cliarc/cliarc/protocol/generated/go/cliarc/protocol"
)

type MyPlugin struct {
    sdk.BasePlugin
}

func (p *MyPlugin) Execute(ctx context.Context, req *pb.ExecuteRequest) (*pb.ExecuteResponse, error) {
    // Handle actions
    return &pb.ExecuteResponse{Status: pb.Status_STATUS_OK}, nil
}

func main() {
    p := &MyPlugin{}
    p.Manifest = &pb.PluginManifest{
        Name: "my-plugin",
        Version: "1.0.0",
        ProtocolVersion: "1",
        Actions: []string{"my.action"},
    }
    server := sdk.NewServer(p)
    server.Serve()
}
```

## Building a Plugin

```bash
go build -o cliarc-myplugin ./plugins/myplugin
```

## Manifest Format

```yaml
name: my-plugin
version: 1.0.0
protocol_version: "1"
runtime:
  type: executable
  command: cliarc-myplugin
permissions:
  - server.read
actions:
  - my.action
```

## Communication Flow

1. Core reads manifest
2. Core starts plugin process with `CLIARC_PLUGIN_GRPC_ADDR`
3. Plugin starts gRPC server on that address
4. Core calls `Initialize`
5. Core calls `Execute` for actions
6. Core calls `Shutdown` before stopping
