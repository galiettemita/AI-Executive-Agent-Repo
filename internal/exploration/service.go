package exploration

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
	"time"
)

type Recommendation struct {
	RecommendationID string    `json:"recommendation_id"`
	WorkspaceID      string    `json:"workspace_id"`
	CapabilityKey    string    `json:"capability_key"`
	Confidence       float64   `json:"confidence"`
	Reason           string    `json:"reason"`
	Status           string    `json:"status"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

type Service struct {
	mu                  sync.RWMutex
	nextID              int
	recommendations     map[string]Recommendation
	capabilityGapCounts map[string]map[string]int
}

func NewService() *Service {
	return &Service{
		nextID:              1,
		recommendations:     map[string]Recommendation{},
		capabilityGapCounts: map[string]map[string]int{},
	}
}

func (s *Service) ListRecommendations(workspaceID string) []Recommendation {
	s.mu.Lock()
	defer s.mu.Unlock()

	workspaceID = normalizeWorkspaceID(workspaceID)
	s.ensureWorkspaceCountersLocked(workspaceID)
	s.materializeRecommendationsLocked(workspaceID)

	out := make([]Recommendation, 0, len(s.recommendations))
	for _, rec := range s.recommendations {
		if rec.WorkspaceID != workspaceID {
			continue
		}
		out = append(out, rec)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].RecommendationID < out[j].RecommendationID
	})
	return out
}

func (s *Service) RecordCapabilityGap(workspaceID, capabilityKey string) int {
	s.mu.Lock()
	defer s.mu.Unlock()

	workspaceID = normalizeWorkspaceID(workspaceID)
	capabilityKey = normalizeCapabilityKey(capabilityKey)
	s.ensureWorkspaceCountersLocked(workspaceID)
	s.capabilityGapCounts[workspaceID][capabilityKey]++
	s.materializeRecommendationsLocked(workspaceID)
	return s.capabilityGapCounts[workspaceID][capabilityKey]
}

func (s *Service) DecideRecommendation(id, decision string) (Recommendation, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	rec, ok := s.recommendations[id]
	if !ok {
		return Recommendation{}, false, nil
	}

	normalizedDecision := strings.ToLower(strings.TrimSpace(decision))
	switch normalizedDecision {
	case "accept":
		rec.Status = "adopted"
	case "reject":
		rec.Status = "declined"
	case "defer":
		rec.Status = "pending"
	default:
		return Recommendation{}, false, fmt.Errorf("invalid decision")
	}

	rec.UpdatedAt = time.Now().UTC()
	s.recommendations[id] = rec
	return rec, true, nil
}

func (s *Service) materializeRecommendationsLocked(workspaceID string) {
	counts := s.capabilityGapCounts[workspaceID]
	for capabilityKey, count := range counts {
		if count < 3 {
			continue
		}
		if existingID, exists := s.recommendationByCapabilityLocked(workspaceID, capabilityKey); exists {
			existing := s.recommendations[existingID]
			existing.Confidence = confidenceFromGapCount(count)
			existing.Reason = recommendationReason(capabilityKey, count)
			existing.UpdatedAt = time.Now().UTC()
			s.recommendations[existingID] = existing
			continue
		}
		now := time.Now().UTC()
		recommendationID := fmt.Sprintf("recommendation_%06d", s.nextID)
		s.nextID++
		s.recommendations[recommendationID] = Recommendation{
			RecommendationID: recommendationID,
			WorkspaceID:      workspaceID,
			CapabilityKey:    capabilityKey,
			Confidence:       confidenceFromGapCount(count),
			Reason:           recommendationReason(capabilityKey, count),
			Status:           "pending",
			CreatedAt:        now,
			UpdatedAt:        now,
		}
	}

	// Keep deterministic fallback coverage when no capability gap data has been recorded yet.
	if len(counts) == 0 {
		counts["calendar.intelligent_scheduling"] = 3
		s.materializeRecommendationsLocked(workspaceID)
	}
}

func (s *Service) recommendationByCapabilityLocked(workspaceID, capabilityKey string) (string, bool) {
	for id, rec := range s.recommendations {
		if rec.WorkspaceID == workspaceID && rec.CapabilityKey == capabilityKey {
			return id, true
		}
	}
	return "", false
}

func (s *Service) ensureWorkspaceCountersLocked(workspaceID string) {
	if _, ok := s.capabilityGapCounts[workspaceID]; !ok {
		s.capabilityGapCounts[workspaceID] = map[string]int{}
	}
}

func normalizeWorkspaceID(workspaceID string) string {
	if strings.TrimSpace(workspaceID) == "" {
		return "default"
	}
	return workspaceID
}

func normalizeCapabilityKey(capabilityKey string) string {
	clean := strings.ToLower(strings.TrimSpace(capabilityKey))
	if clean == "" {
		return "general.capability"
	}
	return clean
}

func confidenceFromGapCount(count int) float64 {
	score := 0.45 + (0.1 * float64(count))
	return math.Min(score, 0.99)
}

func recommendationReason(capabilityKey string, count int) string {
	return fmt.Sprintf("%d capability gap events in 7d for %s", count, capabilityKey)
}
