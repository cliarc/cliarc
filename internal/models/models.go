package models

import (
	"time"
)

// PluginState represents the lifecycle state of a plugin.
type PluginState string

const (
	PluginStateDiscovered   PluginState = "discovered"
	PluginStateRegistered   PluginState = "registered"
	PluginStateStarting     PluginState = "starting"
	PluginStateRunning      PluginState = "running"
	PluginStateUnhealthy    PluginState = "unhealthy"
	PluginStateStopping     PluginState = "stopping"
	PluginStateStopped      PluginState = "stopped"
	PluginStateCrashed      PluginState = "crashed"
)

// PluginInfo holds runtime information about a loaded plugin.
type PluginInfo struct {
	Name            string            `json:"name"`
	Version         string            `json:"version"`
	ProtocolVersion string            `json:"protocol_version"`
	Description     string            `json:"description"`
	Author          string            `json:"author"`
	Permissions     []string          `json:"permissions"`
	Actions         []string          `json:"actions"`
	State           PluginState       `json:"state"`
	PID             int               `json:"pid,omitempty"`
	Address         string            `json:"address,omitempty"`
	StartedAt       *time.Time        `json:"started_at,omitempty"`
	LastHealthCheck *time.Time        `json:"last_health_check,omitempty"`
	ManifestPath    string            `json:"manifest_path"`
	RuntimeType     string            `json:"runtime_type"`
	Command         string            `json:"command"`
	Metadata        map[string]string `json:"metadata,omitempty"`
}

// Server represents a managed server.
type Server struct {
	ID       string            `json:"id"`
	Name     string            `json:"name"`
	Host     string            `json:"host"`
	Port     int               `json:"port"`
	Username string            `json:"username"`
	SSHKeyID string            `json:"ssh_key_id,omitempty"`
	Password string            `json:"password,omitempty"`
	Tags     map[string]string `json:"tags,omitempty"`
}

// DiagnosticResult represents a structured diagnostic output.
type DiagnosticResult struct {
	Status        string                 `json:"status"`
	Category      string                 `json:"category"`
	Code          string                 `json:"code"`
	Message       string                 `json:"message"`
	Confidence    float64                `json:"confidence"`
	FixSuggestion string                 `json:"fix_suggestion,omitempty"`
	FixCommand    string                 `json:"fix_command,omitempty"`
	CanAutoFix    bool                   `json:"can_auto_fix,omitempty"`
	Fixed         bool                   `json:"fixed,omitempty"`
	Details       map[string]interface{} `json:"details,omitempty"`
}
