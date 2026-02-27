package workflows

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
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

type Service struct {
	mu               sync.Mutex
	idempotencyStore map[string]ToolExecutionResult
}

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
