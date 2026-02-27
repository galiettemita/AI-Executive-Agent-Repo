package feature_flags

import (
	"sort"
	"sync"
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
}

type Service struct {
	mu         sync.RWMutex
	flags      map[string]Flag
	rules      map[string][]Rule
	killSwitch bool
}

func NewService() *Service {
	return &Service{
		flags: map[string]Flag{},
		rules: map[string][]Rule{},
	}
}

func (s *Service) UpsertFlag(flag Flag) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.flags[flag.Key] = flag
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
}

func (s *Service) Evaluate(key string, attributes map[string]string) (bool, string) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.killSwitch {
		return false, "FEATURE_DISABLED_BY_KILL_SWITCH"
	}

	flag, ok := s.flags[key]
	if !ok {
		return false, "FLAG_NOT_FOUND"
	}
	if !flag.Enabled {
		return false, "FEATURE_DISABLED"
	}

	for _, rule := range s.rules[key] {
		if attributes[rule.MatchType] == rule.MatchValue {
			if rule.Enabled {
				return true, "FEATURE_RULE_MATCH_ALLOW"
			}
			return false, "FEATURE_RULE_MATCH_DENY"
		}
	}

	return true, "FEATURE_ENABLED_DEFAULT"
}
