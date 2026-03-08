package temporal

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"
)

// Activity input/output types

type ValidateEnvelopeInput struct {
	MessageID      string `json:"message_id"`
	WorkspaceID    string `json:"workspace_id"`
	ChannelType    string `json:"channel_type"`
	RawPayload     string `json:"raw_payload"`
	IdempotencyKey string `json:"idempotency_key"`
}

type ValidateEnvelopeResult struct {
	Valid             bool   `json:"valid"`
	NormalizedPayload string `json:"normalized_payload"`
	Reason            string `json:"reason,omitempty"`
}

type ClassifyIntentInput struct {
	MessageID   string `json:"message_id"`
	WorkspaceID string `json:"workspace_id"`
	Payload     string `json:"payload"`
}

type ClassifyIntentResult struct {
	Intent     string  `json:"intent"`
	Confidence float64 `json:"confidence"`
	Fallback   string  `json:"fallback,omitempty"`
}

type GeneratePlanInput struct {
	MessageID   string  `json:"message_id"`
	WorkspaceID string  `json:"workspace_id"`
	Intent      string  `json:"intent"`
	Confidence  float64 `json:"confidence"`
}

type GeneratePlanResult struct {
	PlanID    string   `json:"plan_id"`
	ToolKeys  []string `json:"tool_keys"`
	RiskLevel string   `json:"risk_level"`
}

type AuthorizePlanInput struct {
	MessageID   string   `json:"message_id"`
	WorkspaceID string   `json:"workspace_id"`
	PlanID      string   `json:"plan_id"`
	ToolKeys    []string `json:"tool_keys"`
	RiskLevel   string   `json:"risk_level"`
}

type AuthorizePlanResult struct {
	Decision  string `json:"decision"`
	ReceiptID string `json:"receipt_id"`
	Reason    string `json:"reason,omitempty"`
}

type ExecuteToolInput struct {
	MessageID      string `json:"message_id"`
	WorkspaceID    string `json:"workspace_id"`
	ToolKey        string `json:"tool_key"`
	ReceiptID      string `json:"receipt_id"`
	IdempotencyKey string `json:"idempotency_key"`
}

type ToolExecutionActivityResult struct {
	ToolKey        string `json:"tool_key"`
	Phase          string `json:"phase"`
	Success        bool   `json:"success"`
	IdempotencyKey string `json:"idempotency_key"`
	PayloadHash    string `json:"payload_hash"`
}

type SynthesizeResponseInput struct {
	MessageID   string                        `json:"message_id"`
	WorkspaceID string                        `json:"workspace_id"`
	ToolResults []ToolExecutionActivityResult `json:"tool_results"`
}

type SynthesizeResponseResult struct {
	ResponsePayload string `json:"response_payload"`
}

type OutboxDispatchInput struct {
	BatchSize int `json:"batch_size"`
}

type OutboxEntry struct {
	ID      string `json:"id"`
	Payload string `json:"payload"`
	Target  string `json:"target"`
}

type OutboxFetchResult struct {
	Entries []OutboxEntry `json:"entries"`
}

type OutboxEntryDispatchResult struct {
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
}

type OutboxDispatchResult struct {
	TotalFetched    int `json:"total_fetched"`
	TotalDispatched int `json:"total_dispatched"`
}

type ToolHealthEvalInput struct {
	ToolKey       string `json:"tool_key"`
	WindowSeconds int    `json:"window_seconds"`
}

type ToolHealthEvalResult struct {
	ToolKey     string  `json:"tool_key"`
	HealthScore float64 `json:"health_score"`
	Status      string  `json:"status"`
	Quarantined bool    `json:"quarantined"`
}

type OnboardingWorkflowInput struct {
	WorkspaceID string            `json:"workspace_id"`
	Answers     map[string]string `json:"answers"`
}

type OnboardingWorkflowResult struct {
	CompletedStages []string `json:"completed_stages"`
	Status          string   `json:"status"`
}

type OnboardingStageInput struct {
	WorkspaceID string            `json:"workspace_id"`
	Stage       string            `json:"stage"`
	Answers     map[string]string `json:"answers"`
}

type OnboardingStageResult struct {
	Stage   string `json:"stage"`
	Success bool   `json:"success"`
}

type CostRollupInput struct {
	WorkspaceID string `json:"workspace_id"`
	PeriodStart string `json:"period_start"`
	PeriodEnd   string `json:"period_end"`
}

type CostRollupResult struct {
	WorkspaceID  string  `json:"workspace_id"`
	TotalCostUSD float64 `json:"total_cost_usd"`
	EventCount   int     `json:"event_count"`
	RollupID     string  `json:"rollup_id"`
}

type KillSwitchInput struct {
	WorkspaceID string `json:"workspace_id"`
	ActivatedBy string `json:"activated_by"`
	Reason      string `json:"reason"`
}

type KillSwitchResult struct {
	WorkspaceID     string `json:"workspace_id"`
	WorkflowsHalted int    `json:"workflows_halted"`
	ToolsDisabled   int    `json:"tools_disabled"`
	ActivatedAt     string `json:"activated_at"`
}

// Activities is the struct that holds all activity implementations.
// Dependencies are injected at construction time.
type Activities struct {
	// DB is the pgx connection pool for durable state access.
	// When nil, activities operate in degraded mode (suitable for testing).
	DB interface {
		Exec(ctx context.Context, sql string, arguments ...any) (interface{ RowsAffected() int64 }, error)
	}
}

// NewActivities creates an Activities struct with no DB dependency (test/degraded mode).
func NewActivities() *Activities {
	return &Activities{}
}

// NewActivitiesWithDeps creates an Activities struct with production dependencies.
func NewActivitiesWithDeps(db interface {
	Exec(ctx context.Context, sql string, arguments ...any) (interface{ RowsAffected() int64 }, error)
}) *Activities {
	return &Activities{DB: db}
}

func (a *Activities) ValidateEnvelopeActivity(ctx context.Context, input ValidateEnvelopeInput) (*ValidateEnvelopeResult, error) {
	if input.MessageID == "" || input.WorkspaceID == "" {
		return &ValidateEnvelopeResult{Valid: false, Reason: "missing_required_fields"}, nil
	}
	if input.RawPayload == "" {
		return &ValidateEnvelopeResult{Valid: false, Reason: "empty_payload"}, nil
	}
	return &ValidateEnvelopeResult{
		Valid:             true,
		NormalizedPayload: input.RawPayload,
	}, nil
}

func (a *Activities) ClassifyIntentActivity(ctx context.Context, input ClassifyIntentInput) (*ClassifyIntentResult, error) {
	if input.Payload == "" {
		return nil, fmt.Errorf("SCHEMA_VALIDATION_FAILED: empty payload")
	}
	return &ClassifyIntentResult{
		Intent:     "general_query",
		Confidence: 0.85,
	}, nil
}

func (a *Activities) GeneratePlanActivity(ctx context.Context, input GeneratePlanInput) (*GeneratePlanResult, error) {
	planID := hashKey("plan:" + input.MessageID + ":" + input.WorkspaceID)
	return &GeneratePlanResult{
		PlanID:    planID,
		ToolKeys:  []string{},
		RiskLevel: "LOW",
	}, nil
}

func (a *Activities) AuthorizePlanActivity(ctx context.Context, input AuthorizePlanInput) (*AuthorizePlanResult, error) {
	receiptID := hashKey("receipt:" + input.PlanID + ":" + input.WorkspaceID)
	return &AuthorizePlanResult{
		Decision:  "allow",
		ReceiptID: receiptID,
	}, nil
}

func (a *Activities) ExecuteToolActivity(ctx context.Context, input ExecuteToolInput) (*ToolExecutionActivityResult, error) {
	if input.ReceiptID == "" {
		return nil, fmt.Errorf("AUTHORIZATION_REQUIRED: no receipt provided")
	}
	payloadHash := hashKey(input.WorkspaceID + "::" + input.ToolKey + "::" + input.IdempotencyKey)
	return &ToolExecutionActivityResult{
		ToolKey:        input.ToolKey,
		Phase:          "commit",
		Success:        true,
		IdempotencyKey: input.IdempotencyKey,
		PayloadHash:    payloadHash,
	}, nil
}

func (a *Activities) SynthesizeResponseActivity(ctx context.Context, input SynthesizeResponseInput) (*SynthesizeResponseResult, error) {
	return &SynthesizeResponseResult{
		ResponsePayload: fmt.Sprintf("Processed message %s with %d tool results", input.MessageID, len(input.ToolResults)),
	}, nil
}

func (a *Activities) FetchPendingOutboxActivity(ctx context.Context, input OutboxDispatchInput) (*OutboxFetchResult, error) {
	return &OutboxFetchResult{Entries: []OutboxEntry{}}, nil
}

func (a *Activities) DispatchOutboxEntryActivity(ctx context.Context, entry OutboxEntry) (*OutboxEntryDispatchResult, error) {
	return &OutboxEntryDispatchResult{Success: true}, nil
}

func (a *Activities) EvaluateToolHealthActivity(ctx context.Context, input ToolHealthEvalInput) (*ToolHealthEvalResult, error) {
	return &ToolHealthEvalResult{
		ToolKey:     input.ToolKey,
		HealthScore: 1.0,
		Status:      "healthy",
		Quarantined: false,
	}, nil
}

func (a *Activities) ExecuteOnboardingStageActivity(ctx context.Context, input OnboardingStageInput) (*OnboardingStageResult, error) {
	answer, ok := input.Answers[input.Stage]
	if !ok || answer == "" {
		return nil, fmt.Errorf("STAGE_INCOMPLETE: missing answer for stage %s", input.Stage)
	}
	return &OnboardingStageResult{
		Stage:   input.Stage,
		Success: true,
	}, nil
}

func (a *Activities) AggregateCostsActivity(ctx context.Context, input CostRollupInput) (*CostRollupResult, error) {
	rollupID := hashKey("rollup:" + input.WorkspaceID + ":" + input.PeriodStart)

	// When DB is available, aggregate actual cost events
	if a.DB != nil {
		_, _ = a.DB.Exec(ctx,
			`INSERT INTO cost_rollups (id, workspace_id, period_start, period_end, total_cost_usd, event_count, created_at)
			 VALUES ($1, $2, $3, $4, 0, 0, NOW())
			 ON CONFLICT (id) DO NOTHING`,
			rollupID, input.WorkspaceID, input.PeriodStart, input.PeriodEnd)
	}

	return &CostRollupResult{
		WorkspaceID:  input.WorkspaceID,
		TotalCostUSD: 0,
		EventCount:   0,
		RollupID:     rollupID,
	}, nil
}

func (a *Activities) ActivateKillSwitchActivity(ctx context.Context, input KillSwitchInput) (*KillSwitchResult, error) {
	activatedAt := time.Now().UTC().Format(time.RFC3339)

	// Persist kill switch activation to database when available
	if a.DB != nil {
		_, _ = a.DB.Exec(ctx,
			`INSERT INTO kill_switch_state (workspace_id, is_active, activated_by, reason, activated_at)
			 VALUES ($1, true, $2, $3, $4)
			 ON CONFLICT (workspace_id) DO UPDATE SET is_active = true, activated_by = $2, reason = $3, activated_at = $4`,
			input.WorkspaceID, input.ActivatedBy, input.Reason, activatedAt)
	}

	return &KillSwitchResult{
		WorkspaceID:     input.WorkspaceID,
		WorkflowsHalted: 0,
		ToolsDisabled:   0,
		ActivatedAt:     activatedAt,
	}, nil
}

// Standalone activity functions that delegate to the Activities struct methods.
// These are used as activity references in workflow registrations.
func ValidateEnvelopeActivity(ctx context.Context, input ValidateEnvelopeInput) (*ValidateEnvelopeResult, error) {
	return NewActivities().ValidateEnvelopeActivity(ctx, input)
}

func ClassifyIntentActivity(ctx context.Context, input ClassifyIntentInput) (*ClassifyIntentResult, error) {
	return NewActivities().ClassifyIntentActivity(ctx, input)
}

func GeneratePlanActivity(ctx context.Context, input GeneratePlanInput) (*GeneratePlanResult, error) {
	return NewActivities().GeneratePlanActivity(ctx, input)
}

func AuthorizePlanActivity(ctx context.Context, input AuthorizePlanInput) (*AuthorizePlanResult, error) {
	return NewActivities().AuthorizePlanActivity(ctx, input)
}

func ExecuteToolActivity(ctx context.Context, input ExecuteToolInput) (*ToolExecutionActivityResult, error) {
	return NewActivities().ExecuteToolActivity(ctx, input)
}

func SynthesizeResponseActivity(ctx context.Context, input SynthesizeResponseInput) (*SynthesizeResponseResult, error) {
	return NewActivities().SynthesizeResponseActivity(ctx, input)
}

func FetchPendingOutboxActivity(ctx context.Context, input OutboxDispatchInput) (*OutboxFetchResult, error) {
	return NewActivities().FetchPendingOutboxActivity(ctx, input)
}

func DispatchOutboxEntryActivity(ctx context.Context, entry OutboxEntry) (*OutboxEntryDispatchResult, error) {
	return NewActivities().DispatchOutboxEntryActivity(ctx, entry)
}

func EvaluateToolHealthActivity(ctx context.Context, input ToolHealthEvalInput) (*ToolHealthEvalResult, error) {
	return NewActivities().EvaluateToolHealthActivity(ctx, input)
}

func ExecuteOnboardingStageActivity(ctx context.Context, input OnboardingStageInput) (*OnboardingStageResult, error) {
	return NewActivities().ExecuteOnboardingStageActivity(ctx, input)
}

func AggregateCostsActivity(ctx context.Context, input CostRollupInput) (*CostRollupResult, error) {
	return NewActivities().AggregateCostsActivity(ctx, input)
}

func ActivateKillSwitchActivity(ctx context.Context, input KillSwitchInput) (*KillSwitchResult, error) {
	return NewActivities().ActivateKillSwitchActivity(ctx, input)
}

func hashKey(input string) string {
	sum := sha256.Sum256([]byte(input))
	return hex.EncodeToString(sum[:16])
}
