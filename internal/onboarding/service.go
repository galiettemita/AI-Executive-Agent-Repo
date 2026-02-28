package onboarding

import (
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/google/uuid"
)

type Question struct {
	Key    string
	Prompt string
}

type StageResult struct {
	StageKey  string
	Extracted map[string]string
}

type WorkspaceProfile struct {
	WorkspaceID string
	VersionInt  int
	Dimensions  map[string]string
}

type WorkspacePersona struct {
	WorkspaceID string
	VersionInt  int
	Persona     map[string]string
}

type WorkspaceBehaviorPolicy struct {
	WorkspaceID string
	VersionInt  int
	Policy      map[string]string
}

type Service struct {
	mu               sync.Mutex
	questionSets     map[string][]Question
	replay           map[string]map[string]string
	profiles         map[string]WorkspaceProfile
	personas         map[string]WorkspacePersona
	behaviorPolicies map[string]WorkspaceBehaviorPolicy
}

func NewService() *Service {
	return &Service{
		questionSets: map[string][]Question{
			"operator_profile_intake_v1": {
				{Key: "role", Prompt: "What is your role?"},
				{Key: "goals", Prompt: "What are your primary goals?"},
				{Key: "industry", Prompt: "What industry does your workspace serve?"},
				{Key: "team_size", Prompt: "What is your team size?"},
				{Key: "timezone", Prompt: "What is your default timezone?"},
				{Key: "decision_style", Prompt: "How do you make decisions?"},
				{Key: "communication_pref", Prompt: "Preferred communication style?"},
				{Key: "kpi_primary", Prompt: "What is the primary KPI?"},
			},
			"behavior_policy_calibration_v1": {
				{Key: "tone", Prompt: "Preferred assistant tone?"},
				{Key: "risk_tolerance", Prompt: "Risk tolerance?"},
				{Key: "autonomy_preference", Prompt: "Autonomy preference?"},
				{Key: "approval_threshold", Prompt: "When should approvals be required?"},
				{Key: "proactive_mode", Prompt: "Should proactive actions be enabled?"},
				{Key: "notification_window", Prompt: "Preferred notification window?"},
				{Key: "initiative_level", Prompt: "How proactive should assistant initiative be?"},
			},
			"codebase_map_ingestion_v1": {
				{Key: "repo", Prompt: "Primary repository?"},
				{Key: "stack", Prompt: "Core stack?"},
				{Key: "planning_horizon", Prompt: "Planning horizon?"},
				{Key: "meeting_load", Prompt: "Weekly meeting load?"},
				{Key: "focus_mode", Prompt: "Preferred focus mode?"},
			},
			"system_map_ingestion_v1": {
				{Key: "integrations", Prompt: "Critical integrations?"},
				{Key: "sla", Prompt: "Critical SLA targets?"},
				{Key: "escalation_path", Prompt: "Escalation path?"},
				{Key: "privacy_mode", Prompt: "Privacy mode?"},
				{Key: "audit_strictness", Prompt: "Audit strictness?"},
				{Key: "delivery_cadence", Prompt: "Delivery cadence?"},
				{Key: "context_budget", Prompt: "Context budget preference?"},
				{Key: "write_actions", Prompt: "Write action policy?"},
				{Key: "language", Prompt: "Preferred language?"},
			},
		},
		replay:           map[string]map[string]string{},
		profiles:         map[string]WorkspaceProfile{},
		personas:         map[string]WorkspacePersona{},
		behaviorPolicies: map[string]WorkspaceBehaviorPolicy{},
	}
}

func replayKey(workspaceID, stageKey string, answers map[string]string) string {
	keys := make([]string, 0, len(answers))
	for key := range answers {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := []string{workspaceID, stageKey}
	for _, key := range keys {
		parts = append(parts, key+"="+answers[key])
	}
	return strings.Join(parts, "::")
}

func (s *Service) QuestionSet(stageKey string) ([]Question, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	questions, ok := s.questionSets[stageKey]
	if !ok {
		return nil, fmt.Errorf("unknown stage: %s", stageKey)
	}
	out := make([]Question, len(questions))
	copy(out, questions)
	return out, nil
}

func (s *Service) RunStage(workspaceID, stageKey string, answers map[string]string) (StageResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	questions, ok := s.questionSets[stageKey]
	if !ok {
		return StageResult{}, fmt.Errorf("unknown stage: %s", stageKey)
	}
	for _, q := range questions {
		if strings.TrimSpace(answers[q.Key]) == "" {
			return StageResult{}, fmt.Errorf("missing answer for %s", q.Key)
		}
	}

	key := replayKey(workspaceID, stageKey, answers)
	if cached, ok := s.replay[key]; ok {
		return StageResult{StageKey: stageKey, Extracted: copyStringMap(cached)}, nil
	}

	extracted := map[string]string{}
	for _, q := range questions {
		extracted[q.Key] = strings.TrimSpace(answers[q.Key])
	}
	s.replay[key] = copyStringMap(extracted)
	return StageResult{StageKey: stageKey, Extracted: extracted}, nil
}

func copyStringMap(input map[string]string) map[string]string {
	out := make(map[string]string, len(input))
	for k, v := range input {
		out[k] = v
	}
	return out
}

func nextVersion(current int) int {
	if current < 1 {
		return 1
	}
	return current + 1
}

func (s *Service) CompleteOnboarding(workspaceID string, stageAnswers map[string]map[string]string) error {
	stages := []string{
		"operator_profile_intake_v1",
		"behavior_policy_calibration_v1",
		"codebase_map_ingestion_v1",
		"system_map_ingestion_v1",
	}
	results := map[string]StageResult{}
	for _, stage := range stages {
		result, err := s.RunStage(workspaceID, stage, stageAnswers[stage])
		if err != nil {
			return err
		}
		results[stage] = result
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	profileVersion := nextVersion(s.profiles[workspaceID].VersionInt)
	personaVersion := nextVersion(s.personas[workspaceID].VersionInt)
	policyVersion := nextVersion(s.behaviorPolicies[workspaceID].VersionInt)

	op := results["operator_profile_intake_v1"].Extracted
	bp := results["behavior_policy_calibration_v1"].Extracted
	cb := results["codebase_map_ingestion_v1"].Extracted
	sy := results["system_map_ingestion_v1"].Extracted

	s.profiles[workspaceID] = WorkspaceProfile{
		WorkspaceID: workspaceID,
		VersionInt:  profileVersion,
		Dimensions: map[string]string{
			"role":                op["role"],
			"goals":               op["goals"],
			"industry":            op["industry"],
			"team_size":           op["team_size"],
			"timezone":            op["timezone"],
			"decision_style":      op["decision_style"],
			"communication_pref":  op["communication_pref"],
			"kpi_primary":         op["kpi_primary"],
			"risk_tolerance":      bp["risk_tolerance"],
			"autonomy_preference": bp["autonomy_preference"],
			"planning_horizon":    cb["planning_horizon"],
			"meeting_load":        cb["meeting_load"],
			"focus_mode":          cb["focus_mode"],
		},
	}
	s.personas[workspaceID] = WorkspacePersona{
		WorkspaceID: workspaceID,
		VersionInt:  personaVersion,
		Persona: map[string]string{
			"tone":               bp["tone"],
			"initiative_level":   bp["initiative_level"],
			"language":           sy["language"],
			"communication_pref": op["communication_pref"],
			"decision_style":     op["decision_style"],
		},
	}
	s.behaviorPolicies[workspaceID] = WorkspaceBehaviorPolicy{
		WorkspaceID: workspaceID,
		VersionInt:  policyVersion,
		Policy: map[string]string{
			"approval_threshold":  bp["approval_threshold"],
			"proactive_mode":      bp["proactive_mode"],
			"notification_window": bp["notification_window"],
			"write_actions":       sy["write_actions"],
			"escalation_path":     sy["escalation_path"],
			"privacy_mode":        sy["privacy_mode"],
			"audit_strictness":    sy["audit_strictness"],
			"delivery_cadence":    sy["delivery_cadence"],
			"context_budget":      sy["context_budget"],
			"sla":                 sy["sla"],
		},
	}
	return nil
}

func (s *Service) WorkspaceState(workspaceID string) (WorkspaceProfile, WorkspacePersona, WorkspaceBehaviorPolicy, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	profile, okProfile := s.profiles[workspaceID]
	persona, okPersona := s.personas[workspaceID]
	policy, okPolicy := s.behaviorPolicies[workspaceID]
	if !okProfile || !okPersona || !okPolicy {
		return WorkspaceProfile{}, WorkspacePersona{}, WorkspaceBehaviorPolicy{}, fmt.Errorf("workspace onboarding state incomplete")
	}
	return profile, persona, policy, nil
}

func NewWorkspaceID() string {
	return uuid.Must(uuid.NewV7()).String()
}
