package marketplace

import "context"

// AgentTrustScorer computes a trust score from operational statistics.
type AgentTrustScorer struct {
	outcomeRepo AgentOutcomeRepository
}

// NewAgentTrustScorer creates a new trust scorer.
func NewAgentTrustScorer(outcomeRepo AgentOutcomeRepository) *AgentTrustScorer {
	return &AgentTrustScorer{outcomeRepo: outcomeRepo}
}

// Compute returns a trust score in [0.0, 1.0] for the given agent.
// Formula (weighted):
//
//	trust = successRate*0.4 + respTimeScore*0.2 + (1-errorRate)*0.2 + adminRating*0.2
//
// Where:
//
//	respTimeScore = max(0, 1 - responseTimeP99Ms/10000)  // 10s = 0 score
//	adminRating   = 0.8 by default (neutral)
func (s *AgentTrustScorer) Compute(ctx context.Context, agentID string, adminRating float64) (float64, error) {
	stats, err := s.outcomeRepo.GetStats(ctx, agentID, 30)
	if err != nil {
		return 0.5, err
	}
	if stats.TotalCalls == 0 {
		return 0.5, nil
	}
	respTimeScore := clampF64(0, 1.0-float64(stats.ResponseTimeP99Ms)/10000.0)
	if adminRating <= 0 {
		adminRating = 0.8
	}
	score := stats.SuccessRate*0.4 +
		respTimeScore*0.2 +
		(1.0-stats.ErrorRate)*0.2 +
		adminRating*0.2
	if score > 1.0 {
		score = 1.0
	}
	if score < 0.0 {
		score = 0.0
	}
	return score, nil
}

func clampF64(min, val float64) float64 {
	if val < min {
		return min
	}
	return val
}
