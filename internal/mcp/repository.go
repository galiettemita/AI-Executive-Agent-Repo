package mcp

import "context"

// ToolRepository persists MCP tool specifications.
type ToolRepository interface {
	RegisterTool(ctx context.Context, spec *ToolSpec) error
	GetTool(ctx context.Context, name string) (*ToolSpec, error)
	ListTools(ctx context.Context) ([]ToolSpec, error)
	RemoveTool(ctx context.Context, name string) error
}
