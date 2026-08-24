package permissions

import (
	"fmt"
	"strings"
	"sync"
)

// Validator checks if a plugin has required permissions.
type Validator struct {
	mu          sync.RWMutex
	pluginPerms map[string]map[string]struct{} // plugin -> permissions set
}

// NewValidator creates a permission validator.
func NewValidator() *Validator {
	return &Validator{
		pluginPerms: make(map[string]map[string]struct{}),
	}
}

// Register adds permissions for a plugin.
func (v *Validator) Register(pluginName string, perms []string) {
	v.mu.Lock()
	defer v.mu.Unlock()
	set := make(map[string]struct{}, len(perms))
	for _, p := range perms {
		set[p] = struct{}{}
	}
	v.pluginPerms[pluginName] = set
}

// Unregister removes a plugin's permissions.
func (v *Validator) Unregister(pluginName string) {
	v.mu.Lock()
	defer v.mu.Unlock()
	delete(v.pluginPerms, pluginName)
}

// Check returns true if the plugin has the required permission.
func (v *Validator) Check(pluginName, permission string) bool {
	v.mu.RLock()
	defer v.mu.RUnlock()
	set, ok := v.pluginPerms[pluginName]
	if !ok {
		return false
	}
	// Exact match
	if _, ok := set[permission]; ok {
		return true
	}
	// Wildcard match: e.g., "server.*" matches "server.read"
	for perm := range set {
		if strings.HasSuffix(perm, ".*") {
			prefix := strings.TrimSuffix(perm, ".*")
			if strings.HasPrefix(permission, prefix+".") {
				return true
			}
		}
	}
	return false
}

// ValidateAction checks if a plugin is allowed to perform an action.
func (v *Validator) ValidateAction(pluginName, action string) error {
	// Derive required permission from action name.
	// e.g., "connection.test" -> "connection.test"
	// Plugins must explicitly declare action permissions.
	if !v.Check(pluginName, action) {
		return fmt.Errorf("permission denied: plugin %q does not have permission %q", pluginName, action)
	}
	return nil
}
