package watermark

import (
	"time"

	"github.com/google/uuid"
)

// WatermarkMeta carries the metadata to be embedded in the watermark.
type WatermarkMeta struct {
	ModelID     string    `json:"model_id"`
	WorkspaceID uuid.UUID `json:"workspace_id"`
	RequestID   uuid.UUID `json:"request_id"`
	Timestamp   time.Time `json:"timestamp"`
}

// WatermarkVerification is the result of a watermark verification check.
type WatermarkVerification struct {
	IsBrevioGenerated bool      `json:"is_brevio_generated"`
	WorkspaceID       uuid.UUID `json:"workspace_id"`
	ModelID           string    `json:"model_id"`
	RequestID         uuid.UUID `json:"request_id"`
	Confidence        float64   `json:"confidence"`
}

// ProvenanceRecord represents a row in the ai_content_provenance table.
type ProvenanceRecord struct {
	RequestID     uuid.UUID `json:"request_id"`
	WorkspaceID   uuid.UUID `json:"workspace_id"`
	ModelID       string    `json:"model_id"`
	Timestamp     time.Time `json:"timestamp"`
	WatermarkHash string    `json:"watermark_hash"`
	ContentHash   string    `json:"content_hash"`
}
