package eu_ai_act

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// DataGovernanceLog implements the Article 10 data governance logging requirement.
type DataGovernanceLog struct {
	pool *pgxpool.Pool
}

// NewDataGovernanceLog creates a new governance log. Returns error if pool is nil.
func NewDataGovernanceLog(pool *pgxpool.Pool) (*DataGovernanceLog, error) {
	if pool == nil {
		return nil, fmt.Errorf("eu_ai_act.NewDataGovernanceLog: pool must not be nil")
	}
	return &DataGovernanceLog{pool: pool}, nil
}

// LogDataset records a dataset provenance entry.
func (d *DataGovernanceLog) LogDataset(ctx context.Context, entry DataGovernanceEntry) error {
	if entry.WorkspaceID == uuid.Nil {
		return fmt.Errorf("DataGovernanceLog.LogDataset: workspace_id required")
	}

	biasJSON, err := json.Marshal(entry.BiasIndicators)
	if err != nil {
		biasJSON = []byte("{}")
	}

	_, err = d.pool.Exec(ctx,
		`INSERT INTO eu_ai_act_data_governance
		 (workspace_id, dataset_name, provenance, quality_score, bias_indicators, dpo_pair_ref, logged_at, created_at)
		 VALUES ($1, $2, $3, $4, $5::jsonb, $6, now(), now())`,
		entry.WorkspaceID, entry.DatasetName, entry.Provenance,
		entry.QualityScore, string(biasJSON), entry.DPOPairRef,
	)
	if err != nil {
		return fmt.Errorf("DataGovernanceLog.LogDataset: %w", err)
	}
	return nil
}

// ListEntries returns all data governance entries for a workspace.
func (d *DataGovernanceLog) ListEntries(ctx context.Context, workspaceID uuid.UUID) ([]DataGovernanceEntry, error) {
	rows, err := d.pool.Query(ctx,
		`SELECT id, workspace_id, dataset_name, provenance, quality_score, bias_indicators, dpo_pair_ref, logged_at, created_at
		 FROM eu_ai_act_data_governance
		 WHERE workspace_id = $1
		 ORDER BY logged_at DESC`,
		workspaceID,
	)
	if err != nil {
		return nil, fmt.Errorf("DataGovernanceLog.ListEntries: %w", err)
	}
	defer rows.Close()

	var entries []DataGovernanceEntry
	for rows.Next() {
		var e DataGovernanceEntry
		var biasJSON []byte
		if err := rows.Scan(&e.ID, &e.WorkspaceID, &e.DatasetName, &e.Provenance,
			&e.QualityScore, &biasJSON, &e.DPOPairRef, &e.LoggedAt, &e.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("DataGovernanceLog.ListEntries: scan: %w", err)
		}
		if len(biasJSON) > 0 {
			_ = json.Unmarshal(biasJSON, &e.BiasIndicators)
		}
		if e.BiasIndicators == nil {
			e.BiasIndicators = map[string]interface{}{}
		}
		entries = append(entries, e)
	}
	if entries == nil {
		entries = []DataGovernanceEntry{}
	}
	return entries, nil
}
