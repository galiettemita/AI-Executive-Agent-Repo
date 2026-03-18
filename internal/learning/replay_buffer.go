package learning

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	learndpo "github.com/brevio/brevio/internal/learning/dpo"
)

// ReplayConfig controls stratified replay sampling.
type ReplayConfig struct {
	TotalBatchSize int
	ReplayFraction float64  // 0.10 = 10% of batch from replay
	Domains        []string
}

// StratifiedReplayBuffer samples preference pairs stratified by domain
// for replay during DPO training to prevent catastrophic forgetting.
type StratifiedReplayBuffer struct {
	db     *pgxpool.Pool
	logger *slog.Logger
}

// NewStratifiedReplayBuffer creates a replay buffer.
func NewStratifiedReplayBuffer(db *pgxpool.Pool, logger *slog.Logger) *StratifiedReplayBuffer {
	return &StratifiedReplayBuffer{db: db, logger: logger}
}

// SampleReplayBatch returns a stratified sample of preference pairs for replay.
func (b *StratifiedReplayBuffer) SampleReplayBatch(ctx context.Context, workspaceID uuid.UUID, config ReplayConfig) ([]learndpo.PreferencePair, error) {
	if b.db == nil || len(config.Domains) == 0 {
		return nil, nil
	}

	replayCount := int(float64(config.TotalBatchSize) * config.ReplayFraction)
	if replayCount < 1 {
		return nil, nil
	}

	samplesPerDomain := replayCount / len(config.Domains)
	if samplesPerDomain < 1 {
		samplesPerDomain = 1
	}

	var allPairs []learndpo.PreferencePair

	for _, domain := range config.Domains {
		rows, err := b.db.Query(ctx,
			`SELECT id, workspace_id, prompt_text, chosen_response, rejected_response
			 FROM preference_pairs
			 WHERE workspace_id=$1 AND used_in_round IS NOT NULL
			 ORDER BY RANDOM() LIMIT $2`,
			workspaceID, samplesPerDomain,
		)
		if err != nil {
			b.logger.Warn("replay_sample_error", "domain", domain, "error", err)
			continue
		}

		for rows.Next() {
			var pair learndpo.PreferencePair
			if err := rows.Scan(&pair.ID, &pair.WorkspaceID, &pair.PromptText,
				&pair.ChosenResponse, &pair.RejectedResponse); err != nil {
				continue
			}
			allPairs = append(allPairs, pair)
		}
		rows.Close()
	}

	b.logger.Info("replay_batch_sampled",
		"workspace_id", workspaceID,
		"requested", replayCount,
		"sampled", len(allPairs),
		"domains", len(config.Domains),
	)

	return allPairs, nil
}

// GetDomains returns distinct domains from preference pairs for a workspace.
func (b *StratifiedReplayBuffer) GetDomains(ctx context.Context, workspaceID uuid.UUID) ([]string, error) {
	if b.db == nil {
		return nil, nil
	}

	rows, err := b.db.Query(ctx,
		`SELECT DISTINCT COALESCE(signal_type, 'unknown') FROM preference_pairs WHERE workspace_id=$1`,
		workspaceID,
	)
	if err != nil {
		return nil, fmt.Errorf("get domains: %w", err)
	}
	defer rows.Close()

	var domains []string
	for rows.Next() {
		var d string
		if err := rows.Scan(&d); err == nil {
			domains = append(domains, d)
		}
	}
	return domains, rows.Err()
}
