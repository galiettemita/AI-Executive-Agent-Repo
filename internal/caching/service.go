package caching

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	layerL1 = "l1"
	layerL2 = "l2"
	layerL3 = "l3"
)

type Policy struct {
	ID          string `json:"id"`
	WorkspaceID string `json:"workspace_id"`
	CacheKey    string `json:"cache_key"`
	TTLSeconds  int    `json:"ttl_seconds"`
	MaxBytes    int    `json:"max_bytes"`
	Enabled     bool   `json:"enabled"`
}

type Stats struct {
	WorkspaceID   string  `json:"workspace_id"`
	Entries       int     `json:"entries"`
	Hits          int     `json:"hits"`
	Misses        int     `json:"misses"`
	Invalidations int     `json:"invalidations"`
	HitRate       float64 `json:"hit_rate"`
	L1Entries     int     `json:"l1_entries"`
	L2Entries     int     `json:"l2_entries"`
	L3Entries     int     `json:"l3_entries"`
}

type cacheEntry struct {
	Value     string
	ExpiresAt time.Time
	SizeBytes int
}

type Service struct {
	mu             sync.RWMutex
	nextPolicyID   int
	policies       map[string]Policy
	workspaceStats map[string]Stats
	layers         map[string]map[string]map[string]cacheEntry
	now            func() time.Time
}

func NewService() *Service {
	return &Service{
		nextPolicyID:   1,
		policies:       map[string]Policy{},
		workspaceStats: map[string]Stats{},
		layers: map[string]map[string]map[string]cacheEntry{
			layerL1: {},
			layerL2: {},
			layerL3: {},
		},
		now: func() time.Time { return time.Now().UTC() },
	}
}

func (s *Service) UpsertPolicy(policy Policy) Policy {
	s.mu.Lock()
	defer s.mu.Unlock()

	if policy.ID == "" {
		policy.ID = fmt.Sprintf("cache_policy_%06d", s.nextPolicyID)
		s.nextPolicyID++
	}
	if policy.WorkspaceID == "" {
		policy.WorkspaceID = "default"
	}
	if policy.CacheKey == "" {
		policy.CacheKey = "compiled_context"
	}
	if policy.TTLSeconds <= 0 {
		policy.TTLSeconds = 300
	}
	if policy.MaxBytes <= 0 {
		policy.MaxBytes = 1 << 20
	}
	if !policy.Enabled {
		policy.Enabled = true
	}
	s.policies[policy.ID] = policy
	return policy
}

func (s *Service) ListPolicies(workspaceID string) []Policy {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if workspaceID == "" {
		workspaceID = "default"
	}
	out := make([]Policy, 0, len(s.policies))
	for _, policy := range s.policies {
		if policy.WorkspaceID != workspaceID {
			continue
		}
		out = append(out, policy)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].CacheKey == out[j].CacheKey {
			return out[i].ID < out[j].ID
		}
		return out[i].CacheKey < out[j].CacheKey
	})
	return out
}

func (s *Service) PutEntry(workspaceID, cacheKey, value string) {
	_ = s.PutEntryAt(workspaceID, cacheKey, value, s.now())
}

func (s *Service) PutEntryAt(workspaceID, cacheKey, value string, now time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	workspaceID = normalizeWorkspaceID(workspaceID)
	cacheKey = strings.TrimSpace(cacheKey)
	if cacheKey == "" {
		return fmt.Errorf("cache_key is required")
	}
	if now.IsZero() {
		now = s.now()
	}

	policy := s.resolvePolicyLocked(workspaceID, cacheKey)
	if !policy.Enabled {
		return nil
	}
	if len(value) > policy.MaxBytes {
		return fmt.Errorf("CACHE_ENTRY_TOO_LARGE")
	}

	entry := cacheEntry{
		Value:     value,
		ExpiresAt: now.Add(time.Duration(policy.TTLSeconds) * time.Second),
		SizeBytes: len(value),
	}
	s.putLayerEntryLocked(layerL1, workspaceID, cacheKey, entry)
	s.putLayerEntryLocked(layerL2, workspaceID, cacheKey, entry)
	s.putLayerEntryLocked(layerL3, workspaceID, cacheKey, entry)
	s.refreshStatsEntriesLocked(workspaceID)
	return nil
}

func (s *Service) GetEntry(workspaceID, cacheKey string) (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.getEntryLocked(workspaceID, cacheKey, s.now())
}

func (s *Service) GetEntryAt(workspaceID, cacheKey string, now time.Time) (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if now.IsZero() {
		now = s.now()
	}
	return s.getEntryLocked(workspaceID, cacheKey, now)
}

func (s *Service) getEntryLocked(workspaceID, cacheKey string, now time.Time) (string, bool) {
	workspaceID = normalizeWorkspaceID(workspaceID)
	for _, layer := range []string{layerL1, layerL2, layerL3} {
		workspaceLayer := s.layers[layer][workspaceID]
		entry, ok := workspaceLayer[cacheKey]
		if !ok {
			continue
		}
		if now.After(entry.ExpiresAt) {
			delete(workspaceLayer, cacheKey)
			s.refreshStatsEntriesLocked(workspaceID)
			continue
		}
		s.recordHitLocked(workspaceID)
		if layer == layerL2 {
			s.putLayerEntryLocked(layerL1, workspaceID, cacheKey, entry)
		}
		if layer == layerL3 {
			s.putLayerEntryLocked(layerL2, workspaceID, cacheKey, entry)
			s.putLayerEntryLocked(layerL1, workspaceID, cacheKey, entry)
		}
		s.refreshStatsEntriesLocked(workspaceID)
		return entry.Value, true
	}
	s.recordMissLocked(workspaceID)
	s.refreshStatsEntriesLocked(workspaceID)
	return "", false
}

func (s *Service) RecordHit(workspaceID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.recordHitLocked(normalizeWorkspaceID(workspaceID))
	s.refreshStatsEntriesLocked(normalizeWorkspaceID(workspaceID))
}

func (s *Service) RecordMiss(workspaceID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.recordMissLocked(normalizeWorkspaceID(workspaceID))
	s.refreshStatsEntriesLocked(normalizeWorkspaceID(workspaceID))
}

func (s *Service) Invalidate(workspaceID, cacheKey string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	workspaceID = normalizeWorkspaceID(workspaceID)
	invalidated := false
	for _, layer := range []string{layerL1, layerL2, layerL3} {
		entries := s.layers[layer][workspaceID]
		if _, ok := entries[cacheKey]; ok {
			delete(entries, cacheKey)
			invalidated = true
		}
	}
	if !invalidated {
		return false
	}

	stats := s.workspaceStats[workspaceID]
	stats.WorkspaceID = workspaceID
	stats.Invalidations++
	s.workspaceStats[workspaceID] = stats
	s.refreshStatsEntriesLocked(workspaceID)
	return true
}

func (s *Service) Stats(workspaceID string) Stats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	workspaceID = normalizeWorkspaceID(workspaceID)
	stats := s.workspaceStats[workspaceID]
	stats.WorkspaceID = workspaceID
	stats.L1Entries = len(s.layers[layerL1][workspaceID])
	stats.L2Entries = len(s.layers[layerL2][workspaceID])
	stats.L3Entries = len(s.layers[layerL3][workspaceID])
	stats.Entries = stats.L1Entries
	stats.HitRate = calculateHitRate(stats.Hits, stats.Misses)
	return stats
}

func (s *Service) resolvePolicyLocked(workspaceID, cacheKey string) Policy {
	lookupKey := cacheNamespace(cacheKey)
	best := Policy{
		WorkspaceID: workspaceID,
		CacheKey:    lookupKey,
		TTLSeconds:  300,
		MaxBytes:    1 << 20,
		Enabled:     true,
	}
	bestScore := -1
	for _, policy := range s.policies {
		if policy.WorkspaceID != workspaceID {
			continue
		}
		score := policyMatchScore(policy.CacheKey, lookupKey)
		if score < 0 {
			continue
		}
		if score > bestScore {
			bestScore = score
			best = policy
		}
	}
	return best
}

func policyMatchScore(policyKey, lookupKey string) int {
	if policyKey == lookupKey {
		return 10
	}
	if policyKey == "*" {
		return 1
	}
	if strings.HasSuffix(policyKey, "*") {
		prefix := strings.TrimSuffix(policyKey, "*")
		if strings.HasPrefix(lookupKey, prefix) {
			return 5
		}
	}
	return -1
}

func cacheNamespace(cacheKey string) string {
	cacheKey = strings.TrimSpace(cacheKey)
	if cacheKey == "" {
		return "compiled_context"
	}
	if idx := strings.Index(cacheKey, ":"); idx > 0 {
		return cacheKey[:idx]
	}
	return cacheKey
}

func (s *Service) putLayerEntryLocked(layer, workspaceID, cacheKey string, entry cacheEntry) {
	if _, ok := s.layers[layer][workspaceID]; !ok {
		s.layers[layer][workspaceID] = map[string]cacheEntry{}
	}
	s.layers[layer][workspaceID][cacheKey] = entry
}

func (s *Service) refreshStatsEntriesLocked(workspaceID string) {
	stats := s.workspaceStats[workspaceID]
	stats.WorkspaceID = workspaceID
	stats.L1Entries = len(s.layers[layerL1][workspaceID])
	stats.L2Entries = len(s.layers[layerL2][workspaceID])
	stats.L3Entries = len(s.layers[layerL3][workspaceID])
	stats.Entries = stats.L1Entries
	stats.HitRate = calculateHitRate(stats.Hits, stats.Misses)
	s.workspaceStats[workspaceID] = stats
}

func (s *Service) recordHitLocked(workspaceID string) {
	stats := s.workspaceStats[workspaceID]
	stats.WorkspaceID = workspaceID
	stats.Hits++
	s.workspaceStats[workspaceID] = stats
}

func (s *Service) recordMissLocked(workspaceID string) {
	stats := s.workspaceStats[workspaceID]
	stats.WorkspaceID = workspaceID
	stats.Misses++
	s.workspaceStats[workspaceID] = stats
}

func normalizeWorkspaceID(workspaceID string) string {
	if strings.TrimSpace(workspaceID) == "" {
		return "default"
	}
	return workspaceID
}

func calculateHitRate(hits, misses int) float64 {
	total := hits + misses
	if total == 0 {
		return 0
	}
	return float64(hits) / float64(total)
}
