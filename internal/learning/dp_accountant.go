package learning

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrPrivacyBudgetExhausted is returned when a workspace's cumulative epsilon
// exceeds the maximum allowed (default 10.0).
var ErrPrivacyBudgetExhausted = errors.New("differential privacy budget exhausted")

// Default RDP alpha values for privacy accounting (Mironov 2017).
var defaultAlphas = []float64{2, 4, 8, 16, 32, 64}

// DP-SGD constants for the DPO pipeline.
const (
	DPOSigma         = 1.0   // noise multiplier for DP-SGD
	DPOSamplingRate  = 0.01  // sampling rate q = batch_size / dataset_size
	DPOClipNorm      = 1.0   // L2 gradient clipping norm
	DPOTargetEpsilon = 3.0   // per-round epsilon target
	DPODelta         = 1e-5  // delta for (epsilon, delta)-DP
	EpsilonAlertThreshold = 8.0  // emit alert when cumulative epsilon exceeds this
	EpsilonMaxDefault     = 10.0 // halt DPO when cumulative epsilon exceeds this
)

// PrivacyBudget tracks the differential privacy budget for a workspace.
type PrivacyBudget struct {
	WorkspaceID       uuid.UUID `json:"workspace_id"`
	CumulativeEpsilon float64   `json:"cumulative_epsilon"`
	EpsilonMax        float64   `json:"epsilon_max"`
	DeltaTarget       float64   `json:"delta_target"`
	RoundsCompleted   int       `json:"rounds_completed"`
	LastUpdatedAt     time.Time `json:"last_updated_at"`
	Halted            bool      `json:"halted"`
}

// RDPAccountant implements Rényi Differential Privacy accounting for DP-SGD
// training, with per-workspace budget tracking and enforcement.
type RDPAccountant struct {
	db     *pgxpool.Pool
	logger *slog.Logger
	// alertFn is called when the privacy budget approaches the limit.
	// Injected at construction for testability.
	alertFn func(workspaceID uuid.UUID, epsilon float64)
}

// NewRDPAccountant creates an accountant backed by the given database pool.
func NewRDPAccountant(db *pgxpool.Pool, logger *slog.Logger) *RDPAccountant {
	return &RDPAccountant{
		db:     db,
		logger: logger,
		alertFn: func(wsID uuid.UUID, eps float64) {
			logger.Warn("dp_budget_alert",
				"workspace_id", wsID,
				"cumulative_epsilon", eps,
				"message", fmt.Sprintf("DP budget approaching limit for workspace %s", wsID),
			)
		},
	}
}

// ComputeRDPEpsilon implements Rényi Differential Privacy composition for DP-SGD.
// Uses the simplified Gaussian mechanism RDP bound (Mironov 2017):
//
//	epsilon_rdp(alpha) = alpha / (2 * sigma^2) * q^2 * T
//
// Then converts to (epsilon, delta)-DP via:
//
//	epsilon = min over alphas of [epsilon_rdp(alpha) + log(1/delta) / (alpha - 1)]
func ComputeRDPEpsilon(sigma float64, samplingRate float64, numSteps int, alphas []float64) float64 {
	if len(alphas) == 0 {
		alphas = defaultAlphas
	}

	delta := DPODelta
	minEpsilon := math.Inf(1)

	for _, alpha := range alphas {
		if alpha <= 1.0 {
			continue
		}
		// RDP guarantee for Gaussian mechanism under Poisson subsampling.
		epsilonRDP := (alpha / (2.0 * sigma * sigma)) * samplingRate * samplingRate * float64(numSteps)

		// Convert RDP to (epsilon, delta) via optimal conversion.
		epsilon := epsilonRDP + math.Log(1.0/delta)/(alpha-1.0)

		if epsilon < minEpsilon {
			minEpsilon = epsilon
		}
	}

	if math.IsInf(minEpsilon, 1) {
		return 0.0
	}
	return minEpsilon
}

// RecordRound computes the epsilon for a training round and updates the workspace's
// cumulative privacy budget. Returns ErrPrivacyBudgetExhausted if the budget is exceeded.
func (a *RDPAccountant) RecordRound(ctx context.Context, workspaceID uuid.UUID, sigma float64, samplingRate float64, numSteps int) error {
	roundEpsilon := ComputeRDPEpsilon(sigma, samplingRate, numSteps, defaultAlphas)

	budget, err := a.GetBudget(ctx, workspaceID)
	if err != nil {
		return fmt.Errorf("get budget: %w", err)
	}

	if budget == nil {
		budget = &PrivacyBudget{
			WorkspaceID:       workspaceID,
			CumulativeEpsilon: 0,
			EpsilonMax:        EpsilonMaxDefault,
			DeltaTarget:       DPODelta,
			RoundsCompleted:   0,
			Halted:            false,
		}
	}

	if budget.Halted {
		return ErrPrivacyBudgetExhausted
	}

	budget.CumulativeEpsilon += roundEpsilon
	budget.RoundsCompleted++
	budget.LastUpdatedAt = time.Now()

	// Alert at 80% of max.
	if budget.CumulativeEpsilon > EpsilonAlertThreshold && !budget.Halted {
		a.alertFn(workspaceID, budget.CumulativeEpsilon)
	}

	// Halt if budget exceeded.
	if budget.CumulativeEpsilon > budget.EpsilonMax {
		budget.Halted = true
		a.logger.Warn("dp_budget_exhausted",
			"workspace_id", workspaceID,
			"cumulative_epsilon", budget.CumulativeEpsilon,
		)
	}

	// Upsert budget.
	if err := a.upsertBudget(ctx, budget); err != nil {
		return fmt.Errorf("upsert budget: %w", err)
	}

	a.logger.Info("dp_round_recorded",
		"workspace_id", workspaceID,
		"round_epsilon", roundEpsilon,
		"cumulative_epsilon", budget.CumulativeEpsilon,
		"rounds_completed", budget.RoundsCompleted,
		"halted", budget.Halted,
	)

	if budget.Halted {
		return ErrPrivacyBudgetExhausted
	}
	return nil
}

// GetBudget returns the privacy budget for a workspace, or nil if none exists.
func (a *RDPAccountant) GetBudget(ctx context.Context, workspaceID uuid.UUID) (*PrivacyBudget, error) {
	if a.db == nil {
		return nil, nil
	}

	var b PrivacyBudget
	err := a.db.QueryRow(ctx,
		`SELECT workspace_id, cumulative_epsilon, epsilon_max, delta_target,
		        rounds_completed, halted, last_updated_at
		 FROM workspace_dp_budgets WHERE workspace_id = $1`,
		workspaceID,
	).Scan(&b.WorkspaceID, &b.CumulativeEpsilon, &b.EpsilonMax, &b.DeltaTarget,
		&b.RoundsCompleted, &b.Halted, &b.LastUpdatedAt)

	if err != nil {
		if err.Error() == "no rows in result set" {
			return nil, nil
		}
		return nil, fmt.Errorf("query budget: %w", err)
	}

	return &b, nil
}

// IsHalted returns whether DPO is halted for a workspace due to budget exhaustion.
func (a *RDPAccountant) IsHalted(ctx context.Context, workspaceID uuid.UUID) (bool, error) {
	budget, err := a.GetBudget(ctx, workspaceID)
	if err != nil {
		return false, err
	}
	if budget == nil {
		return false, nil
	}
	return budget.Halted, nil
}

// ResetBudget resets the privacy budget for a workspace (admin-only operation).
func (a *RDPAccountant) ResetBudget(ctx context.Context, workspaceID uuid.UUID) error {
	if a.db == nil {
		return fmt.Errorf("no database connection")
	}

	_, err := a.db.Exec(ctx,
		`UPDATE workspace_dp_budgets
		 SET cumulative_epsilon = 0, halted = false, rounds_completed = 0, last_updated_at = NOW()
		 WHERE workspace_id = $1`,
		workspaceID,
	)
	if err != nil {
		return fmt.Errorf("reset budget: %w", err)
	}

	a.logger.Info("dp_budget_reset", "workspace_id", workspaceID)
	return nil
}

func (a *RDPAccountant) upsertBudget(ctx context.Context, b *PrivacyBudget) error {
	if a.db == nil {
		return nil
	}

	_, err := a.db.Exec(ctx,
		`INSERT INTO workspace_dp_budgets (workspace_id, cumulative_epsilon, epsilon_max, delta_target, rounds_completed, halted, last_updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)
		 ON CONFLICT (workspace_id) DO UPDATE SET
		   cumulative_epsilon = EXCLUDED.cumulative_epsilon,
		   rounds_completed = EXCLUDED.rounds_completed,
		   halted = EXCLUDED.halted,
		   last_updated_at = EXCLUDED.last_updated_at`,
		b.WorkspaceID, b.CumulativeEpsilon, b.EpsilonMax, b.DeltaTarget,
		b.RoundsCompleted, b.Halted, b.LastUpdatedAt,
	)
	return err
}
