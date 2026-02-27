package exploration

import (
	"fmt"
	"sort"
	"sync"
)

type Recommendation struct {
	ID            string `json:"id"`
	WorkspaceID   string `json:"workspace_id"`
	CapabilityKey string `json:"capability_key"`
	Status        string `json:"status"`
	Reason        string `json:"reason"`
}

type Service struct {
	mu              sync.RWMutex
	nextID          int
	recommendations map[string]Recommendation
}

func NewService() *Service {
	return &Service{
		nextID:          1,
		recommendations: map[string]Recommendation{},
	}
}

func (s *Service) ListRecommendations(workspaceID string) []Recommendation {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.recommendations) == 0 {
		rec := Recommendation{
			ID:            fmt.Sprintf("recommendation_%06d", s.nextID),
			WorkspaceID:   workspaceID,
			CapabilityKey: "calendar.intelligent_scheduling",
			Status:        "pending",
			Reason:        "frequent scheduling requests detected",
		}
		s.nextID++
		s.recommendations[rec.ID] = rec
	}
	out := make([]Recommendation, 0, len(s.recommendations))
	for _, rec := range s.recommendations {
		if workspaceID != "" && rec.WorkspaceID != workspaceID {
			continue
		}
		out = append(out, rec)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].ID < out[j].ID
	})
	return out
}

func (s *Service) DecideRecommendation(id, decision string) (Recommendation, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rec, ok := s.recommendations[id]
	if !ok {
		return Recommendation{}, false
	}
	if decision == "accept" {
		rec.Status = "accepted"
	} else {
		rec.Status = "rejected"
	}
	s.recommendations[id] = rec
	return rec, true
}
