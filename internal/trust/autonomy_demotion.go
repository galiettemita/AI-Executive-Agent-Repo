package trust

import (
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// DemotionConfig defines thresholds that trigger autonomy demotion.
type DemotionConfig struct {
	IncidentThreshold float64 `json:"incident_threshold"` // trust score below which demotion triggers
	DriftDays         int     `json:"drift_days"`         // number of days of inactivity before drift demotion
}

// DemotionEvent records a single autonomy demotion event.
type DemotionEvent struct {
	ID            string    `json:"id"`
	WorkspaceID   string    `json:"workspace_id"`
	Domain        string    `json:"domain"`
	PreviousLevel int       `json:"previous_level"`
	NewLevel      int       `json:"new_level"`
	Reason        string    `json:"reason"`
	DemotedAt     time.Time `json:"demoted_at"`
}

// AutonomyDemotionService manages autonomy demotion checks and history.
type AutonomyDemotionService struct {
	mu     sync.Mutex
	config DemotionConfig
	events []DemotionEvent
	levels map[string]int // key: workspaceID::domain -> current level
	now    func() time.Time
}

// NewAutonomyDemotionService creates a new demotion service with defaults.
func NewAutonomyDemotionService(config DemotionConfig) *AutonomyDemotionService {
	if config.IncidentThreshold <= 0 {
		config.IncidentThreshold = 0.4
	}
	if config.DriftDays <= 0 {
		config.DriftDays = 30
	}
	return &AutonomyDemotionService{
		config: config,
		events: []DemotionEvent{},
		levels: map[string]int{},
		now:    func() time.Time { return time.Now().UTC() },
	}
}

func domainKey(workspaceID, domain string) string {
	return workspaceID + "::" + domain
}

// SetLevel sets the current autonomy level for a workspace/domain.
func (s *AutonomyDemotionService) SetLevel(workspaceID, domain string, level int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.levels[domainKey(workspaceID, domain)] = level
}

// GetLevel returns the current autonomy level for a workspace/domain.
func (s *AutonomyDemotionService) GetLevel(workspaceID, domain string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.levels[domainKey(workspaceID, domain)]
}

// CheckForDemotion evaluates whether a demotion should occur based on trust score
// and failure count. Returns the demotion event if demotion happened, or nil.
func (s *AutonomyDemotionService) CheckForDemotion(workspaceID, domain string, trustScore float64, failureCount int) (*DemotionEvent, error) {
	if workspaceID == "" {
		return nil, fmt.Errorf("workspace_id is required")
	}
	if domain == "" {
		return nil, fmt.Errorf("domain is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	key := domainKey(workspaceID, domain)
	currentLevel := s.levels[key]
	if currentLevel <= 0 {
		return nil, nil // already at minimum
	}

	shouldDemote := false
	reason := ""

	if trustScore < s.config.IncidentThreshold {
		shouldDemote = true
		reason = fmt.Sprintf("trust score %.2f below threshold %.2f", trustScore, s.config.IncidentThreshold)
	}

	if failureCount >= 3 {
		shouldDemote = true
		if reason != "" {
			reason += "; "
		}
		reason += fmt.Sprintf("failure count %d >= 3", failureCount)
	}

	if !shouldDemote {
		return nil, nil
	}

	newLevel := currentLevel - 1
	if newLevel < 0 {
		newLevel = 0
	}

	event := DemotionEvent{
		ID:            uuid.Must(uuid.NewV7()).String(),
		WorkspaceID:   workspaceID,
		Domain:        domain,
		PreviousLevel: currentLevel,
		NewLevel:      newLevel,
		Reason:        reason,
		DemotedAt:     s.now(),
	}
	s.levels[key] = newLevel
	s.events = append(s.events, event)
	return &event, nil
}

// Demote forcefully demotes a workspace/domain by one level.
func (s *AutonomyDemotionService) Demote(workspaceID, domain, reason string) (*DemotionEvent, error) {
	if workspaceID == "" || domain == "" {
		return nil, fmt.Errorf("workspace_id and domain are required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	key := domainKey(workspaceID, domain)
	currentLevel := s.levels[key]
	if currentLevel <= 0 {
		return nil, fmt.Errorf("already at minimum autonomy level")
	}

	newLevel := currentLevel - 1
	event := DemotionEvent{
		ID:            uuid.Must(uuid.NewV7()).String(),
		WorkspaceID:   workspaceID,
		Domain:        domain,
		PreviousLevel: currentLevel,
		NewLevel:      newLevel,
		Reason:        reason,
		DemotedAt:     s.now(),
	}
	s.levels[key] = newLevel
	s.events = append(s.events, event)
	return &event, nil
}

// GetDemotionHistory returns all demotion events for a workspace.
func (s *AutonomyDemotionService) GetDemotionHistory(workspaceID string) []DemotionEvent {
	s.mu.Lock()
	defer s.mu.Unlock()

	var out []DemotionEvent
	for _, e := range s.events {
		if e.WorkspaceID == workspaceID {
			out = append(out, e)
		}
	}
	return out
}

// GetDemotionCount90d returns the number of demotions for a workspace in the last 90 days.
func (s *AutonomyDemotionService) GetDemotionCount90d(workspaceID string) int {
	s.mu.Lock()
	defer s.mu.Unlock()

	cutoff := s.now().AddDate(0, 0, -90)
	count := 0
	for _, e := range s.events {
		if e.WorkspaceID == workspaceID && e.DemotedAt.After(cutoff) {
			count++
		}
	}
	return count
}
