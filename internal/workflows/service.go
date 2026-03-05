package workflows

import (
	"context"
	"crypto/sha256"
	"encoding/base32"
	"encoding/hex"
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
	"time"
)

type WorkflowStep struct {
	StepKey string
	Status  string
}

type InteractiveResult struct {
	FinalState string
	Steps      []WorkflowStep
}

type ProvisioningResult struct {
	Status           string
	ExecutedSteps    []string
	CompensatedSteps []string
}

type OnboardingResult struct {
	CompletedStages []string
	Status          string
}

type ToolExecutionResult struct {
	IdempotencyKey string
	PayloadHash    string
	CreatedAt      time.Time
}

type TrustEvalResult struct {
	TrustScore         float64
	PromotionEligible  bool
	SuccessCount30d    int
	FailureCount30d    int
	OverrideCount30d   int
	Trailing14dFailure int
}

type ReasoningConstraints struct {
	PlannerRetryLimit int
	CriticRetryLimit  int
	ExecutorLoopLimit int
	MaxPlanCandidates int
	RequestedTier     string
	ResolvedTier      string
}

type ReActExecutionResult struct {
	RequestedTier  string   `json:"requested_tier"`
	ResolvedTier   string   `json:"resolved_tier"`
	MaxSteps       int      `json:"max_steps"`
	StepsExecuted  int      `json:"steps_executed"`
	EarlyExit      bool     `json:"early_exit"`
	ExitReason     string   `json:"exit_reason"`
	PartialResults []string `json:"partial_results"`
}

type PlanCandidate struct {
	ID                  string
	ToolKeys            []string
	NumTools            int
	MaxToolsForTier     int
	MaxToolRisk         int
	EstimatedCost       float64
	BudgetRemaining     float64
	EstimatedLatencyMS  int
	TierSLOMS           int
	CapabilityStrengths []int
}

type PlanScoreWeights struct {
	ToolCountScoreWeight            float64
	RiskScoreWeight                 float64
	CostScoreWeight                 float64
	LatencyScoreWeight              float64
	CapabilityConfidenceScoreWeight float64
}

type ActivityRetryPolicy struct {
	StartToCloseTimeout time.Duration
	MaxAttempts         int
	BackoffCoefficient  float64
	NonRetryableErrors  []string
}

type RetryDefaults struct {
	InitialInterval    time.Duration
	BackoffCoefficient float64
	MaximumInterval    time.Duration
}

func DefaultPlanScoreWeights() PlanScoreWeights {
	return PlanScoreWeights{
		ToolCountScoreWeight:            0.15,
		RiskScoreWeight:                 0.30,
		CostScoreWeight:                 0.20,
		LatencyScoreWeight:              0.15,
		CapabilityConfidenceScoreWeight: 0.20,
	}
}

func InteractiveTurnActivityPolicies(tier string) map[string]ActivityRetryPolicy {
	planGenerationTimeout := 120 * time.Second
	if strings.EqualFold(strings.TrimSpace(tier), "T1") {
		planGenerationTimeout = 60 * time.Second
	}

	return map[string]ActivityRetryPolicy{
		"context_assembly": {
			StartToCloseTimeout: 10 * time.Second,
			MaxAttempts:         2,
			BackoffCoefficient:  2.0,
			NonRetryableErrors:  []string{"WORKSPACE_NOT_FOUND", "USER_SUSPENDED"},
		},
		"plan_generation": {
			StartToCloseTimeout: planGenerationTimeout,
			MaxAttempts:         2,
			BackoffCoefficient:  2.0,
			NonRetryableErrors:  []string{"ATTENTION_BUDGET_EXHAUSTED", "SCHEMA_VALIDATION_FAILED"},
		},
		"policy_evaluation": {
			StartToCloseTimeout: 5 * time.Second,
			MaxAttempts:         3,
			BackoffCoefficient:  2.0,
			NonRetryableErrors:  []string{"POLICY_ENGINE_UNAVAILABLE"},
		},
		"tool_simulate": {
			StartToCloseTimeout: 30 * time.Second,
			MaxAttempts:         2,
			BackoffCoefficient:  2.0,
			NonRetryableErrors:  []string{"TOOL_NOT_FOUND", "AUTH_EXPIRED", "AUTONOMY_INSUFFICIENT"},
		},
		"tool_commit": {
			StartToCloseTimeout: 30 * time.Second,
			MaxAttempts:         3,
			BackoffCoefficient:  2.0,
			NonRetryableErrors:  []string{"IDEMPOTENCY_CONFLICT", "AUTH_EXPIRED", "BUDGET_EXHAUSTED"},
		},
		"trust_receipt_emit": {
			StartToCloseTimeout: 5 * time.Second,
			MaxAttempts:         3,
			BackoffCoefficient:  2.0,
			NonRetryableErrors:  []string{},
		},
		"response_synthesis": {
			StartToCloseTimeout: 60 * time.Second,
			MaxAttempts:         2,
			BackoffCoefficient:  2.0,
			NonRetryableErrors:  []string{"ATTENTION_BUDGET_EXHAUSTED"},
		},
	}
}

func ProvisioningActivityPolicies() map[string]ActivityRetryPolicy {
	return map[string]ActivityRetryPolicy{
		"preflight": {
			StartToCloseTimeout: 10 * time.Second,
			MaxAttempts:         3,
			BackoffCoefficient:  2.0,
			NonRetryableErrors:  []string{"DUPLICATE_REQUEST"},
		},
		"policy_gate": {
			StartToCloseTimeout: 10 * time.Second,
			MaxAttempts:         2,
			BackoffCoefficient:  2.0,
			NonRetryableErrors:  []string{"POLICY_DENY"},
		},
		"allocate_server": {
			StartToCloseTimeout: 60 * time.Second,
			MaxAttempts:         3,
			BackoffCoefficient:  2.0,
			NonRetryableErrors:  []string{"NO_CAPACITY"},
		},
		"oauth_flow": {
			StartToCloseTimeout: 600 * time.Second,
			MaxAttempts:         1,
			BackoffCoefficient:  2.0,
			NonRetryableErrors:  []string{"USER_DENIED", "OAUTH_TIMEOUT"},
		},
		"deploy_server": {
			StartToCloseTimeout: 120 * time.Second,
			MaxAttempts:         3,
			BackoffCoefficient:  2.0,
			NonRetryableErrors:  []string{"ARTIFACT_VERIFICATION_FAILED"},
		},
		"fetch_schemas": {
			StartToCloseTimeout: 30 * time.Second,
			MaxAttempts:         3,
			BackoffCoefficient:  2.0,
			NonRetryableErrors:  []string{},
		},
		"health_check": {
			StartToCloseTimeout: 15 * time.Second,
			MaxAttempts:         3,
			BackoffCoefficient:  2.0,
			NonRetryableErrors:  []string{},
		},
		"commit_registry": {
			StartToCloseTimeout: 10 * time.Second,
			MaxAttempts:         3,
			BackoffCoefficient:  2.0,
			NonRetryableErrors:  []string{},
		},
	}
}

func CommonRetryDefaults() RetryDefaults {
	return RetryDefaults{
		InitialInterval:    1 * time.Second,
		BackoffCoefficient: 2.0,
		MaximumInterval:    60 * time.Second,
	}
}

func DriftWatchdogCadence() map[string]time.Duration {
	return map[string]time.Duration{
		"health_check":         5 * time.Minute,
		"schema_snapshot_diff": 1 * time.Hour,
		"deep_health_check":    24 * time.Hour,
	}
}

func DelegationPairingFlowSteps() []string {
	return []string{
		"owner_requests_delegate_pairing",
		"brain_proposes_delegation_permissions",
		"owner_confirms_pairing_request",
		"generate_pairing_code",
		"create_pairing_invitation_record",
		"deliver_code_to_owner",
		"delegate_submits_code",
		"validate_pairing_code_and_expiry",
		"create_or_lookup_delegate_user",
		"create_delegation_grant_record",
		"create_delegate_channel_binding",
		"emit_paired_event_and_mark_consumed",
	}
}

type TriggerSpec struct {
	WorkflowID string
	Trigger    string
}

type WorkflowInstanceMirror struct {
	WorkflowKey string
	Status      string
	UpdatedAt   time.Time
}

type WorkflowStepMirror struct {
	StepKey        string
	Status         string
	IdempotencyKey string
	UpdatedAt      time.Time
}

type TwoPhaseExecutionResult struct {
	Simulate ToolExecutionResult
	Commit   ToolExecutionResult
	Replayed bool
}

type Service struct {
	mu                sync.Mutex
	idempotencyStore  map[string]ToolExecutionResult
	workflowInstances map[string]WorkflowInstanceMirror
	workflowSteps     map[string][]WorkflowStepMirror
	twoPhaseStore     map[string]TwoPhaseExecutionResult
	idempotencyTTL    time.Duration
	dailyCaptureDates map[string]struct{}
	dailyLogTurns     map[string]struct{}
}

// Supported workflow IDs for closure mapping:
// message_processing_v1, daily_rhythm_v1, interactive_turn_v1,
// provisioning_v9, onboarding_v1, drift_watchdog_v1,
// outbox_hold_worker, memory_consolidation, daily_capture_v1,
// daily_log_capture_v1, goal_review_v1, trust_eval_v1,
// learning_consolidation_v1, capability_exploration_v1,
// cross_repo_analysis_v1, mission_control_refresh_v1, rag_ingest_v1,
// rag_eval_v1, tool_health_eval_v1, memory_conflict_resolve_v1,
// compliance_evidence_v1, admin_kpi_report_v1, admin_alert_eval_v1,
// cache_maintenance_v1.
func NewService() *Service {
	return &Service{
		idempotencyStore:  map[string]ToolExecutionResult{},
		workflowInstances: map[string]WorkflowInstanceMirror{},
		workflowSteps:     map[string][]WorkflowStepMirror{},
		twoPhaseStore:     map[string]TwoPhaseExecutionResult{},
		idempotencyTTL:    30 * 24 * time.Hour,
		dailyCaptureDates: map[string]struct{}{},
		dailyLogTurns:     map[string]struct{}{},
	}
}

func V91WorkflowTriggerSpecs() map[string]TriggerSpec {
	return map[string]TriggerSpec{
		"daily_capture_v1": {
			WorkflowID: "daily_capture_v1",
			Trigger:    "end_of_last_session_each_day_or_cron_if_no_session_by_configured_time",
		},
		"daily_log_capture_v1": {
			WorkflowID: "daily_log_capture_v1",
			Trigger:    "after_each_interactive_turn_v1_completion",
		},
		"goal_review_v1": {
			WorkflowID: "goal_review_v1",
			Trigger:    "mission_control_refresh_or_cron_default_weekly",
		},
		"trust_eval_v1": {
			WorkflowID: "trust_eval_v1",
			Trigger:    "daily_03_00_utc_or_after_operator_override_event",
		},
		"learning_consolidation_v1": {
			WorkflowID: "learning_consolidation_v1",
			Trigger:    "weekly_or_pending_feedback_gt_20",
		},
		"capability_exploration_v1": {
			WorkflowID: "capability_exploration_v1",
			Trigger:    "monthly_or_capability_gap_events_gte_3_within_7d",
		},
		"cross_repo_analysis_v1": {
			WorkflowID: "cross_repo_analysis_v1",
			Trigger:    "after_codebase_map_ingestion_v1_or_operator_request",
		},
		"mission_control_refresh_v1": {
			WorkflowID: "mission_control_refresh_v1",
			Trigger:    "cron_per_mission_control_config_refresh_cadence_minutes",
		},
	}
}

func (s *Service) recordWorkflowInstance(workflowKey, status string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.workflowInstances[workflowKey] = WorkflowInstanceMirror{
		WorkflowKey: workflowKey,
		Status:      status,
		UpdatedAt:   time.Now().UTC(),
	}
}

func (s *Service) appendWorkflowStep(workflowKey, stepKey, status, idempotencyKey string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.workflowSteps[workflowKey] = append(s.workflowSteps[workflowKey], WorkflowStepMirror{
		StepKey:        stepKey,
		Status:         status,
		IdempotencyKey: idempotencyKey,
		UpdatedAt:      time.Now().UTC(),
	})
}

func (s *Service) WorkflowInstance(workflowKey string) (WorkflowInstanceMirror, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	instance, ok := s.workflowInstances[workflowKey]
	return instance, ok
}

func (s *Service) WorkflowSteps(workflowKey string) []WorkflowStepMirror {
	s.mu.Lock()
	defer s.mu.Unlock()
	steps := s.workflowSteps[workflowKey]
	out := make([]WorkflowStepMirror, len(steps))
	copy(out, steps)
	return out
}

func (s *Service) InteractiveTurnV1(_ context.Context, message string) InteractiveResult {
	s.recordWorkflowInstance("interactive_turn_v1", "running")
	constraints := ReasoningConstraintsForTier("T2")
	steps := []WorkflowStep{{StepKey: "planner", Status: "completed"}}
	s.appendWorkflowStep("interactive_turn_v1", "planner", "completed", "")
	if message == "" {
		s.appendWorkflowStep("interactive_turn_v1", "terminal", "completed", "")
		s.recordWorkflowInstance("interactive_turn_v1", "completed")
		return InteractiveResult{FinalState: "TERMINAL", Steps: append(steps, WorkflowStep{StepKey: "terminal", Status: "completed"})}
	}
	for i := 0; i < minInt(1, constraints.ExecutorLoopLimit); i++ {
		steps = append(steps, WorkflowStep{StepKey: "executor", Status: "completed"})
		s.appendWorkflowStep("interactive_turn_v1", "executor", "completed", "")
	}
	steps = append(steps,
		WorkflowStep{StepKey: "critic", Status: "completed"},
		WorkflowStep{StepKey: "reflector", Status: "completed"},
		WorkflowStep{StepKey: "terminal", Status: "completed"},
	)
	s.appendWorkflowStep("interactive_turn_v1", "critic", "completed", "")
	s.appendWorkflowStep("interactive_turn_v1", "reflector", "completed", "")
	s.appendWorkflowStep("interactive_turn_v1", "terminal", "completed", "")
	s.recordWorkflowInstance("interactive_turn_v1", "completed")
	return InteractiveResult{FinalState: "TERMINAL", Steps: steps}
}

func (s *Service) ProvisioningV9(_ context.Context, failAt string) ProvisioningResult {
	s.recordWorkflowInstance("provisioning_v9", "running")
	steps := []string{
		"Preflight",
		"CreateRequest",
		"PolicyGate",
		"AllocateOrReuseServer",
		"VerifyArtifact",
		"DeployServer",
		"FetchToolSchemas",
		"HealthCheck",
		"CommitRegistry",
		"Active",
	}

	executed := []string{}
	compensated := []string{}
	for _, step := range steps {
		stepID := formatIdempotencyKey("provisioning_v9::" + step)
		s.appendWorkflowStep("provisioning_v9", step, "running", stepID)
		executed = append(executed, step)
		if failAt != "" && step == failAt {
			s.appendWorkflowStep("provisioning_v9", step, "failed", stepID)
			for i := len(executed) - 1; i >= 0; i-- {
				compensated = append(compensated, executed[i])
				s.appendWorkflowStep("provisioning_v9", "compensate_"+executed[i], "completed", formatIdempotencyKey("provisioning_v9::compensate::"+executed[i]))
			}
			s.recordWorkflowInstance("provisioning_v9", "failed")
			return ProvisioningResult{Status: "failed", ExecutedSteps: executed, CompensatedSteps: compensated}
		}
		s.appendWorkflowStep("provisioning_v9", step, "completed", stepID)
	}
	s.recordWorkflowInstance("provisioning_v9", "active")
	return ProvisioningResult{Status: "active", ExecutedSteps: executed}
}

func (s *Service) OnboardingV1(_ context.Context, answers map[string]string) OnboardingResult {
	s.recordWorkflowInstance("onboarding_v1", "running")
	stages := []string{
		"operator_profile_intake_v1",
		"behavior_policy_calibration_v1",
		"codebase_map_ingestion_v1",
		"system_map_ingestion_v1",
	}
	for _, stage := range stages {
		if answers[stage] == "" {
			s.appendWorkflowStep("onboarding_v1", stage, "failed", "")
			s.recordWorkflowInstance("onboarding_v1", "incomplete")
			return OnboardingResult{CompletedStages: []string{}, Status: "incomplete"}
		}
		s.appendWorkflowStep("onboarding_v1", stage, "completed", formatIdempotencyKey("onboarding_v1::"+stage))
	}
	s.recordWorkflowInstance("onboarding_v1", "completed")
	return OnboardingResult{CompletedStages: stages, Status: "completed"}
}

func (s *Service) DriftWatchdogV1(_ context.Context, hasDrift bool) string {
	if hasDrift {
		return "quarantined"
	}
	return "healthy"
}

func (s *Service) OutboxHoldWorker(_ context.Context, holdUntil time.Time) string {
	if time.Now().UTC().Before(holdUntil) {
		return "held"
	}
	return "sent"
}

func (s *Service) MemoryConsolidation(_ context.Context, entries []string) []string {
	unique := map[string]struct{}{}
	out := make([]string, 0, len(entries))
	for _, entry := range entries {
		if _, exists := unique[entry]; exists {
			continue
		}
		unique[entry] = struct{}{}
		out = append(out, entry)
	}
	return out
}

func (s *Service) DailyCaptureV1(_ context.Context, trigger string) string {
	if trigger == "" {
		return "skipped"
	}

	dayKey := time.Now().UTC().Format("2006-01-02")
	s.mu.Lock()
	_, exists := s.dailyCaptureDates[dayKey]
	if !exists {
		s.dailyCaptureDates[dayKey] = struct{}{}
	}
	s.mu.Unlock()
	if exists {
		return "skipped"
	}

	s.recordWorkflowInstance("daily_capture_v1", "completed")
	s.appendWorkflowStep("daily_capture_v1", "memory_item_daily_log", "completed", formatIdempotencyKey("daily_capture_v1::"+dayKey))
	return "completed"
}

func (s *Service) DailyLogCaptureV1(_ context.Context, interactiveTurnID string) string {
	if interactiveTurnID == "" {
		return "skipped"
	}

	s.mu.Lock()
	if _, exists := s.dailyLogTurns[interactiveTurnID]; !exists {
		s.dailyLogTurns[interactiveTurnID] = struct{}{}
		s.mu.Unlock()
		s.recordWorkflowInstance("daily_log_capture_v1", "captured")
		s.appendWorkflowStep("daily_log_capture_v1", "append_daily_log", "completed", formatIdempotencyKey("daily_log_capture_v1::"+interactiveTurnID))
		return "captured"
	}
	s.mu.Unlock()
	return "captured"
}

func (s *Service) GoalReviewV1(_ context.Context, stalledGoals int) string {
	if stalledGoals > 0 {
		return "stalled_detected"
	}
	return "reviewed"
}

// TrustEvalV1 implements the V9.1 deterministic trust-score formula.
func (s *Service) TrustEvalV1(_ context.Context, successCount30d, failureCount30d, overrideCount30d, trailing14dFailure int) TrustEvalResult {
	denominator := maxInt(successCount30d+failureCount30d+overrideCount30d, 1)
	score := float64(successCount30d-2*failureCount30d-3*overrideCount30d) / float64(denominator)
	score = math.Round(score*10000) / 10000

	eligible := score >= 0.85 &&
		successCount30d >= 20 &&
		trailing14dFailure == 0

	return TrustEvalResult{
		TrustScore:         score,
		PromotionEligible:  eligible,
		SuccessCount30d:    successCount30d,
		FailureCount30d:    failureCount30d,
		OverrideCount30d:   overrideCount30d,
		Trailing14dFailure: trailing14dFailure,
	}
}

func (s *Service) LearningConsolidationV1(_ context.Context, pendingFeedback int, requiresConfirmation bool) string {
	if pendingFeedback <= 0 {
		return "skipped"
	}
	if requiresConfirmation {
		return "requires_confirmation"
	}
	return "consolidated"
}

func (s *Service) CapabilityExplorationV1(_ context.Context, capabilityGapEventsLast7d int) string {
	s.recordWorkflowInstance("capability_exploration_v1", "running")
	if capabilityGapEventsLast7d >= 3 {
		s.appendWorkflowStep("capability_exploration_v1", "analyze_capability_gaps", "completed", formatIdempotencyKey("capability_exploration_v1::analyze_capability_gaps"))
		s.appendWorkflowStep("capability_exploration_v1", "emit_recommendations", "completed", formatIdempotencyKey("capability_exploration_v1::emit_recommendations"))
		s.recordWorkflowInstance("capability_exploration_v1", "completed")
		return "recommendations_created"
	}
	s.appendWorkflowStep("capability_exploration_v1", "analyze_capability_gaps", "completed", formatIdempotencyKey("capability_exploration_v1::analyze_capability_gaps"))
	s.recordWorkflowInstance("capability_exploration_v1", "no_action")
	return "no_action"
}

func (s *Service) CrossRepoAnalysisV1(_ context.Context, repositoryCount int) string {
	s.recordWorkflowInstance("cross_repo_analysis_v1", "running")
	if repositoryCount <= 1 {
		s.appendWorkflowStep("cross_repo_analysis_v1", "validate_repository_count", "skipped", "")
		s.recordWorkflowInstance("cross_repo_analysis_v1", "insufficient_repositories")
		return "insufficient_repositories"
	}
	s.appendWorkflowStep("cross_repo_analysis_v1", "collect_dependency_graph", "completed", formatIdempotencyKey("cross_repo_analysis_v1::collect_dependency_graph"))
	s.appendWorkflowStep("cross_repo_analysis_v1", "detect_shared_patterns", "completed", formatIdempotencyKey("cross_repo_analysis_v1::detect_shared_patterns"))
	s.recordWorkflowInstance("cross_repo_analysis_v1", "completed")
	return "patterns_detected"
}

func (s *Service) MissionControlRefreshV1(_ context.Context, widgetCount int) string {
	if widgetCount <= 0 {
		return "empty"
	}
	return "refreshed"
}

func (s *Service) RagIngestV1(_ context.Context, documents int) string {
	if documents <= 0 {
		return "skipped"
	}
	return "ingested"
}

func (s *Service) RagEvalV1(_ context.Context, faithfulness, relevance float64) string {
	if faithfulness >= 0.80 && relevance >= 0.75 {
		return "passed"
	}
	return "failed"
}

func (s *Service) ToolHealthEvalV1(_ context.Context, failuresLast60s int) string {
	if failuresLast60s >= 5 {
		return "quarantined"
	}
	return "healthy"
}

func (s *Service) MemoryConflictResolveV1(_ context.Context, conflictDetected bool) string {
	if conflictDetected {
		return "resolved"
	}
	return "no_conflict"
}

func (s *Service) ComplianceEvidenceV1(_ context.Context, framework string) string {
	if framework == "" {
		return "skipped"
	}
	return "collected"
}

func (s *Service) AdminKPIReportV1(_ context.Context, metricsCount int) string {
	if metricsCount <= 0 {
		return "empty"
	}
	return "generated"
}

func (s *Service) AdminAlertEvalV1(_ context.Context, thresholdBreached bool) string {
	if thresholdBreached {
		return "fired"
	}
	return "clear"
}

func (s *Service) CacheMaintenanceV1(_ context.Context, expiredEntries int) int {
	if expiredEntries <= 0 {
		return 0
	}
	return expiredEntries
}

func payloadHash(workspaceID, toolKey, logicalAction string) string {
	sum := sha256.Sum256([]byte(workspaceID + "::" + toolKey + "::" + logicalAction))
	return hex.EncodeToString(sum[:])
}

func formatIdempotencyKey(scopeHash string) string {
	sum := sha256.Sum256([]byte(scopeHash))
	encoded := strings.ToLower(base32.HexEncoding.WithPadding(base32.NoPadding).EncodeToString(sum[:]))
	if len(encoded) < 16 {
		return "idem_" + encoded
	}
	return "idem_" + encoded[:26]
}

func (s *Service) ExecuteToolWithIdempotency(workspaceID, toolKey, logicalAction string) (ToolExecutionResult, error) {
	if workspaceID == "" || toolKey == "" || logicalAction == "" {
		return ToolExecutionResult{}, fmt.Errorf("workspace_id, tool_key, and logical_action are required")
	}
	payload := payloadHash(workspaceID, toolKey, logicalAction)
	scopeKey := workspaceID + "::" + toolKey + "::" + payload
	idempotencyKey := formatIdempotencyKey(scopeKey)

	s.mu.Lock()
	defer s.mu.Unlock()
	if existing, ok := s.idempotencyStore[scopeKey]; ok {
		return existing, nil
	}

	result := ToolExecutionResult{
		IdempotencyKey: idempotencyKey,
		PayloadHash:    payload,
		CreatedAt:      time.Now().UTC(),
	}
	s.idempotencyStore[scopeKey] = result
	return result, nil
}

func (s *Service) ExecuteTwoPhaseTool(workspaceID, toolKey, logicalAction string, now time.Time) (TwoPhaseExecutionResult, error) {
	if workspaceID == "" || toolKey == "" || logicalAction == "" {
		return TwoPhaseExecutionResult{}, fmt.Errorf("workspace_id, tool_key, and logical_action are required")
	}
	payload := payloadHash(workspaceID, toolKey, logicalAction)
	scopeKey := workspaceID + "::" + toolKey + "::" + payload
	if now.IsZero() {
		now = time.Now().UTC()
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if existing, ok := s.twoPhaseStore[scopeKey]; ok {
		latest := existing.Commit.CreatedAt
		if latest.IsZero() {
			latest = existing.Simulate.CreatedAt
		}
		if now.Sub(latest) <= s.idempotencyTTL {
			existing.Replayed = true
			return existing, nil
		}
	}

	simulate := ToolExecutionResult{
		IdempotencyKey: formatIdempotencyKey(scopeKey + "::simulate"),
		PayloadHash:    payload,
		CreatedAt:      now.UTC(),
	}
	commit := ToolExecutionResult{
		IdempotencyKey: formatIdempotencyKey(scopeKey + "::commit"),
		PayloadHash:    payload,
		CreatedAt:      now.UTC(),
	}
	result := TwoPhaseExecutionResult{
		Simulate: simulate,
		Commit:   commit,
		Replayed: false,
	}
	s.twoPhaseStore[scopeKey] = result
	return result, nil
}

func ReasoningConstraintsForTier(requestedTier string) ReasoningConstraints {
	normalized := strings.ToUpper(strings.TrimSpace(requestedTier))
	resolvedTier := normalized
	constraints := ReasoningConstraints{
		PlannerRetryLimit: 1,
		CriticRetryLimit:  1,
		RequestedTier:     requestedTier,
		ResolvedTier:      normalized,
	}

	switch normalized {
	case "T1":
		constraints.ExecutorLoopLimit = 2
		constraints.MaxPlanCandidates = 1
	case "T2":
		constraints.ExecutorLoopLimit = 5
		constraints.MaxPlanCandidates = 2
	case "T3":
		constraints.ExecutorLoopLimit = 10
		constraints.MaxPlanCandidates = 3
	default:
		resolvedTier = "T1"
		constraints.ExecutorLoopLimit = 2
		constraints.MaxPlanCandidates = 1
		constraints.ResolvedTier = resolvedTier
	}
	return constraints
}

func ExecuteReActEarlyExit(tier string, stepsExecuted int, signals, partialResults []string) ReActExecutionResult {
	constraints := ReasoningConstraintsForTier(tier)
	result := ReActExecutionResult{
		RequestedTier:  tier,
		ResolvedTier:   constraints.ResolvedTier,
		MaxSteps:       constraints.ExecutorLoopLimit,
		StepsExecuted:  stepsExecuted,
		EarlyExit:      false,
		ExitReason:     "CONTINUE",
		PartialResults: append([]string(nil), partialResults...),
	}
	if result.MaxSteps <= 0 {
		result.MaxSteps = 2
	}

	if stepsExecuted >= result.MaxSteps {
		result.EarlyExit = true
		result.ExitReason = "MAX_STEPS_REACHED"
		return normalizePartialResults(result)
	}

	normalizedSignals := append([]string(nil), signals...)
	sort.Strings(normalizedSignals)
	for _, signal := range normalizedSignals {
		normalizedSignal := strings.ToUpper(strings.TrimSpace(signal))
		switch normalizedSignal {
		case "USER_STOP", "TOOL_RESULT_SUFFICIENT", "BUDGET_LIMIT_REACHED", "POLICY_BLOCKED":
			result.EarlyExit = true
			result.ExitReason = "EARLY_EXIT_SIGNAL_" + normalizedSignal
			return normalizePartialResults(result)
		}
	}

	return normalizePartialResults(result)
}

func normalizePartialResults(result ReActExecutionResult) ReActExecutionResult {
	if len(result.PartialResults) == 0 {
		if result.EarlyExit {
			result.PartialResults = []string{"partial_result_available"}
		}
		return result
	}
	if len(result.PartialResults) > 3 {
		result.PartialResults = result.PartialResults[:3]
	}
	return result
}

func DeterministicToolSchemaOrder(toolKeys []string) []string {
	out := append([]string(nil), toolKeys...)
	sort.Strings(out)
	return out
}

func DeterministicContextItemOrder(items []string) []string {
	out := append([]string(nil), items...)
	sort.Strings(out)
	return out
}

// ScorePlan implements addendum utility U(plan).
func ScorePlan(plan PlanCandidate, weights PlanScoreWeights) float64 {
	maxTools := maxInt(plan.MaxToolsForTier, 1)
	budgetRemaining := plan.BudgetRemaining
	if budgetRemaining <= 0 {
		budgetRemaining = 1
	}
	tierSLO := maxInt(plan.TierSLOMS, 1)

	numTools := plan.NumTools
	if numTools <= 0 {
		numTools = len(plan.ToolKeys)
	}

	toolCountScore := 1.0 - (float64(numTools) / float64(maxTools))
	riskScore := 1.0 - (float64(plan.MaxToolRisk) / 4.0)
	costScore := 1.0 - (plan.EstimatedCost / budgetRemaining)
	latencyScore := 1.0 - (float64(plan.EstimatedLatencyMS) / float64(tierSLO))

	capabilityConfidence := 0.0
	if len(plan.CapabilityStrengths) > 0 {
		minStrength := plan.CapabilityStrengths[0]
		for _, strength := range plan.CapabilityStrengths[1:] {
			if strength < minStrength {
				minStrength = strength
			}
		}
		capabilityConfidence = float64(minStrength) / 100.0
	}

	score := 0.0
	score += weights.ToolCountScoreWeight * clamp01(toolCountScore)
	score += weights.RiskScoreWeight * clamp01(riskScore)
	score += weights.CostScoreWeight * clamp01(costScore)
	score += weights.LatencyScoreWeight * clamp01(latencyScore)
	score += weights.CapabilityConfidenceScoreWeight * clamp01(capabilityConfidence)
	return score
}

func SelectBestPlan(candidates []PlanCandidate, weights PlanScoreWeights) (PlanCandidate, float64, bool) {
	if len(candidates) == 0 {
		return PlanCandidate{}, 0, false
	}
	type scored struct {
		candidate PlanCandidate
		score     float64
	}
	scoredPlans := make([]scored, 0, len(candidates))
	for _, candidate := range candidates {
		scoredPlans = append(scoredPlans, scored{
			candidate: candidate,
			score:     ScorePlan(candidate, weights),
		})
	}
	sort.Slice(scoredPlans, func(i, j int) bool {
		if scoredPlans[i].score == scoredPlans[j].score {
			leftTools := scoredPlans[i].candidate.NumTools
			if leftTools <= 0 {
				leftTools = len(scoredPlans[i].candidate.ToolKeys)
			}
			rightTools := scoredPlans[j].candidate.NumTools
			if rightTools <= 0 {
				rightTools = len(scoredPlans[j].candidate.ToolKeys)
			}
			if leftTools == rightTools {
				leftSeq := strings.Join(scoredPlans[i].candidate.ToolKeys, ",")
				rightSeq := strings.Join(scoredPlans[j].candidate.ToolKeys, ",")
				return leftSeq < rightSeq
			}
			return leftTools < rightTools
		}
		return scoredPlans[i].score > scoredPlans[j].score
	})
	best := scoredPlans[0]
	return best.candidate, best.score, true
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}
