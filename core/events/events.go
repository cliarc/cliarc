package events

import (
	"context"
	"sync"
	"time"
)

// Event represents a system event.
type Event struct {
	Type      string                 `json:"type"`
	Source    string                 `json:"source"`
	Timestamp time.Time              `json:"timestamp"`
	Payload   map[string]interface{} `json:"payload"`
}

// Handler is a function that processes events.
type Handler func(ctx context.Context, event Event) error

// Bus is an in-memory event bus.
type Bus struct {
	mu       sync.RWMutex
	handlers map[string][]Handler
}

// NewBus creates an event bus.
func NewBus() *Bus {
	return &Bus{
		handlers: make(map[string][]Handler),
	}
}

// Subscribe registers a handler for an event type.
func (b *Bus) Subscribe(eventType string, h Handler) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.handlers[eventType] = append(b.handlers[eventType], h)
}

// Publish sends an event to all subscribed handlers.
func (b *Bus) Publish(ctx context.Context, event Event) {
	b.mu.RLock()
	hs := make([]Handler, len(b.handlers[event.Type]))
	copy(hs, b.handlers[event.Type])
	b.mu.RUnlock()

	for _, h := range hs {
		go func(handler Handler) {
			ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
			defer cancel()
			_ = handler(ctx, event)
		}(h)
	}
}

// Common event types
const (
	EventPluginDiscovered  = "plugin.discovered"
	EventPluginStarted     = "plugin.started"
	EventPluginStopped     = "plugin.stopped"
	EventPluginCrashed     = "plugin.crashed"
	EventPluginHealthFail  = "plugin.health_fail"
	EventServerAdded       = "server.added"
	EventServerRemoved     = "server.removed"
	EventPermissionDenied  = "permission.denied"
)
