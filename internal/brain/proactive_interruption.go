package brain

import (
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// InterruptionRule defines a trigger condition for proactive interruptions.
type InterruptionRule struct {
	ID              string `json:"id"`
	WorkspaceID     string `json:"workspace_id"`
	TriggerType     string `json:"trigger_type"` // "deadline", "anomaly", "reminder", "insight"
	Priority        int    `json:"priority"`      // 1-10, higher = more important
	Condition       string `json:"condition"`      // human-readable condition description
	CooldownMinutes int    `json:"cooldown_minutes"`
}

// InterruptionCandidate represents a potential interruption that has been evaluated.
type InterruptionCandidate struct {
	RuleID  string  `json:"rule_id"`
	Urgency float64 `json:"urgency"` // 0.0 to 1.0
	Message string  `json:"message"`
}

// ProactiveInterruptionService manages interruption rules and evaluation.
type ProactiveInterruptionService struct {
	mu          sync.Mutex
	rules       []InterruptionRule
	lastFired   map[string]time.Time // ruleID -> last fired time
	now         func() time.Time
}

// NewProactiveInterruptionService creates a new interruption service.
func NewProactiveInterruptionService() *ProactiveInterruptionService {
	return &ProactiveInterruptionService{
		rules:     []InterruptionRule{},
		lastFired: map[string]time.Time{},
		now:       func() time.Time { return time.Now().UTC() },
	}
}

// AddRule adds a new interruption rule.
func (s *ProactiveInterruptionService) AddRule(rule InterruptionRule) (InterruptionRule, error) {
	if rule.WorkspaceID == "" {
		return InterruptionRule{}, fmt.Errorf("workspace_id is required")
	}
	if rule.TriggerType == "" {
		return InterruptionRule{}, fmt.Errorf("trigger_type is required")
	}
	validTypes := map[string]bool{"deadline": true, "anomaly": true, "reminder": true, "insight": true}
	if !validTypes[rule.TriggerType] {
		return InterruptionRule{}, fmt.Errorf("invalid trigger_type: %s", rule.TriggerType)
	}
	if rule.Priority < 1 || rule.Priority > 10 {
		return InterruptionRule{}, fmt.Errorf("priority must be between 1 and 10")
	}
	if rule.CooldownMinutes < 0 {
		rule.CooldownMinutes = 0
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	rule.ID = uuid.Must(uuid.NewV7()).String()
	s.rules = append(s.rules, rule)
	return rule, nil
}

// GetRules returns all rules for a workspace.
func (s *ProactiveInterruptionService) GetRules(workspaceID string) []InterruptionRule {
	s.mu.Lock()
	defer s.mu.Unlock()

	var out []InterruptionRule
	for _, r := range s.rules {
		if r.WorkspaceID == workspaceID {
			out = append(out, r)
		}
	}
	return out
}

// EvaluateInterruptions evaluates all rules for a workspace against a context string
// and returns candidates that should be surfaced.
func (s *ProactiveInterruptionService) EvaluateInterruptions(workspaceID, contextStr string) []InterruptionCandidate {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := s.now()
	var candidates []InterruptionCandidate

	for _, rule := range s.rules {
		if rule.WorkspaceID != workspaceID {
			continue
		}

		// Check cooldown.
		if lastFired, ok := s.lastFired[rule.ID]; ok {
			cooldown := time.Duration(rule.CooldownMinutes) * time.Minute
			if now.Sub(lastFired) < cooldown {
				continue
			}
		}

		// Simple condition matching: check if context contains the condition keyword.
		urgency := float64(rule.Priority) / 10.0
		if containsAny(contextStr, rule.Condition) {
			urgency = float64(rule.Priority) / 10.0 * 1.2
			if urgency > 1.0 {
				urgency = 1.0
			}
		}

		// Only include candidates above a minimum urgency threshold.
		if urgency >= 0.3 {
			candidates = append(candidates, InterruptionCandidate{
				RuleID:  rule.ID,
				Urgency: urgency,
				Message: fmt.Sprintf("[%s] %s (priority %d)", rule.TriggerType, rule.Condition, rule.Priority),
			})
			s.lastFired[rule.ID] = now
		}
	}

	return candidates
}

// ShouldInterrupt determines whether a candidate warrants an actual interruption.
// Returns true if urgency is above the interruption threshold (0.5).
func (s *ProactiveInterruptionService) ShouldInterrupt(candidate InterruptionCandidate) bool {
	return candidate.Urgency >= 0.5
}
