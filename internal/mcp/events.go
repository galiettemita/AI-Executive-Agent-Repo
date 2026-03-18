package mcp

import (
	"context"
	"time"
)

// MCPToolChangedEvent is emitted when a tool's schema is added, updated, or removed.
type MCPToolChangedEvent struct {
	ToolKey    string    `json:"tool_key"`
	ServerID   string    `json:"server_id"`
	ChangeType string    `json:"change_type"` // "added", "updated", "removed"
	Timestamp  time.Time `json:"timestamp"`
}

// EventPublisher publishes domain events to the application event bus.
type EventPublisher interface {
	Publish(ctx context.Context, event interface{}) error
}

// NoopEventPublisher is a no-op event publisher for use when no bus is configured.
type NoopEventPublisher struct{}

// Publish does nothing.
func (n *NoopEventPublisher) Publish(_ context.Context, _ interface{}) error { return nil }
