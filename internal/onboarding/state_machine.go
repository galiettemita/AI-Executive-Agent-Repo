package onboarding

import (
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Stage constants for the onboarding state machine.
const (
	StageWelcome      = "welcome"
	StageDiscovery    = "discovery"
	StageOAuthConnect = "oauth_connect"
	StageCalibration  = "calibration"
	StageFirstValue   = "first_value"
	StageWrapUp       = "wrap_up"
)

var stageOrder = []string{
	StageWelcome,
	StageDiscovery,
	StageOAuthConnect,
	StageCalibration,
	StageFirstValue,
	StageWrapUp,
}

// OnboardingSession tracks progress through the onboarding funnel.
type OnboardingSession struct {
	ID              string            `json:"id"`
	WorkspaceID     string            `json:"workspace_id"`
	CurrentStage    string            `json:"current_stage"`
	CompletedStages []string          `json:"completed_stages"`
	SkippedStages   []string          `json:"skipped_stages"`
	StageAnswers    map[string]string `json:"stage_answers"`
	StartedAt       time.Time         `json:"started_at"`
	CompletedAt     time.Time         `json:"completed_at,omitempty"`
}

// OnboardingService manages the user onboarding state machine.
type OnboardingService struct {
	mu       sync.Mutex
	sessions map[string]OnboardingSession // keyed by session ID
	byWS     map[string]string            // workspace ID -> session ID
	now      func() time.Time
}

// NewOnboardingService creates a new OnboardingService.
func NewOnboardingService() *OnboardingService {
	return &OnboardingService{
		sessions: map[string]OnboardingSession{},
		byWS:     map[string]string{},
		now:      func() time.Time { return time.Now().UTC() },
	}
}

// StartSession begins a new onboarding session for a workspace.
func (s *OnboardingService) StartSession(workspaceID string) (*OnboardingSession, error) {
	if workspaceID == "" {
		return nil, fmt.Errorf("workspace ID is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if existingID, ok := s.byWS[workspaceID]; ok {
		existing := s.sessions[existingID]
		if !existing.CompletedAt.IsZero() {
			// Allow re-onboarding after completion.
		} else {
			return nil, fmt.Errorf("workspace %s already has an active onboarding session", workspaceID)
		}
	}

	session := OnboardingSession{
		ID:              uuid.Must(uuid.NewV7()).String(),
		WorkspaceID:     workspaceID,
		CurrentStage:    StageWelcome,
		CompletedStages: []string{},
		SkippedStages:   []string{},
		StageAnswers:    map[string]string{},
		StartedAt:       s.now(),
	}
	s.sessions[session.ID] = session
	s.byWS[workspaceID] = session.ID
	return &session, nil
}

// AdvanceStage validates the current stage, records answers, and moves to the next stage.
func (s *OnboardingService) AdvanceStage(sessionID string, answers map[string]string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	session, ok := s.sessions[sessionID]
	if !ok {
		return fmt.Errorf("session not found: %s", sessionID)
	}
	if !session.CompletedAt.IsZero() {
		return fmt.Errorf("session is already completed")
	}

	// Record answers for the current stage.
	for k, v := range answers {
		session.StageAnswers[session.CurrentStage+"."+k] = v
	}

	session.CompletedStages = append(session.CompletedStages, session.CurrentStage)

	nextStage := nextStageName(session.CurrentStage)
	if nextStage == "" {
		session.CompletedAt = s.now()
		session.CurrentStage = ""
	} else {
		session.CurrentStage = nextStage
	}

	s.sessions[sessionID] = session
	return nil
}

// SkipStage skips the current stage and moves to the next one.
func (s *OnboardingService) SkipStage(sessionID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	session, ok := s.sessions[sessionID]
	if !ok {
		return fmt.Errorf("session not found: %s", sessionID)
	}
	if !session.CompletedAt.IsZero() {
		return fmt.Errorf("session is already completed")
	}

	session.SkippedStages = append(session.SkippedStages, session.CurrentStage)

	nextStage := nextStageName(session.CurrentStage)
	if nextStage == "" {
		session.CompletedAt = s.now()
		session.CurrentStage = ""
	} else {
		session.CurrentStage = nextStage
	}

	s.sessions[sessionID] = session
	return nil
}

// GetStatus returns the onboarding session for a workspace.
func (s *OnboardingService) GetStatus(workspaceID string) (*OnboardingSession, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	sid, ok := s.byWS[workspaceID]
	if !ok {
		return nil, fmt.Errorf("no onboarding session for workspace: %s", workspaceID)
	}
	session := s.sessions[sid]
	return &session, nil
}

// IsComplete returns whether a session has been completed.
func (s *OnboardingService) IsComplete(sessionID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	session, ok := s.sessions[sessionID]
	if !ok {
		return false
	}
	return !session.CompletedAt.IsZero()
}

func nextStageName(current string) string {
	for i, stage := range stageOrder {
		if stage == current && i+1 < len(stageOrder) {
			return stageOrder[i+1]
		}
	}
	return ""
}

func stageIndex(stage string) int {
	for i, s := range stageOrder {
		if s == stage {
			return i
		}
	}
	return -1
}

// AllStages returns the ordered list of onboarding stages.
func AllStages() []string {
	out := make([]string, len(stageOrder))
	copy(out, stageOrder)
	return out
}

// IsValidStage checks whether a stage name is valid.
func IsValidStage(stage string) bool {
	return stageIndex(stage) >= 0
}
