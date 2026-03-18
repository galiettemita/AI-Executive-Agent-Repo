package mcp

import (
	"context"
	"time"
)

// MCPDiscoveredServer is a registered MCP server with optional Bearer auth.
type MCPDiscoveredServer struct {
	ID        string
	BaseURL   string
	AuthToken string // optional Bearer token
	IsActive  bool
}

// MCPToolSchema represents a single discovered tool's JSON Schema.
type MCPToolSchema struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"inputSchema"`
}

// DiscoveredTool is a tool found from a remote MCP server.
type DiscoveredTool struct {
	ToolKey        string
	ServerID       string
	Schema         MCPToolSchema
	SchemaHash     string
	SchemaJSON     []byte
	DiscoveredAt   time.Time
	LastVerifiedAt time.Time
	IsActive       bool
}

// MCPServerDiscoveryRepository provides read access to MCP servers for discovery.
type MCPServerDiscoveryRepository interface {
	ListActiveServers(ctx context.Context) ([]MCPDiscoveredServer, error)
}

// MCPToolCatalogRepository provides read/write access to the tool catalog.
type MCPToolCatalogRepository interface {
	Upsert(ctx context.Context, tool DiscoveredTool) error
	GetByKey(ctx context.Context, toolKey string) (*DiscoveredTool, error)
	MarkInactiveExcept(ctx context.Context, serverID string, activeToolKeys []string) error
	ListActive(ctx context.Context) ([]DiscoveredTool, error)
}

// DangerousParamPatterns are param names that indicate a dangerous tool.
var DangerousParamPatterns = []string{
	"exec", "shell", "filesystem", "network_unrestricted", "system_command", "eval",
}
