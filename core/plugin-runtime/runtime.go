package runtime

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	goRuntime "runtime"
	"strings"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	pb "github.com/cliarc/cliarc/protocol/generated/go/cliarc/protocol"
	"github.com/cliarc/cliarc/internal/manifest"
	"github.com/cliarc/cliarc/internal/models"
)

// PluginRuntime manages the lifecycle of a plugin process or script bridge.
type PluginRuntime struct {
	mu           sync.RWMutex
	manifest     *manifest.Manifest
	cmd          *exec.Cmd
	conn         *grpc.ClientConn
	client       pb.PluginServiceClient
	info         *models.PluginInfo
	cancel       context.CancelFunc
	stopped      chan struct{}
	bridgeServer *grpc.Server
	bridgeLis    net.Listener
}

// NewPluginRuntime creates a runtime for a plugin manifest.
func NewPluginRuntime(m *manifest.Manifest) *PluginRuntime {
	return &PluginRuntime{
		manifest: m,
		stopped:  make(chan struct{}),
	}
}

// Start launches the plugin process or script bridge and establishes gRPC communication.
func (r *PluginRuntime) Start(ctx context.Context, workDir string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.cmd != nil && r.cmd.Process != nil {
		return fmt.Errorf("runtime: plugin %q already running", r.manifest.Name)
	}

	targetCmd := r.resolveCommand(r.manifest.Runtime.Command, workDir)

	// If runtime type is "script", host an in-process gRPC bridge for direct execution
	if r.manifest.Runtime.Type == "script" {
		bridgeLis, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			return fmt.Errorf("runtime: failed to listen for script bridge: %w", err)
		}
		bridgeAddr := bridgeLis.Addr().String()

		bridgeSrv := &scriptBridgeServer{
			manifest:  r.manifest,
			targetCmd: targetCmd,
			args:      r.manifest.Runtime.Args,
			workDir:   workDir,
			env:       r.manifest.Runtime.Env,
		}

		grpcSrv := grpc.NewServer()
		pb.RegisterPluginServiceServer(grpcSrv, bridgeSrv)
		go grpcSrv.Serve(bridgeLis)

		r.bridgeServer = grpcSrv
		r.bridgeLis = bridgeLis

		dialCtx, dialCancel := context.WithTimeout(ctx, 3*time.Second)
		defer dialCancel()
		conn, err := grpc.DialContext(dialCtx, bridgeAddr, grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithBlock())
		if err != nil {
			grpcSrv.Stop()
			bridgeLis.Close()
			return fmt.Errorf("runtime: failed to connect to script bridge: %w", err)
		}

		r.conn = conn
		r.client = pb.NewPluginServiceClient(conn)
		r.info = &models.PluginInfo{
			Name:            r.manifest.Name,
			Version:         r.manifest.Version,
			ProtocolVersion: r.manifest.ProtocolVersion,
			Description:     r.manifest.Description,
			Author:          r.manifest.Author,
			Permissions:     r.manifest.Permissions,
			Actions:         r.manifest.Actions,
			State:           models.PluginStateRunning,
			PID:             os.Getpid(),
			Address:         bridgeAddr,
			RuntimeType:     r.manifest.Runtime.Type,
			Command:         r.manifest.Runtime.Command,
		}

		return nil
	}

	// Runtime is "executable" -> Launch native gRPC plugin binary
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return fmt.Errorf("runtime: failed to find available port: %w", err)
	}
	addr := lis.Addr().String()
	lis.Close()

	env := os.Environ()
	env = append(env, fmt.Sprintf("CLIARC_PLUGIN_GRPC_ADDR=%s", addr))
	for k, v := range r.manifest.Runtime.Env {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}

	cmd := exec.CommandContext(ctx, targetCmd, r.manifest.Runtime.Args...)
	cmd.Env = env
	cmd.Dir = workDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("runtime: failed to start plugin %q (cmd %q): %w", r.manifest.Name, targetCmd, err)
	}

	r.cmd = cmd
	r.info = &models.PluginInfo{
		Name:            r.manifest.Name,
		Version:         r.manifest.Version,
		ProtocolVersion: r.manifest.ProtocolVersion,
		Description:     r.manifest.Description,
		Author:          r.manifest.Author,
		Permissions:     r.manifest.Permissions,
		Actions:         r.manifest.Actions,
		State:           models.PluginStateStarting,
		PID:             cmd.Process.Pid,
		Address:         addr,
		RuntimeType:     r.manifest.Runtime.Type,
		Command:         r.manifest.Runtime.Command,
	}

	// Wait for the plugin to start its gRPC server and establish connection
	var conn *grpc.ClientConn
	dialDeadline := time.Now().Add(7 * time.Second)
	for {
		dialCtx, dialCancel := context.WithTimeout(ctx, 500*time.Millisecond)
		c, dialErr := grpc.DialContext(dialCtx, addr, grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithBlock())
		dialCancel()
		if dialErr == nil {
			conn = c
			break
		}
		if time.Now().After(dialDeadline) {
			r.cmd.Process.Kill()
			r.cmd = nil
			return fmt.Errorf("runtime: failed to connect to plugin %q gRPC (%s): %w", r.manifest.Name, addr, dialErr)
		}
		time.Sleep(150 * time.Millisecond)
	}

	r.conn = conn
	r.client = pb.NewPluginServiceClient(conn)

	initCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	resp, err := r.client.Initialize(initCtx, &pb.InitializeRequest{
		CoreVersion: "0.1.0",
		WorkDir:     workDir,
	})
	if err != nil {
		r.conn.Close()
		r.cmd.Process.Kill()
		r.cmd = nil
		return fmt.Errorf("runtime: plugin %q initialize failed: %w", r.manifest.Name, err)
	}
	if resp.Status != pb.Status_STATUS_OK {
		r.conn.Close()
		r.cmd.Process.Kill()
		r.cmd = nil
		return fmt.Errorf("runtime: plugin %q initialize rejected: %s", r.manifest.Name, resp.Error.GetMessage())
	}

	r.info.State = models.PluginStateRunning

	loopCtx, loopCancel := context.WithCancel(context.Background())
	r.cancel = loopCancel
	go r.healthCheckLoop(loopCtx)
	go r.crashDetectionLoop(loopCtx)

	return nil
}

// Stop gracefully stops the plugin process and bridge server.
func (r *PluginRuntime) Stop(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.cancel != nil {
		r.cancel()
	}

	if r.client != nil {
		shutdownCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
		defer cancel()
		_, _ = r.client.Shutdown(shutdownCtx, &pb.ShutdownRequest{Graceful: true, Reason: "stop"})
	}

	if r.conn != nil {
		_ = r.conn.Close()
		r.conn = nil
	}

	if r.bridgeServer != nil {
		r.bridgeServer.GracefulStop()
		r.bridgeServer = nil
	}
	if r.bridgeLis != nil {
		_ = r.bridgeLis.Close()
		r.bridgeLis = nil
	}

	if r.cmd != nil && r.cmd.Process != nil {
		r.info.State = models.PluginStateStopping
		done := make(chan error, 1)
		go func() {
			done <- r.cmd.Wait()
		}()

		select {
		case <-done:
		case <-time.After(3 * time.Second):
			_ = r.cmd.Process.Kill()
		}
		r.cmd = nil
	}

	if r.info != nil {
		r.info.State = models.PluginStateStopped
	}

	return nil
}

// Client returns the gRPC client for the plugin.
func (r *PluginRuntime) Client() pb.PluginServiceClient {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.client
}

// Info returns a copy of the current plugin info.
func (r *PluginRuntime) Info() *models.PluginInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.info == nil {
		return nil
	}
	cp := *r.info
	return &cp
}

// healthCheckLoop periodically checks plugin health.
func (r *PluginRuntime) healthCheckLoop(ctx context.Context) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if r.client == nil {
				continue
			}
			healthCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			resp, err := r.client.Health(healthCtx, &pb.HealthRequest{Nonce: fmt.Sprintf("%d", time.Now().UnixNano())})
			cancel()
			r.mu.Lock()
			if err != nil {
				r.info.State = models.PluginStateUnhealthy
			} else {
				r.info.State = models.PluginStateRunning
				if resp.Status == pb.Status_STATUS_ERROR {
					r.info.State = models.PluginStateUnhealthy
				}
			}
			now := time.Now()
			r.info.LastHealthCheck = &now
			r.mu.Unlock()
		}
	}
}

// crashDetectionLoop waits for the plugin process to exit unexpectedly.
func (r *PluginRuntime) crashDetectionLoop(ctx context.Context) {
	if r.cmd == nil {
		return
	}
	_ = r.cmd.Wait()
	select {
	case <-ctx.Done():
		return
	default:
	}

	r.mu.Lock()
	if r.info != nil && r.info.State != models.PluginStateStopping && r.info.State != models.PluginStateStopped {
		r.info.State = models.PluginStateCrashed
	}
	if r.conn != nil {
		r.conn.Close()
	}
	r.mu.Unlock()
	close(r.stopped)
}

// IsRunning returns true if the plugin process is active.
func (r *PluginRuntime) IsRunning() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return (r.cmd != nil && r.cmd.Process != nil && r.info != nil && r.info.State == models.PluginStateRunning) || (r.bridgeServer != nil && r.info != nil && r.info.State == models.PluginStateRunning)
}

// Manifest returns the underlying plugin manifest.
func (r *PluginRuntime) Manifest() *manifest.Manifest {
	return r.manifest
}

// resolveCommand searches for the plugin executable across workDir, bin/, and PATH.
func (r *PluginRuntime) resolveCommand(cmd string, workDir string) string {
	var candidates []string
	if goRuntime.GOOS == "windows" && !strings.HasSuffix(strings.ToLower(cmd), ".exe") {
		candidates = append(candidates, cmd+".exe")
	}
	candidates = append(candidates, cmd)

	cmdLower := strings.ToLower(cmd)
	switch cmdLower {
	case "python", "python3", "py":
		for _, alt := range []string{"python", "python3", "py"} {
			if goRuntime.GOOS == "windows" {
				candidates = append(candidates, alt+".exe")
			}
			candidates = append(candidates, alt)
		}
	case "node", "nodejs":
		for _, alt := range []string{"node", "nodejs"} {
			if goRuntime.GOOS == "windows" {
				candidates = append(candidates, alt+".exe")
			}
			candidates = append(candidates, alt)
		}
	case "bash", "sh":
		for _, alt := range []string{"bash", "sh"} {
			if goRuntime.GOOS == "windows" {
				candidates = append(candidates, alt+".exe")
			}
			candidates = append(candidates, alt)
		}
	}

	if filepath.IsAbs(cmd) {
		for _, c := range candidates {
			if _, err := os.Stat(c); err == nil {
				return c
			}
		}
	}

	if r.manifest != nil && r.manifest.Dir != "" {
		for _, c := range candidates {
			p := filepath.Join(r.manifest.Dir, c)
			if _, err := os.Stat(p); err == nil {
				return p
			}
			pBin := filepath.Join(r.manifest.Dir, "bin", c)
			if _, err := os.Stat(pBin); err == nil {
				return pBin
			}
		}
	}

	if workDir != "" {
		for _, c := range candidates {
			p := filepath.Join(workDir, c)
			if _, err := os.Stat(p); err == nil {
				return p
			}
			pBin := filepath.Join(workDir, "bin", c)
			if _, err := os.Stat(pBin); err == nil {
				return pBin
			}
			if r.manifest != nil {
				pNamed := filepath.Join(workDir, r.manifest.Name, c)
				if _, err := os.Stat(pNamed); err == nil {
					return pNamed
				}
				pNamedBin := filepath.Join(workDir, r.manifest.Name, "bin", c)
				if _, err := os.Stat(pNamedBin); err == nil {
					return pNamedBin
				}
			}
		}
	}

	for _, c := range candidates {
		p := filepath.Join("bin", c)
		if _, err := os.Stat(p); err == nil {
			if abs, err := filepath.Abs(p); err == nil {
				return abs
			}
			return p
		}
	}

	if execPath, err := os.Executable(); err == nil {
		execDir := filepath.Dir(execPath)
		for _, c := range candidates {
			p := filepath.Join(execDir, c)
			if _, err := os.Stat(p); err == nil {
				return p
			}
			pBin := filepath.Join(execDir, "bin", c)
			if _, err := os.Stat(pBin); err == nil {
				return pBin
			}
		}
	}

	for _, c := range candidates {
		if path, err := exec.LookPath(c); err == nil {
			return path
		}
	}

	return cmd
}

// scriptBridgeServer serves gRPC requests for script-based plugins by invoking their entrypoint.
type scriptBridgeServer struct {
	pb.UnimplementedPluginServiceServer
	manifest  *manifest.Manifest
	targetCmd string
	args      []string
	workDir   string
	env       map[string]string
}

func (s *scriptBridgeServer) Initialize(ctx context.Context, req *pb.InitializeRequest) (*pb.InitializeResponse, error) {
	return &pb.InitializeResponse{
		Status: pb.Status_STATUS_OK,
		Manifest: &pb.PluginManifest{
			Name:        s.manifest.Name,
			Version:     s.manifest.Version,
			Description: s.manifest.Description,
			Actions:     s.manifest.Actions,
		},
	}, nil
}

func (s *scriptBridgeServer) Execute(ctx context.Context, req *pb.ExecuteRequest) (*pb.ExecuteResponse, error) {
	execArgs := append([]string{}, s.args...)
	execArgs = append(execArgs, req.Action)
	if len(req.Payload) > 0 {
		execArgs = append(execArgs, string(req.Payload))
	}

	cmd := exec.CommandContext(ctx, s.targetCmd, execArgs...)
	cmd.Dir = s.workDir

	env := os.Environ()
	for k, v := range s.env {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}
	cmd.Env = env

	output, err := cmd.CombinedOutput()
	if err != nil {
		return &pb.ExecuteResponse{
			Status: pb.Status_STATUS_ERROR,
			Error: &pb.ErrorInfo{
				Code:     "execution_failed",
				Message:  fmt.Sprintf("script execution failed: %v\nOutput: %s", err, string(output)),
				Category: "execution",
			},
		}, nil
	}

	trimmed := bytes.TrimSpace(output)
	if len(trimmed) == 0 {
		trimmed = []byte(`{"status":"ok"}`)
	}

	return &pb.ExecuteResponse{
		Status: pb.Status_STATUS_OK,
		Result: trimmed,
	}, nil
}

func (s *scriptBridgeServer) Health(ctx context.Context, req *pb.HealthRequest) (*pb.HealthResponse, error) {
	execArgs := append([]string{}, s.args...)
	execArgs = append(execArgs, "health")

	cmd := exec.CommandContext(ctx, s.targetCmd, execArgs...)
	cmd.Dir = s.workDir

	env := os.Environ()
	for k, v := range s.env {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}
	cmd.Env = env

	out, err := cmd.CombinedOutput()
	details := map[string]string{
		"runtime":   s.manifest.Runtime.Command,
		"targetCmd": s.targetCmd,
	}

	if err == nil && len(out) > 0 {
		var parsed map[string]interface{}
		if jsonErr := json.Unmarshal(out, &parsed); jsonErr == nil {
			for k, v := range parsed {
				details[k] = fmt.Sprintf("%v", v)
			}
		}
	}

	return &pb.HealthResponse{
		Status:  pb.Status_STATUS_OK,
		Details: details,
	}, nil
}

func (s *scriptBridgeServer) Shutdown(ctx context.Context, req *pb.ShutdownRequest) (*pb.ShutdownResponse, error) {
	return &pb.ShutdownResponse{Status: pb.Status_STATUS_OK}, nil
}
