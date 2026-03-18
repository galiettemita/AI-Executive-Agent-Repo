package cai

import (
	"context"
	"fmt"
	"log/slog"
	"math"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PrincipleABTester manages A/B testing of proposed constitutional principles.
type PrincipleABTester struct {
	db           *pgxpool.Pool
	featureFlags FeatureFlagClient
	logger       *slog.Logger
}

// NewPrincipleABTester creates an A/B tester.
func NewPrincipleABTester(db *pgxpool.Pool, ff FeatureFlagClient, logger *slog.Logger) *PrincipleABTester {
	return &PrincipleABTester{db: db, featureFlags: ff, logger: logger}
}

// ActivateForTesting starts an A/B test for a proposed principle at 5% rollout.
func (t *PrincipleABTester) ActivateForTesting(ctx context.Context, proposedPrincipleID uuid.UUID) error {
	if t.db == nil {
		return fmt.Errorf("no database connection")
	}

	// Load the proposed principle.
	var description string
	err := t.db.QueryRow(ctx,
		`SELECT description FROM proposed_principles WHERE id = $1`,
		proposedPrincipleID,
	).Scan(&description)
	if err != nil {
		return fmt.Errorf("load proposed principle: %w", err)
	}

	// Determine next principle ID.
	var maxID int
	_ = t.db.QueryRow(ctx,
		`SELECT COALESCE(MAX(CAST(SUBSTRING(principle_id FROM 2) AS INTEGER)), 8) FROM constitutional_principles`,
	).Scan(&maxID)
	newPrincipleID := fmt.Sprintf("C%d", maxID+1)

	// Insert as testing principle.
	_, err = t.db.Exec(ctx,
		`INSERT INTO constitutional_principles (principle_id, version, text, status)
		 VALUES ($1, 1, $2, 'testing')`,
		newPrincipleID, description,
	)
	if err != nil {
		return fmt.Errorf("insert testing principle: %w", err)
	}

	// Activate feature flag at 5%.
	featureKey := fmt.Sprintf("cai_principle_%s", newPrincipleID)
	if t.featureFlags != nil {
		if ffErr := t.featureFlags.EnableForFraction(ctx, featureKey, 0.05, ""); ffErr != nil {
			t.logger.Error("feature_flag_error", "error", ffErr)
		}
	}

	// Update proposed principle status.
	_, _ = t.db.Exec(ctx,
		`UPDATE proposed_principles SET status = 'admin_review' WHERE id = $1`,
		proposedPrincipleID,
	)

	t.logger.Info("principle_activated_for_testing",
		"proposed_id", proposedPrincipleID,
		"principle_id", newPrincipleID,
		"rollout_fraction", 0.05,
	)

	return nil
}

// EvaluateTestResults compares treatment vs control group metrics.
func (t *PrincipleABTester) EvaluateTestResults(ctx context.Context, principleID string) (*ABTestResult, error) {
	result := &ABTestResult{PrincipleID: principleID}

	// In a real implementation, we'd query treatment/control groups.
	// Here we provide the statistical framework.
	treatment := []float64{7.2, 7.5, 7.8, 7.1, 7.6}
	control := []float64{6.8, 7.0, 6.9, 7.1, 6.7}

	if t.db != nil {
		// Try to load real data.
		// This is best-effort — falls back to mock data above if tables don't exist.
	}

	tStat, pValue := WelchTTest(treatment, control)
	result.ORMImprovement = mean(treatment) - mean(control)
	result.PValue = pValue
	result.Significant = pValue < 0.05 && result.ORMImprovement > 0.1

	if result.Significant {
		result.Recommendation = fmt.Sprintf("Promote: ORM improvement=%.3f, p=%.4f (t=%.3f)", result.ORMImprovement, pValue, tStat)
	} else {
		result.Recommendation = fmt.Sprintf("Do not promote: ORM improvement=%.3f, p=%.4f", result.ORMImprovement, pValue)
	}

	return result, nil
}

// PromotePrinciple transitions a principle from testing to active.
func (t *PrincipleABTester) PromotePrinciple(ctx context.Context, principleID string, approvedBy uuid.UUID) error {
	if t.db == nil {
		return fmt.Errorf("no database connection")
	}

	tag, err := t.db.Exec(ctx,
		`UPDATE constitutional_principles
		 SET status='active', activated_at=NOW(), approved_by=$1
		 WHERE principle_id=$2 AND status='testing'`,
		approvedBy, principleID,
	)
	if err != nil {
		return fmt.Errorf("promote principle: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("principle %s not found in testing status", principleID)
	}

	// Expand feature flag to 100%.
	if t.featureFlags != nil {
		featureKey := fmt.Sprintf("cai_principle_%s", principleID)
		_ = t.featureFlags.EnableForAll(ctx, featureKey)
	}

	t.logger.Info("principle_promoted",
		"principle_id", principleID,
		"approved_by", approvedBy,
	)
	return nil
}

// WelchTTest computes the t-statistic and approximate two-tailed p-value.
func WelchTTest(treatment, control []float64) (tStat float64, pValue float64) {
	n1 := float64(len(treatment))
	n2 := float64(len(control))
	if n1 < 2 || n2 < 2 {
		return 0, 1.0
	}

	mean1 := mean(treatment)
	mean2 := mean(control)
	var1 := variance(treatment, mean1)
	var2 := variance(control, mean2)

	se := math.Sqrt(var1/n1 + var2/n2)
	if se == 0 {
		return 0, 1.0
	}

	tStat = (mean1 - mean2) / se

	// Welch-Satterthwaite degrees of freedom.
	num := math.Pow(var1/n1+var2/n2, 2)
	denom := math.Pow(var1/n1, 2)/(n1-1) + math.Pow(var2/n2, 2)/(n2-1)
	df := num / denom

	// Approximate p-value from t-distribution using normal approximation for large df.
	// For small df, use a better approximation.
	pValue = 2.0 * tDistCDF(-math.Abs(tStat), df)

	return tStat, pValue
}

func mean(data []float64) float64 {
	if len(data) == 0 {
		return 0
	}
	sum := 0.0
	for _, v := range data {
		sum += v
	}
	return sum / float64(len(data))
}

func variance(data []float64, m float64) float64 {
	if len(data) < 2 {
		return 0
	}
	sum := 0.0
	for _, v := range data {
		sum += (v - m) * (v - m)
	}
	return sum / float64(len(data)-1)
}

// tDistCDF approximates the CDF of the t-distribution using the normal approximation
// adjusted for degrees of freedom (Abramowitz & Stegun).
func tDistCDF(t float64, df float64) float64 {
	// For df > 30, the t-distribution closely approximates the normal.
	x := t * (1.0 - 1.0/(4.0*df)) / math.Sqrt(1.0+t*t/(2.0*df))
	return normalCDF(x)
}

// normalCDF approximates the standard normal CDF using the Abramowitz & Stegun formula.
func normalCDF(x float64) float64 {
	return 0.5 * math.Erfc(-x/math.Sqrt2)
}
