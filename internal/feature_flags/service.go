package feature_flags

import (
	"sort"
	"strings"
	"sync"
	"time"
)

type Flag struct {
	Key      string `json:"key"`
	FlagType string `json:"flag_type"`
	Enabled  bool   `json:"enabled"`
}

type Rule struct {
	MatchType  string `json:"match_type"`
	MatchValue string `json:"match_value"`
	Enabled    bool   `json:"enabled"`
	Variant    string `json:"variant,omitempty"`
}

type Evaluation struct {
	FlagKey     string `json:"flag_key"`
	WorkspaceID string `json:"workspace_id"`
	Enabled     bool   `json:"enabled"`
	Variant     string `json:"variant"`
	Reason      string `json:"reason"`
}

type cachedEvaluation struct {
	result    Evaluation
	expiresAt time.Time
}

type Service struct {
	mu              sync.RWMutex
	flags           map[string]Flag
	rules           map[string][]Rule
	killSwitch      bool
	evaluationCache map[string]cachedEvaluation
	cacheTTL        time.Duration
	now             func() time.Time
}

func NewService() *Service {
	return &Service{
		flags:           map[string]Flag{},
		rules:           map[string][]Rule{},
		evaluationCache: map[string]cachedEvaluation{},
		cacheTTL:        5 * time.Minute,
		now:             func() time.Time { return time.Now().UTC() },
	}
}

func (s *Service) UpsertFlag(flag Flag) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if strings.TrimSpace(flag.Key) == "" {
		return
	}
	if strings.TrimSpace(flag.FlagType) == "" {
		flag.FlagType = "boolean"
	}
	s.flags[flag.Key] = flag
	s.resetCacheLocked()
}

func (s *Service) GetFlag(key string) (Flag, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	flag, ok := s.flags[key]
	return flag, ok
}

func (s *Service) ListFlags() []Flag {
	s.mu.RLock()
	defer s.mu.RUnlock()

	keys := make([]string, 0, len(s.flags))
	for key := range s.flags {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	out := make([]Flag, 0, len(keys))
	for _, key := range keys {
		out = append(out, s.flags[key])
	}
	return out
}

func (s *Service) DeleteFlag(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.flags, key)
	delete(s.rules, key)
	s.resetCacheLocked()
}

func (s *Service) SetRules(key string, rules []Rule) {
	s.mu.Lock()
	defer s.mu.Unlock()
	copied := make([]Rule, len(rules))
	copy(copied, rules)
	sort.Slice(copied, func(i, j int) bool {
		if copied[i].MatchType == copied[j].MatchType {
			return copied[i].MatchValue < copied[j].MatchValue
		}
		return copied[i].MatchType < copied[j].MatchType
	})
	s.rules[key] = copied
	s.resetCacheLocked()
}

func (s *Service) GetRules(key string) []Rule {
	s.mu.RLock()
	defer s.mu.RUnlock()
	copied := make([]Rule, len(s.rules[key]))
	copy(copied, s.rules[key])
	return copied
}

func (s *Service) SetKillSwitch(enabled bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.killSwitch = enabled
	s.resetCacheLocked()
}

func (s *Service) Evaluate(key string, attributes map[string]string) (bool, string) {
	workspaceID := ""
	if attributes != nil {
		workspaceID = attributes["workspace"]
	}
	result := s.EvaluateForWorkspace(key, workspaceID, attributes)
	return result.Enabled, result.Reason
}

func (s *Service) EvaluateForWorkspace(key, workspaceID string, attributes map[string]string) Evaluation {
	s.mu.Lock()
	defer s.mu.Unlock()

	workspaceID = normalizeWorkspaceID(workspaceID, attributes)
	cacheKey := evaluationCacheKey(key, workspaceID, attributes)
	if cached, ok := s.evaluationCache[cacheKey]; ok && cached.expiresAt.After(s.now()) {
		return cached.result
	}

	result := s.evaluateNoCacheLocked(key, workspaceID, attributes)
	s.evaluationCache[cacheKey] = cachedEvaluation{
		result:    result,
		expiresAt: s.now().Add(s.cacheTTL),
	}
	return result
}

func (s *Service) evaluateNoCacheLocked(key, workspaceID string, attributes map[string]string) Evaluation {
	result := Evaluation{
		FlagKey:     key,
		WorkspaceID: workspaceID,
		Enabled:     false,
		Variant:     "off",
		Reason:      "FLAG_NOT_FOUND",
	}

	if s.killSwitch {
		result.Reason = "FEATURE_DISABLED_BY_KILL_SWITCH"
		return result
	}

	flag, ok := s.flags[key]
	if !ok {
		return result
	}
	if !flag.Enabled {
		result.Reason = "FEATURE_DISABLED"
		return result
	}

	for _, rule := range s.rules[key] {
		if attributes[rule.MatchType] != rule.MatchValue {
			continue
		}
		if rule.Enabled {
			result.Enabled = true
			result.Variant = defaultVariant(rule.Variant, "on")
			result.Reason = "FEATURE_RULE_MATCH_ALLOW"
			return result
		}
		result.Reason = "FEATURE_RULE_MATCH_DENY"
		result.Variant = defaultVariant(rule.Variant, "off")
		return result
	}

	result.Enabled = true
	result.Variant = "on"
	result.Reason = "FEATURE_ENABLED_DEFAULT"
	return result
}

func (s *Service) resetCacheLocked() {
	s.evaluationCache = map[string]cachedEvaluation{}
}

func defaultVariant(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func normalizeWorkspaceID(workspaceID string, attributes map[string]string) string {
	if strings.TrimSpace(workspaceID) != "" {
		return workspaceID
	}
	if attributes != nil {
		if fromAttributes := strings.TrimSpace(attributes["workspace"]); fromAttributes != "" {
			return fromAttributes
		}
		if fromAttributes := strings.TrimSpace(attributes["workspace_id"]); fromAttributes != "" {
			return fromAttributes
		}
	}
	return "default"
}

func evaluationCacheKey(key, workspaceID string, attributes map[string]string) string {
	if len(attributes) == 0 {
		return key + "|" + workspaceID
	}
	attributeKeys := make([]string, 0, len(attributes))
	for attributeKey := range attributes {
		attributeKeys = append(attributeKeys, attributeKey)
	}
	sort.Strings(attributeKeys)

	parts := make([]string, 0, len(attributeKeys)+2)
	parts = append(parts, key, workspaceID)
	for _, attributeKey := range attributeKeys {
		parts = append(parts, attributeKey+"="+attributes[attributeKey])
	}
	return strings.Join(parts, "|")
}
