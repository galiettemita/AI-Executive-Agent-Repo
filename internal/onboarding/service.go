package onboarding

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

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
			},
			"behavior_policy_calibration_v1": {
				{Key: "tone", Prompt: "Preferred tone?"},
				{Key: "risk", Prompt: "Risk tolerance?"},
			},
			"codebase_map_ingestion_v1": {
				{Key: "repo", Prompt: "Primary repository?"},
				{Key: "stack", Prompt: "Core stack?"},
			},
			"system_map_ingestion_v1": {
				{Key: "integrations", Prompt: "Critical integrations?"},
				{Key: "sla", Prompt: "Critical SLA targets?"},
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
		return StageResult{StageKey: stageKey, Extracted: cached}, nil
	}

	extracted := map[string]string{}
	for _, q := range questions {
		extracted[q.Key] = strings.TrimSpace(answers[q.Key])
	}
	s.replay[key] = extracted
	return StageResult{StageKey: stageKey, Extracted: extracted}, nil
}

func (s *Service) CompleteOnboarding(workspaceID string, stageAnswers map[string]map[string]string) error {
	stages := []string{
		"operator_profile_intake_v1",
		"behavior_policy_calibration_v1",
		"codebase_map_ingestion_v1",
		"system_map_ingestion_v1",
	}
	for _, stage := range stages {
		if _, err := s.RunStage(workspaceID, stage, stageAnswers[stage]); err != nil {
			return err
		}
	}

	now := time.Now().UTC()
	_ = now
	s.profiles[workspaceID] = WorkspaceProfile{
		WorkspaceID: workspaceID,
		VersionInt:  1,
		Dimensions: map[string]string{
			"focus":      stageAnswers["operator_profile_intake_v1"]["goals"],
			"role":       stageAnswers["operator_profile_intake_v1"]["role"],
			"risk":       stageAnswers["behavior_policy_calibration_v1"]["risk"],
			"tech_stack": stageAnswers["codebase_map_ingestion_v1"]["stack"],
		},
	}
	s.personas[workspaceID] = WorkspacePersona{
		WorkspaceID: workspaceID,
		VersionInt:  1,
		Persona: map[string]string{
			"tone": stageAnswers["behavior_policy_calibration_v1"]["tone"],
		},
	}
	s.behaviorPolicies[workspaceID] = WorkspaceBehaviorPolicy{
		WorkspaceID: workspaceID,
		VersionInt:  1,
		Policy: map[string]string{
			"integrations": stageAnswers["system_map_ingestion_v1"]["integrations"],
			"sla":          stageAnswers["system_map_ingestion_v1"]["sla"],
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
