package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/brevio/brevio/internal/database"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// IngressTurnRepository persists ingress turns and identity envelopes.
type IngressTurnRepository interface {
	// InsertTurn persists an ingress turn, returning false if the dedup_hash already exists.
	InsertTurn(ctx context.Context, turn *IngressTurn) (inserted bool, err error)

	// GetTurn retrieves an ingress turn by ID.
	GetTurn(ctx context.Context, turnID uuid.UUID) (*IngressTurn, error)

	// InsertIdentityEnvelope persists a channel identity envelope.
	InsertIdentityEnvelope(ctx context.Context, workspaceID uuid.UUID, envelope *ChannelIdentityEnvelope) error
}

// IdempotencyRepository provides DB-backed HTTP response caching for idempotency.
type IdempotencyRepository interface {
	// Get retrieves a cached response by idempotency key. Returns ok=false if not found or expired.
	Get(ctx context.Context, key string) (statusCode int, body []byte, ok bool)

	// Set stores a cached response with a TTL.
	Set(ctx context.Context, key string, statusCode int, body []byte, ttl time.Duration) error
}

// PgIngressTurnRepository implements IngressTurnRepository using pgx.
type PgIngressTurnRepository struct {
	db database.Querier
}

var _ IngressTurnRepository = (*PgIngressTurnRepository)(nil)

// NewPgIngressTurnRepository creates a new pgx-backed ingress turn repository.
func NewPgIngressTurnRepository(db database.Querier) *PgIngressTurnRepository {
	return &PgIngressTurnRepository{db: db}
}

func (r *PgIngressTurnRepository) InsertTurn(ctx context.Context, turn *IngressTurn) (bool, error) {
	attachmentsJSON, err := json.Marshal(turn.Attachments)
	if err != nil {
		return false, fmt.Errorf("marshal attachments: %w", err)
	}

	tag, err := r.db.Exec(ctx, `
		INSERT INTO ingress_turns (
			id, workspace_id, user_channel_id, dedup_hash, raw_payload,
			parsed_interactive_reply, parsed_discovery_answer, transcript,
			attachments, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		ON CONFLICT (workspace_id, dedup_hash) DO NOTHING`,
		turn.ID, turn.WorkspaceID, turn.UserChannelID, turn.DedupHash, turn.Payload,
		turn.ParsedInteractiveReply, turn.ParsedDiscoveryAnswer, turn.Transcript,
		attachmentsJSON, turn.CreatedAt,
	)
	if err != nil {
		return false, fmt.Errorf("insert ingress turn: %w", err)
	}
	return tag.RowsAffected() > 0, nil
}

func (r *PgIngressTurnRepository) GetTurn(ctx context.Context, turnID uuid.UUID) (*IngressTurn, error) {
	row := r.db.QueryRow(ctx, `
		SELECT id, workspace_id, user_channel_id, dedup_hash, raw_payload,
			parsed_interactive_reply, parsed_discovery_answer, transcript,
			attachments, created_at
		FROM ingress_turns
		WHERE id = $1`, turnID)

	var turn IngressTurn
	var attachmentsJSON []byte
	err := row.Scan(
		&turn.ID, &turn.WorkspaceID, &turn.UserChannelID, &turn.DedupHash, &turn.Payload,
		&turn.ParsedInteractiveReply, &turn.ParsedDiscoveryAnswer, &turn.Transcript,
		&attachmentsJSON, &turn.CreatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("ingress turn not found: %s", turnID)
		}
		return nil, fmt.Errorf("get ingress turn: %w", err)
	}

	if len(attachmentsJSON) > 0 {
		if err := json.Unmarshal(attachmentsJSON, &turn.Attachments); err != nil {
			return nil, fmt.Errorf("unmarshal attachments: %w", err)
		}
	}
	return &turn, nil
}

func (r *PgIngressTurnRepository) InsertIdentityEnvelope(ctx context.Context, workspaceID uuid.UUID, envelope *ChannelIdentityEnvelope) error {
	envelopeJSON, err := json.Marshal(envelope)
	if err != nil {
		return fmt.Errorf("marshal identity envelope: %w", err)
	}

	_, err = r.db.Exec(ctx, `
		INSERT INTO channel_identity_envelopes (workspace_id, ingress_turn_id, envelope, created_at)
		VALUES ($1, $2, $3, $4)`,
		workspaceID, envelope.IngressTurnID, envelopeJSON, envelope.VerifiedAt,
	)
	if err != nil {
		return fmt.Errorf("insert identity envelope: %w", err)
	}
	return nil
}

// PgIdempotencyRepository implements IdempotencyRepository using pgx.
type PgIdempotencyRepository struct {
	db database.Querier
}

var _ IdempotencyRepository = (*PgIdempotencyRepository)(nil)

// NewPgIdempotencyRepository creates a new pgx-backed idempotency repository.
func NewPgIdempotencyRepository(db database.Querier) *PgIdempotencyRepository {
	return &PgIdempotencyRepository{db: db}
}

func (r *PgIdempotencyRepository) Get(ctx context.Context, key string) (int, []byte, bool) {
	row := r.db.QueryRow(ctx, `
		SELECT status_code, response_body FROM gateway_idempotency_cache
		WHERE idempotency_key = $1 AND expires_at > now()`, key)

	var statusCode int
	var body []byte
	if err := row.Scan(&statusCode, &body); err != nil {
		return 0, nil, false
	}
	return statusCode, body, true
}

func (r *PgIdempotencyRepository) Set(ctx context.Context, key string, statusCode int, body []byte, ttl time.Duration) error {
	expiresAt := time.Now().UTC().Add(ttl)
	_, err := r.db.Exec(ctx, `
		INSERT INTO gateway_idempotency_cache (idempotency_key, status_code, response_body, expires_at)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (idempotency_key) DO UPDATE SET
			status_code = EXCLUDED.status_code,
			response_body = EXCLUDED.response_body,
			expires_at = EXCLUDED.expires_at`,
		key, statusCode, body, expiresAt,
	)
	if err != nil {
		return fmt.Errorf("set idempotency cache: %w", err)
	}
	return nil
}
