package connectors

import "context"

type ToolCallRequest struct {
	WorkspaceID    string
	ConnectorKey   string
	ToolKey        string
	Arguments      map[string]any
	IdempotencyKey string
}

type SimulateResult struct {
	Success        bool
	Preview        map[string]any
	EstimatedRisk  string
	EstimatedCost  float64
	EstimatedLatMS int
}

type CommitResult struct {
	Success     bool
	ReceiptID   string
	Output      map[string]any
	CommittedAt string
}

type HealthResult struct {
	Reachable     bool
	Authenticated bool
	Status        string
}

// ConnectorClient is the addendum contract implemented by native and MCP clients.
type ConnectorClient interface {
	Simulate(ctx context.Context, req ToolCallRequest) (SimulateResult, error)
	Commit(ctx context.Context, req ToolCallRequest) (CommitResult, error)
	HealthCheck(ctx context.Context, workspaceID string) (HealthResult, error)
}
