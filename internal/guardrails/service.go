package guardrails

import (
	"fmt"
	"sort"
	"strings"
	"sync"
)

type Config struct {
	WorkspaceID               string `json:"workspace_id"`
	EnablePIIRedaction        bool   `json:"enable_pii_redaction"`
	EnableJailbreakDetection  bool   `json:"enable_jailbreak_detection"`
	BlockThreshold            int    `json:"block_threshold"`
	RequireAuditOnAllow       bool   `json:"require_audit_on_allow"`
	IncludePromptFingerprint  bool   `json:"include_prompt_fingerprint"`
	IncludeResponseValidation bool   `json:"include_response_validation"`
}

type RuleSet struct {
	ID          string   `json:"id"`
	WorkspaceID string   `json:"workspace_id"`
	Name        string   `json:"name"`
	Mode        string   `json:"mode"`
	Patterns    []string `json:"patterns"`
	Enabled     bool     `json:"enabled"`
}

type Event struct {
	ID          string `json:"id"`
	WorkspaceID string `json:"workspace_id"`
	RuleSetID   string `json:"rule_set_id"`
	EventType   string `json:"event_type"`
	Action      string `json:"action"`
	InputHash   string `json:"input_hash"`
}

type Service struct {
	mu       sync.RWMutex
	nextID   int
	configs  map[string]Config
	ruleSets map[string]RuleSet
	events   []Event
}

func NewService() *Service {
	return &Service{
		nextID:   1,
		configs:  map[string]Config{},
		ruleSets: map[string]RuleSet{},
		events:   []Event{},
	}
}

func (s *Service) DefaultConfig(workspaceID string) Config {
	return Config{
		WorkspaceID:               workspaceID,
		EnablePIIRedaction:        true,
		EnableJailbreakDetection:  true,
		BlockThreshold:            80,
		RequireAuditOnAllow:       true,
		IncludePromptFingerprint:  true,
		IncludeResponseValidation: true,
	}
}

func (s *Service) UpsertConfig(workspaceID string, cfg Config) Config {
	s.mu.Lock()
	defer s.mu.Unlock()

	if workspaceID == "" {
		workspaceID = "default"
	}
	defaults := s.DefaultConfig(workspaceID)
	cfg.WorkspaceID = workspaceID
	if cfg.BlockThreshold == 0 {
		cfg.BlockThreshold = defaults.BlockThreshold
	}
	s.configs[workspaceID] = cfg
	return cfg
}

func (s *Service) GetConfig(workspaceID string) (Config, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	cfg, ok := s.configs[workspaceID]
	return cfg, ok
}

func (s *Service) UpsertRuleSet(ruleSet RuleSet) RuleSet {
	s.mu.Lock()
	defer s.mu.Unlock()

	if ruleSet.ID == "" {
		ruleSet.ID = s.nextRuleSetID()
	}
	if ruleSet.WorkspaceID == "" {
		ruleSet.WorkspaceID = "default"
	}
	if ruleSet.Mode == "" {
		ruleSet.Mode = "block"
	}
	ruleSet.Patterns = dedupeAndSort(ruleSet.Patterns)
	s.ruleSets[ruleSet.ID] = ruleSet
	return ruleSet
}

func (s *Service) DeleteRuleSet(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.ruleSets[id]; !ok {
		return false
	}
	delete(s.ruleSets, id)
	return true
}

func (s *Service) ListRuleSets(workspaceID string) []RuleSet {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]RuleSet, 0, len(s.ruleSets))
	for _, ruleSet := range s.ruleSets {
		if workspaceID != "" && ruleSet.WorkspaceID != workspaceID {
			continue
		}
		out = append(out, ruleSet)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].ID < out[j].ID
	})
	return out
}

func (s *Service) RecordEvent(workspaceID, ruleSetID, eventType, action, input string) Event {
	s.mu.Lock()
	defer s.mu.Unlock()

	if workspaceID == "" {
		workspaceID = "default"
	}
	event := Event{
		ID:          fmt.Sprintf("guardrail_event_%06d", len(s.events)+1),
		WorkspaceID: workspaceID,
		RuleSetID:   ruleSetID,
		EventType:   eventType,
		Action:      action,
		InputHash:   simpleHash(input),
	}
	s.events = append(s.events, event)
	return event
}

func (s *Service) ListEvents(workspaceID string) []Event {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]Event, 0, len(s.events))
	for _, event := range s.events {
		if workspaceID != "" && event.WorkspaceID != workspaceID {
			continue
		}
		out = append(out, event)
	}
	return out
}

func (s *Service) nextRuleSetID() string {
	id := fmt.Sprintf("rule_set_%06d", s.nextID)
	s.nextID++
	return id
}

func dedupeAndSort(values []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		clean := strings.TrimSpace(value)
		if clean == "" {
			continue
		}
		if _, ok := seen[clean]; ok {
			continue
		}
		seen[clean] = struct{}{}
		out = append(out, clean)
	}
	sort.Strings(out)
	return out
}

func simpleHash(input string) string {
	total := 0
	for _, ch := range []byte(input) {
		total = (total * 31) + int(ch)
	}
	return fmt.Sprintf("h%08x", total&0xffffffff)
}
