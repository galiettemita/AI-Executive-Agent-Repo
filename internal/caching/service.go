package caching

import (
	"fmt"
	"sort"
	"sync"
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
}

type Service struct {
	mu             sync.RWMutex
	nextPolicyID   int
	policies       map[string]Policy
	workspaceStats map[string]Stats
	entries        map[string]map[string]string
}

func NewService() *Service {
	return &Service{
		nextPolicyID:   1,
		policies:       map[string]Policy{},
		workspaceStats: map[string]Stats{},
		entries:        map[string]map[string]string{},
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
	if policy.TTLSeconds == 0 {
		policy.TTLSeconds = 300
	}
	if policy.MaxBytes == 0 {
		policy.MaxBytes = 1 << 20
	}
	s.policies[policy.ID] = policy
	return policy
}

func (s *Service) ListPolicies(workspaceID string) []Policy {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]Policy, 0, len(s.policies))
	for _, policy := range s.policies {
		if workspaceID != "" && policy.WorkspaceID != workspaceID {
			continue
		}
		out = append(out, policy)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].ID < out[j].ID
	})
	return out
}

func (s *Service) PutEntry(workspaceID, cacheKey, value string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if workspaceID == "" {
		workspaceID = "default"
	}
	if _, ok := s.entries[workspaceID]; !ok {
		s.entries[workspaceID] = map[string]string{}
	}
	s.entries[workspaceID][cacheKey] = value
	stats := s.workspaceStats[workspaceID]
	stats.WorkspaceID = workspaceID
	stats.Entries = len(s.entries[workspaceID])
	stats.HitRate = calculateHitRate(stats.Hits, stats.Misses)
	s.workspaceStats[workspaceID] = stats
}

func (s *Service) RecordHit(workspaceID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	stats := s.workspaceStats[workspaceID]
	stats.WorkspaceID = workspaceID
	stats.Hits++
	stats.HitRate = calculateHitRate(stats.Hits, stats.Misses)
	s.workspaceStats[workspaceID] = stats
}

func (s *Service) RecordMiss(workspaceID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	stats := s.workspaceStats[workspaceID]
	stats.WorkspaceID = workspaceID
	stats.Misses++
	stats.HitRate = calculateHitRate(stats.Hits, stats.Misses)
	s.workspaceStats[workspaceID] = stats
}

func (s *Service) Invalidate(workspaceID, cacheKey string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	entries, ok := s.entries[workspaceID]
	if !ok {
		return false
	}
	if _, ok := entries[cacheKey]; !ok {
		return false
	}
	delete(entries, cacheKey)

	stats := s.workspaceStats[workspaceID]
	stats.WorkspaceID = workspaceID
	stats.Invalidations++
	stats.Entries = len(entries)
	stats.HitRate = calculateHitRate(stats.Hits, stats.Misses)
	s.workspaceStats[workspaceID] = stats
	return true
}

func (s *Service) Stats(workspaceID string) Stats {
	s.mu.RLock()
	defer s.mu.RUnlock()
	stats := s.workspaceStats[workspaceID]
	stats.WorkspaceID = workspaceID
	stats.HitRate = calculateHitRate(stats.Hits, stats.Misses)
	return stats
}

func calculateHitRate(hits, misses int) float64 {
	total := hits + misses
	if total == 0 {
		return 0
	}
	return float64(hits) / float64(total)
}
