package workflows

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math"
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

type Service struct {
	mu               sync.Mutex
	idempotencyStore map[string]ToolExecutionResult
}

// Supported workflow IDs for closure mapping:
// interactive_turn_v1, provisioning_v9, onboarding_v1, drift_watchdog_v1,
// outbox_hold_worker, memory_consolidation, daily_capture_v1,
// daily_log_capture_v1, goal_review_v1, trust_eval_v1,
// learning_consolidation_v1, capability_exploration_v1,
// cross_repo_analysis_v1, mission_control_refresh_v1, rag_ingest_v1,
// rag_eval_v1, tool_health_eval_v1, memory_conflict_resolve_v1,
// compliance_evidence_v1, admin_kpi_report_v1, admin_alert_eval_v1,
// cache_maintenance_v1.
func NewService() *Service {
	return &Service{idempotencyStore: map[string]ToolExecutionResult{}}
}

func (s *Service) InteractiveTurnV1(_ context.Context, message string) InteractiveResult {
	steps := []WorkflowStep{{StepKey: "planner", Status: "completed"}}
	if message == "" {
		return InteractiveResult{FinalState: "TERMINAL", Steps: append(steps, WorkflowStep{StepKey: "terminal", Status: "completed"})}
	}
	steps = append(steps,
		WorkflowStep{StepKey: "executor", Status: "completed"},
		WorkflowStep{StepKey: "critic", Status: "completed"},
		WorkflowStep{StepKey: "reflector", Status: "completed"},
		WorkflowStep{StepKey: "terminal", Status: "completed"},
	)
	return InteractiveResult{FinalState: "TERMINAL", Steps: steps}
}

func (s *Service) ProvisioningV9(_ context.Context, failAt string) ProvisioningResult {
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
		executed = append(executed, step)
		if failAt != "" && step == failAt {
			for i := len(executed) - 1; i >= 0; i-- {
				compensated = append(compensated, executed[i])
			}
			return ProvisioningResult{Status: "failed", ExecutedSteps: executed, CompensatedSteps: compensated}
		}
	}
	return ProvisioningResult{Status: "active", ExecutedSteps: executed}
}

func (s *Service) OnboardingV1(_ context.Context, answers map[string]string) OnboardingResult {
	stages := []string{
		"operator_profile_intake_v1",
		"behavior_policy_calibration_v1",
		"codebase_map_ingestion_v1",
		"system_map_ingestion_v1",
	}
	for _, stage := range stages {
		if answers[stage] == "" {
			return OnboardingResult{CompletedStages: []string{}, Status: "incomplete"}
		}
	}
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
	return "completed"
}

func (s *Service) DailyLogCaptureV1(_ context.Context, interactiveTurnID string) string {
	if interactiveTurnID == "" {
		return "skipped"
	}
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
	if capabilityGapEventsLast7d >= 3 {
		return "recommendations_created"
	}
	return "no_action"
}

func (s *Service) CrossRepoAnalysisV1(_ context.Context, repositoryCount int) string {
	if repositoryCount <= 1 {
		return "insufficient_repositories"
	}
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

func (s *Service) ExecuteToolWithIdempotency(workspaceID, toolKey, logicalAction string) (ToolExecutionResult, error) {
	if workspaceID == "" || toolKey == "" || logicalAction == "" {
		return ToolExecutionResult{}, fmt.Errorf("workspace_id, tool_key, and logical_action are required")
	}
	payload := payloadHash(workspaceID, toolKey, logicalAction)
	key := workspaceID + "::" + toolKey + "::" + payload

	s.mu.Lock()
	defer s.mu.Unlock()
	if existing, ok := s.idempotencyStore[key]; ok {
		return existing, nil
	}

	result := ToolExecutionResult{
		IdempotencyKey: key,
		PayloadHash:    payload,
		CreatedAt:      time.Now().UTC(),
	}
	s.idempotencyStore[key] = result
	return result, nil
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
