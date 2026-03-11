package contextlayer

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/brevio/brevio/internal/database"
)

// CompressionArtifactRow represents a persisted compression artifact.
type CompressionArtifactRow struct {
	ID                string   `json:"id"`
	WorkspaceID       string   `json:"workspace_id"`
	SessionID         string   `json:"session_id"`
	OriginalTurnCount int      `json:"original_turn_count"`
	CompressedCount   int      `json:"compressed_count"`
	EntityRefs        []string `json:"entity_refs"`
	SummaryText       string   `json:"summary_text"`
	TokenSavings      int      `json:"token_savings"`
}

// CompressionRepository persists compression artifacts for auditability.
type CompressionRepository interface {
	RecordCompression(ctx context.Context, row CompressionArtifactRow) error
	GetCompressionHistory(ctx context.Context, workspaceID, sessionID string, limit int) ([]CompressionArtifactRow, error)
	GetTotalTokenSavings(ctx context.Context, workspaceID string) (int, error)
}

// PgCompressionRepository implements CompressionRepository backed by pgx.
type PgCompressionRepository struct {
	q database.Querier
}

// NewPgCompressionRepository creates a new PgCompressionRepository.
func NewPgCompressionRepository(q database.Querier) *PgCompressionRepository {
	return &PgCompressionRepository{q: q}
}

// RecordCompression persists a compression artifact.
func (r *PgCompressionRepository) RecordCompression(ctx context.Context, row CompressionArtifactRow) error {
	entityJSON, _ := json.Marshal(row.EntityRefs)
	_, err := r.q.Exec(ctx,
		`INSERT INTO compression_artifacts (workspace_id, session_id, original_turn_count, compressed_count, entity_refs, summary_text, token_savings)
		 VALUES ($1::uuid, $2, $3, $4, $5, $6, $7)`,
		row.WorkspaceID, row.SessionID, row.OriginalTurnCount, row.CompressedCount, entityJSON, row.SummaryText, row.TokenSavings,
	)
	if err != nil {
		return fmt.Errorf("record compression: %w", err)
	}
	return nil
}

// GetCompressionHistory returns compression artifacts for a session.
func (r *PgCompressionRepository) GetCompressionHistory(ctx context.Context, workspaceID, sessionID string, limit int) ([]CompressionArtifactRow, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := r.q.Query(ctx,
		`SELECT id, workspace_id, session_id, original_turn_count, compressed_count, entity_refs, summary_text, token_savings
		 FROM compression_artifacts
		 WHERE workspace_id = $1::uuid AND session_id = $2
		 ORDER BY created_at DESC LIMIT $3`,
		workspaceID, sessionID, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("get compression history: %w", err)
	}
	defer rows.Close()

	var result []CompressionArtifactRow
	for rows.Next() {
		var c CompressionArtifactRow
		var entityJSON []byte
		if err := rows.Scan(&c.ID, &c.WorkspaceID, &c.SessionID, &c.OriginalTurnCount, &c.CompressedCount, &entityJSON, &c.SummaryText, &c.TokenSavings); err != nil {
			return nil, fmt.Errorf("scan compression artifact: %w", err)
		}
		_ = json.Unmarshal(entityJSON, &c.EntityRefs)
		result = append(result, c)
	}
	return result, rows.Err()
}

// GetTotalTokenSavings returns total tokens saved by compression for a workspace.
func (r *PgCompressionRepository) GetTotalTokenSavings(ctx context.Context, workspaceID string) (int, error) {
	var total int
	err := r.q.QueryRow(ctx,
		`SELECT COALESCE(SUM(token_savings), 0) FROM compression_artifacts WHERE workspace_id = $1::uuid`,
		workspaceID,
	).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("get total token savings: %w", err)
	}
	return total, nil
}
