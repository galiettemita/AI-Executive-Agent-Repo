package trust

import (
	"fmt"
	"sort"
	"sync"
)

type TrustScore struct {
	WorkspaceID      string  `json:"workspace_id"`
	Score            float64 `json:"score"`
	SuccessCount30d  int     `json:"success_count_30d"`
	FailureCount30d  int     `json:"failure_count_30d"`
	OverrideCount30d int     `json:"override_count_30d"`
}

type Promotion struct {
	ID           string `json:"id"`
	WorkspaceID  string `json:"workspace_id"`
	FromAutonomy string `json:"from_autonomy"`
	ToAutonomy   string `json:"to_autonomy"`
	Status       string `json:"status"`
}

type Service struct {
	mu         sync.RWMutex
	nextID     int
	scores     map[string]TrustScore
	promotions map[string]Promotion
}

func NewService() *Service {
	return &Service{
		nextID:     1,
		scores:     map[string]TrustScore{},
		promotions: map[string]Promotion{},
	}
}

func (s *Service) UpsertScore(score TrustScore) TrustScore {
	s.mu.Lock()
	defer s.mu.Unlock()
	if score.WorkspaceID == "" {
		score.WorkspaceID = "default"
	}
	s.scores[score.WorkspaceID] = score
	return score
}

func (s *Service) ListScores() []TrustScore {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]TrustScore, 0, len(s.scores))
	for _, score := range s.scores {
		out = append(out, score)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].WorkspaceID < out[j].WorkspaceID
	})
	return out
}

func (s *Service) AddPromotion(promotion Promotion) Promotion {
	s.mu.Lock()
	defer s.mu.Unlock()
	promotion.ID = fmt.Sprintf("promotion_%06d", s.nextID)
	s.nextID++
	if promotion.WorkspaceID == "" {
		promotion.WorkspaceID = "default"
	}
	if promotion.FromAutonomy == "" {
		promotion.FromAutonomy = "A1"
	}
	if promotion.ToAutonomy == "" {
		promotion.ToAutonomy = "A2"
	}
	if promotion.Status == "" {
		promotion.Status = "pending"
	}
	s.promotions[promotion.ID] = promotion
	return promotion
}

func (s *Service) ListPromotions() []Promotion {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Promotion, 0, len(s.promotions))
	for _, promotion := range s.promotions {
		out = append(out, promotion)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].ID < out[j].ID
	})
	return out
}

func (s *Service) DecidePromotion(id, decision string) (Promotion, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	promotion, ok := s.promotions[id]
	if !ok {
		return Promotion{}, false
	}
	if decision == "approve" {
		promotion.Status = "approved"
	} else {
		promotion.Status = "denied"
	}
	s.promotions[id] = promotion
	return promotion, true
}
