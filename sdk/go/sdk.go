package sdk

import (
	"context"
	"fmt"
	"net"
	"os"
	"time"

	"google.golang.org/grpc"

	pb "github.com/cliarc/cliarc/protocol/generated/go/cliarc/protocol"
)

// Plugin is the interface that all CLIARC plugins must implement.
type Plugin interface {
	// Initialize is called once when the plugin starts.
	Initialize(ctx context.Context, req *pb.InitializeRequest) (*pb.InitializeResponse, error)
	// Execute handles action requests from the Core.
	Execute(ctx context.Context, req *pb.ExecuteRequest) (*pb.ExecuteResponse, error)
	// Health returns the current health status.
	Health(ctx context.Context, req *pb.HealthRequest) (*pb.HealthResponse, error)
	// Shutdown is called before the plugin process exits.
	Shutdown(ctx context.Context, req *pb.ShutdownRequest) (*pb.ShutdownResponse, error)
}

// BasePlugin provides a minimal implementation of Plugin.
// Plugin authors can embed this and override methods.
type BasePlugin struct {
	Manifest *pb.PluginManifest
}

func (p *BasePlugin) Initialize(ctx context.Context, req *pb.InitializeRequest) (*pb.InitializeResponse, error) {
	return &pb.InitializeResponse{
		Status:   pb.Status_STATUS_OK,
		Manifest: p.Manifest,
	}, nil
}

func (p *BasePlugin) Execute(ctx context.Context, req *pb.ExecuteRequest) (*pb.ExecuteResponse, error) {
	return &pb.ExecuteResponse{
		Status: pb.Status_STATUS_ERROR,
		Error: &pb.ErrorInfo{
			Code:    "not_implemented",
			Message: fmt.Sprintf("action %q not implemented", req.Action),
		},
	}, nil
}

func (p *BasePlugin) Health(ctx context.Context, req *pb.HealthRequest) (*pb.HealthResponse, error) {
	return &pb.HealthResponse{
		Status: pb.Status_STATUS_OK,
		Details: map[string]string{
			"healthy": "true",
		},
	}, nil
}

func (p *BasePlugin) Shutdown(ctx context.Context, req *pb.ShutdownRequest) (*pb.ShutdownResponse, error) {
	return &pb.ShutdownResponse{
		Status: pb.Status_STATUS_OK,
	}, nil
}

// Server wraps a Plugin and exposes it as a gRPC service.
type Server struct {
	pb.UnimplementedPluginServiceServer
	plugin Plugin
	grpc   *grpc.Server
	lis    net.Listener
}

// NewServer creates a new SDK server for the given plugin.
func NewServer(plugin Plugin) *Server {
	return &Server{plugin: plugin}
}

// Serve starts the gRPC server and connects to the Core.
// It reads CLIARC_PLUGIN_GRPC_ADDR from the environment.
func (s *Server) Serve() error {
	addr := os.Getenv("CLIARC_PLUGIN_GRPC_ADDR")
	if addr == "" {
		// For standalone testing, listen on a random port
		lis, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			return fmt.Errorf("sdk: failed to listen: %w", err)
		}
		s.lis = lis
		addr = lis.Addr().String()
		fmt.Fprintf(os.Stderr, "[sdk] standalone mode, listening on %s\n", addr)
	} else {
		lis, err := net.Listen("tcp", addr)
		if err != nil {
			return fmt.Errorf("sdk: failed to listen on %s: %w", addr, err)
		}
		s.lis = lis
	}

	s.grpc = grpc.NewServer()
	pb.RegisterPluginServiceServer(s.grpc, s)
	return s.grpc.Serve(s.lis)
}

// Stop gracefully stops the gRPC server.
func (s *Server) Stop() {
	if s.grpc != nil {
		s.grpc.GracefulStop()
	}
}

// Initialize implements PluginServiceServer.
func (s *Server) Initialize(ctx context.Context, req *pb.InitializeRequest) (*pb.InitializeResponse, error) {
	return s.plugin.Initialize(ctx, req)
}

// Execute implements PluginServiceServer.
func (s *Server) Execute(ctx context.Context, req *pb.ExecuteRequest) (*pb.ExecuteResponse, error) {
	return s.plugin.Execute(ctx, req)
}

// Health implements PluginServiceServer.
func (s *Server) Health(ctx context.Context, req *pb.HealthRequest) (*pb.HealthResponse, error) {
	return s.plugin.Health(ctx, req)
}

// Shutdown implements PluginServiceServer.
func (s *Server) Shutdown(ctx context.Context, req *pb.ShutdownRequest) (*pb.ShutdownResponse, error) {
	resp, err := s.plugin.Shutdown(ctx, req)
	go func() {
		time.Sleep(100 * time.Millisecond)
		s.Stop()
	}()
	return resp, err
}
