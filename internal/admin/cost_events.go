package admin

// Outbox event type constants for cost attribution (NNR-104).
// Cost events flow through the transactional outbox so that hot-path
// activities never perform blocking writes to ledger tables.
const (
	// EventTypeLLMCost is enqueued when an LLM call completes.
	EventTypeLLMCost = "BREVIO.cost.llm.v1"

	// EventTypeConnectorCost is enqueued when a connector call completes.
	EventTypeConnectorCost = "BREVIO.cost.connector.v1"

	// EventTypeToolCost is enqueued when a tool execution completes.
	EventTypeToolCost = "BREVIO.cost.tool.v1"

	// EventTypeSubscription is enqueued when a Stripe webhook arrives.
	EventTypeSubscription = "BREVIO.subscription.event.v1"

	// AggregateTypeCost is the outbox aggregate type for cost events.
	AggregateTypeCost = "cost"

	// AggregateTypeSubscription is the outbox aggregate type for subscription events.
	AggregateTypeSubscription = "subscription"
)

// LLMCostEvent is the outbox payload schema for EventTypeLLMCost.
type LLMCostEvent struct {
	WorkspaceID  string  `json:"workspace_id"`
	UserID       string  `json:"user_id"`
	WorkflowRunID string `json:"workflow_run_id"`
	Provider     string  `json:"provider"`
	Model        string  `json:"model"`
	TokensInput  int     `json:"tokens_input"`
	TokensOutput int     `json:"tokens_output"`
	CostUSD      float64 `json:"cost_usd"`
	LatencyMs    int     `json:"latency_ms"`
	CacheHit     bool    `json:"cache_hit"`
}

// ConnectorCostEvent is the outbox payload schema for EventTypeConnectorCost.
type ConnectorCostEvent struct {
	WorkspaceID   string  `json:"workspace_id"`
	UserID        string  `json:"user_id"`
	WorkflowRunID string  `json:"workflow_run_id"`
	ConnectorID   string  `json:"connector_id"`
	ConnectorName string  `json:"connector_name"`
	Operation     string  `json:"operation"`
	CostUSD       float64 `json:"cost_usd"`
	LatencyMs     int     `json:"latency_ms"`
}

// SubscriptionEventPayload is the outbox payload schema for EventTypeSubscription.
type SubscriptionEventPayload struct {
	WorkspaceID   string  `json:"workspace_id"`
	StripeEventID string  `json:"stripe_event_id"`
	EventType     string  `json:"event_type"`
	Amount        float64 `json:"amount"`
	Currency      string  `json:"currency"`
}
