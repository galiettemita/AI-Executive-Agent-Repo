package watermark

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ProvenanceStore persists and queries AI content provenance records.
type ProvenanceStore struct {
	db     *pgxpool.Pool
	logger *slog.Logger
}

// NewProvenanceStore creates a store backed by the given database pool.
func NewProvenanceStore(db *pgxpool.Pool, logger *slog.Logger) *ProvenanceStore {
	return &ProvenanceStore{db: db, logger: logger}
}

// Record inserts a provenance record into ai_content_provenance.
// Idempotent: uses ON CONFLICT (request_id) DO NOTHING.
func (s *ProvenanceStore) Record(ctx context.Context, originalText, watermarkedText string, meta WatermarkMeta) error {
	if s.db == nil {
		s.logger.Warn("provenance_record_skipped", "reason", "no database")
		return nil
	}

	contentHash := ContentHash(originalText)
	watermarkHash := ContentHash(watermarkedText)

	_, err := s.db.Exec(ctx,
		`INSERT INTO ai_content_provenance (request_id, workspace_id, model_id, timestamp, watermark_hash, content_hash, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, NOW())
		 ON CONFLICT (request_id) DO NOTHING`,
		meta.RequestID, meta.WorkspaceID, meta.ModelID, meta.Timestamp,
		watermarkHash, contentHash,
	)
	if err != nil {
		return fmt.Errorf("insert provenance: %w", err)
	}

	s.logger.Info("provenance_recorded",
		"request_id", meta.RequestID,
		"workspace_id", meta.WorkspaceID,
	)
	return nil
}

// LookupByRequestID retrieves a provenance record by request ID.
func (s *ProvenanceStore) LookupByRequestID(ctx context.Context, requestID uuid.UUID) (*ProvenanceRecord, error) {
	if s.db == nil {
		return nil, fmt.Errorf("no database connection")
	}

	var rec ProvenanceRecord
	err := s.db.QueryRow(ctx,
		`SELECT request_id, workspace_id, model_id, timestamp, watermark_hash, content_hash
		 FROM ai_content_provenance WHERE request_id = $1`,
		requestID,
	).Scan(&rec.RequestID, &rec.WorkspaceID, &rec.ModelID, &rec.Timestamp, &rec.WatermarkHash, &rec.ContentHash)
	if err != nil {
		return nil, fmt.Errorf("lookup by request_id: %w", err)
	}

	return &rec, nil
}

// LookupByContentHash retrieves a provenance record by content hash.
func (s *ProvenanceStore) LookupByContentHash(ctx context.Context, contentHash string) (*ProvenanceRecord, error) {
	if s.db == nil {
		return nil, fmt.Errorf("no database connection")
	}

	var rec ProvenanceRecord
	err := s.db.QueryRow(ctx,
		`SELECT request_id, workspace_id, model_id, timestamp, watermark_hash, content_hash
		 FROM ai_content_provenance WHERE content_hash = $1
		 ORDER BY created_at DESC LIMIT 1`,
		contentHash,
	).Scan(&rec.RequestID, &rec.WorkspaceID, &rec.ModelID, &rec.Timestamp, &rec.WatermarkHash, &rec.ContentHash)
	if err != nil {
		return nil, fmt.Errorf("lookup by content_hash: %w", err)
	}

	return &rec, nil
}

// ContentHash computes SHA-256 hex of text.
func ContentHash(text string) string {
	h := sha256.Sum256([]byte(text))
	return hex.EncodeToString(h[:])
}
