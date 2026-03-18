package federated

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
)

// FederatedAggregator performs federated averaging of noisy gradients.
type FederatedAggregator struct {
	db     *pgxpool.Pool
	logger *slog.Logger
}

// NewFederatedAggregator creates an aggregator.
func NewFederatedAggregator(db *pgxpool.Pool, logger *slog.Logger) *FederatedAggregator {
	return &FederatedAggregator{db: db, logger: logger}
}

// AggregateGradients computes element-wise mean across all workspace gradients.
func (a *FederatedAggregator) AggregateGradients(ctx context.Context, gradients []NoisyGradient) ([]float64, error) {
	if len(gradients) < 2 {
		return nil, ErrInsufficientParticipants
	}

	// Find max dimension.
	maxDim := 0
	for _, g := range gradients {
		if len(g.GradientVector) > maxDim {
			maxDim = len(g.GradientVector)
		}
	}

	if maxDim == 0 {
		return nil, fmt.Errorf("all gradient vectors are empty")
	}

	// Federated averaging: element-wise mean.
	aggregated := make([]float64, maxDim)
	for _, g := range gradients {
		for i, v := range g.GradientVector {
			aggregated[i] += v
		}
		// Shorter vectors implicitly contribute 0 for missing dimensions.
	}

	n := float64(len(gradients))
	for i := range aggregated {
		aggregated[i] /= n
	}

	a.logger.Info("gradients_aggregated",
		"participants", len(gradients),
		"dimensions", maxDim,
	)

	return aggregated, nil
}

// RecordFederatedRound inserts a round record into the database.
func (a *FederatedAggregator) RecordFederatedRound(ctx context.Context, participantCount, gradientDim int, maxEpsilon float64) error {
	if a.db == nil {
		return nil
	}

	_, err := a.db.Exec(ctx,
		`INSERT INTO federated_rounds (participating_count, gradient_dimensions, max_epsilon_used)
		 VALUES ($1, $2, $3)`,
		participantCount, gradientDim, maxEpsilon,
	)
	return err
}
