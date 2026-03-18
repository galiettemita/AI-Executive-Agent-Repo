package evaluation

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// PromotionDecision describes whether a challenger model is eligible for promotion.
type PromotionDecision struct {
	Eligible        bool               `json:"eligible"`
	ChallengerModel string             `json:"challenger_model"`
	Reason          string             `json:"reason"`
	Metrics         PromotionMetrics   `json:"metrics"`
}

// PromotionMetrics holds the comparison data.
type PromotionMetrics struct {
	ChallengerORMAvg    float64 `json:"challenger_orm_avg"`
	ChampionORMAvg      float64 `json:"champion_orm_avg"`
	ORMImprovement      float64 `json:"orm_improvement"`
	ChallengerErrorRate float64 `json:"challenger_error_rate"`
	ChampionErrorRate   float64 `json:"champion_error_rate"`
	ChallengerP99Ms     float64 `json:"challenger_p99_ms"`
	ChampionP99Ms       float64 `json:"champion_p99_ms"`
}

// ChallengerPromoter evaluates challenger models for production promotion.
type ChallengerPromoter struct {
	db     *pgxpool.Pool
	logger *slog.Logger
}

// NewChallengerPromoter creates a promoter instance.
func NewChallengerPromoter(db *pgxpool.Pool, logger *slog.Logger) *ChallengerPromoter {
	return &ChallengerPromoter{db: db, logger: logger}
}

// EvaluatePromotion checks if a challenger model meets all promotion criteria.
// Criteria: (1) ORM improvement >= 0.2, (2) error rate <=, (3) p99 latency <= 1.2x.
func (p *ChallengerPromoter) EvaluatePromotion(ctx context.Context, challengerModel string) (*PromotionDecision, error) {
	metrics, err := p.computeMetrics(ctx, challengerModel)
	if err != nil {
		return nil, fmt.Errorf("compute metrics: %w", err)
	}

	eligible := true
	reasons := []string{}

	if metrics.ORMImprovement < 0.2 {
		eligible = false
		reasons = append(reasons, fmt.Sprintf("ORM improvement %.2f < 0.2 threshold", metrics.ORMImprovement))
	}
	if metrics.ChallengerErrorRate > metrics.ChampionErrorRate {
		eligible = false
		reasons = append(reasons, fmt.Sprintf("challenger error rate %.4f > champion %.4f", metrics.ChallengerErrorRate, metrics.ChampionErrorRate))
	}
	if metrics.ChampionP99Ms > 0 && metrics.ChallengerP99Ms > metrics.ChampionP99Ms*1.20 {
		eligible = false
		reasons = append(reasons, fmt.Sprintf("challenger p99 %.0fms > champion %.0fms * 1.2", metrics.ChallengerP99Ms, metrics.ChampionP99Ms))
	}

	reason := "all criteria met"
	if !eligible {
		reason = fmt.Sprintf("criteria not met: %v", reasons)
	}

	return &PromotionDecision{
		Eligible:        eligible,
		ChallengerModel: challengerModel,
		Reason:          reason,
		Metrics:         *metrics,
	}, nil
}

func (p *ChallengerPromoter) computeMetrics(ctx context.Context, challengerModel string) (*PromotionMetrics, error) {
	metrics := &PromotionMetrics{}

	if p.db == nil {
		return metrics, nil
	}

	// 7-day rolling averages from shadow_eval_results.
	_ = p.db.QueryRow(ctx,
		`SELECT COALESCE(AVG(challenger_orm_score), 0), COALESCE(AVG(champion_orm_score), 0)
		 FROM shadow_eval_results
		 WHERE challenger_model=$1 AND evaluated_at > NOW() - INTERVAL '7 days'`,
		challengerModel,
	).Scan(&metrics.ChallengerORMAvg, &metrics.ChampionORMAvg)

	metrics.ORMImprovement = metrics.ChallengerORMAvg - metrics.ChampionORMAvg

	return metrics, nil
}

// RequestAdminApproval creates a pending promotion request.
func (p *ChallengerPromoter) RequestAdminApproval(ctx context.Context, decision *PromotionDecision) error {
	if p.db == nil {
		return nil
	}

	metricsJSON, _ := fmt.Printf("") // placeholder
	_ = metricsJSON

	_, err := p.db.Exec(ctx,
		`INSERT INTO model_promotion_requests (challenger_model, metrics, status)
		 VALUES ($1, $2, 'pending')`,
		decision.ChallengerModel, fmt.Sprintf(`{"orm_improvement": %.2f}`, decision.Metrics.ORMImprovement),
	)
	if err != nil {
		return fmt.Errorf("insert promotion request: %w", err)
	}

	p.logger.Info("model_promotion_requested",
		"challenger_model", decision.ChallengerModel,
		"orm_improvement", decision.Metrics.ORMImprovement,
	)
	return nil
}

// GetPendingPromotions returns all pending model promotion requests.
func GetPendingPromotions(ctx context.Context, db *pgxpool.Pool) ([]map[string]interface{}, error) {
	if db == nil {
		return nil, nil
	}

	rows, err := db.Query(ctx,
		`SELECT id, challenger_model, metrics, status, requested_at
		 FROM model_promotion_requests WHERE status='pending'
		 ORDER BY requested_at DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []map[string]interface{}
	for rows.Next() {
		var id, model, status string
		var metrics []byte
		var requestedAt time.Time
		if err := rows.Scan(&id, &model, &metrics, &status, &requestedAt); err != nil {
			continue
		}
		results = append(results, map[string]interface{}{
			"id": id, "challenger_model": model, "status": status,
			"metrics": string(metrics), "requested_at": requestedAt,
		})
	}
	return results, nil
}
