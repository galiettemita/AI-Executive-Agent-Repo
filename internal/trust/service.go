package trust

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
	"time"
)

type TrustScore struct {
	WorkspaceID        string  `json:"workspace_id"`
	Score              float64 `json:"score"`
	SuccessCount30d    int     `json:"success_count_30d"`
	FailureCount30d    int     `json:"failure_count_30d"`
	OverrideCount30d   int     `json:"override_count_30d"`
	Trailing14dFailure int     `json:"trailing_14d_failure"`
	CurrentAutonomy    string  `json:"current_autonomy"`
	PromotionEligible  bool    `json:"promotion_eligible"`
}

type Promotion struct {
	ID           string    `json:"id"`
	WorkspaceID  string    `json:"workspace_id"`
	FromAutonomy string    `json:"from_autonomy"`
	ToAutonomy   string    `json:"to_autonomy"`
	Status       string    `json:"status"`
	CreatedAt    time.Time `json:"created_at"`
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

func computeTrustScore(successCount30d, failureCount30d, overrideCount30d int) float64 {
	denominator := successCount30d + failureCount30d + overrideCount30d
	if denominator < 1 {
		denominator = 1
	}
	score := float64(successCount30d-2*failureCount30d-3*overrideCount30d) / float64(denominator)
	return math.Round(score*10000) / 10000
}

func isPromotionEligible(score float64, successCount30d, trailing14dFailure int) bool {
	return score >= 0.85 && successCount30d >= 20 && trailing14dFailure == 0
}

func nextAutonomyLevel(current string) string {
	switch strings.ToUpper(strings.TrimSpace(current)) {
	case "A0":
		return "A1"
	case "A1":
		return "A2"
	case "A2":
		return "A3"
	case "A3":
		return "A4"
	default:
		return "A4"
	}
}

func (s *Service) UpsertScore(score TrustScore) TrustScore {
	s.mu.Lock()
	defer s.mu.Unlock()
	if score.WorkspaceID == "" {
		score.WorkspaceID = "default"
	}
	if score.CurrentAutonomy == "" {
		score.CurrentAutonomy = "A1"
	}
	score.Score = computeTrustScore(score.SuccessCount30d, score.FailureCount30d, score.OverrideCount30d)
	score.PromotionEligible = isPromotionEligible(score.Score, score.SuccessCount30d, score.Trailing14dFailure)
	s.scores[score.WorkspaceID] = score
	return score
}

func (s *Service) RecalculateScore(workspaceID string, successCount30d, failureCount30d, overrideCount30d, trailing14dFailure int, currentAutonomy string) TrustScore {
	score := s.UpsertScore(TrustScore{
		WorkspaceID:        workspaceID,
		SuccessCount30d:    successCount30d,
		FailureCount30d:    failureCount30d,
		OverrideCount30d:   overrideCount30d,
		Trailing14dFailure: trailing14dFailure,
		CurrentAutonomy:    currentAutonomy,
	})
	if score.PromotionEligible {
		s.AddPromotion(Promotion{WorkspaceID: workspaceID, FromAutonomy: currentAutonomy, ToAutonomy: nextAutonomyLevel(currentAutonomy)})
	}
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
	if promotion.WorkspaceID == "" {
		promotion.WorkspaceID = "default"
	}
	if promotion.FromAutonomy == "" {
		promotion.FromAutonomy = "A1"
	}
	if promotion.ToAutonomy == "" {
		promotion.ToAutonomy = nextAutonomyLevel(promotion.FromAutonomy)
	}
	if promotion.Status == "" {
		promotion.Status = "pending"
	}
	promotion.ID = fmt.Sprintf("promotion_%06d", s.nextID)
	s.nextID++
	if promotion.CreatedAt.IsZero() {
		promotion.CreatedAt = time.Now().UTC()
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
	if promotion.Status != "pending" {
		return promotion, true
	}
	if strings.EqualFold(decision, "approve") {
		promotion.Status = "approved"
	} else {
		promotion.Status = "denied"
	}
	s.promotions[id] = promotion
	return promotion, true
}
