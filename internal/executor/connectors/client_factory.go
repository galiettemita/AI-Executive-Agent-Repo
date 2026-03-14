package connectors

import (
	"context"
	"fmt"
	"time"
)

type ClientFactory struct {
	registry *Registry
	clients  map[string]ConnectorClient
}

func NewClientFactory(registry *Registry) *ClientFactory {
	if registry == nil {
		registry = NewRegistry()
	}
	return &ClientFactory{
		registry: registry,
		clients:  map[string]ConnectorClient{},
	}
}

func (f *ClientFactory) RegisterClient(connectorKey string, client ConnectorClient) error {
	if connectorKey == "" {
		return fmt.Errorf("connector_key is required")
	}
	if client == nil {
		return fmt.Errorf("client is required")
	}
	f.clients[connectorKey] = client
	return nil
}

func (f *ClientFactory) Resolve(connectorKey string) (ConnectorClient, error) {
	client, ok := f.clients[connectorKey]
	if !ok {
		return nil, fmt.Errorf("connector client not found: %s", connectorKey)
	}
	return client, nil
}

// TestOnlyNoopClient is a test-only stub that returns success for all operations.
// MUST NOT be used in production — use ClientFactory.Resolve() which returns an
// error for unregistered connectors. This type exists solely for unit tests.
type TestOnlyNoopClient struct{}

func (TestOnlyNoopClient) Simulate(_ context.Context, req ToolCallRequest) (SimulateResult, error) {
	return SimulateResult{
		Success:        true,
		Preview:        map[string]any{"tool_key": req.ToolKey},
		EstimatedRisk:  "low",
		EstimatedCost:  0,
		EstimatedLatMS: 50,
	}, nil
}

func (TestOnlyNoopClient) Commit(_ context.Context, req ToolCallRequest) (CommitResult, error) {
	return CommitResult{
		Success:     true,
		ReceiptID:   req.IdempotencyKey,
		Output:      map[string]any{"status": "ok", "tool_key": req.ToolKey},
		CommittedAt: time.Now().UTC().Format(time.RFC3339),
	}, nil
}

func (TestOnlyNoopClient) HealthCheck(_ context.Context, _ string) (HealthResult, error) {
	return HealthResult{
		Reachable:     true,
		Authenticated: true,
		Status:        "healthy",
	}, nil
}
