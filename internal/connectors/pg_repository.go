package connectors

import (
	"context"
	"fmt"
	"time"

	"github.com/brevio/brevio/internal/database"
)

// OAuthTokenRepository persists encrypted OAuth tokens in PostgreSQL.
type OAuthTokenRepository interface {
	// StoreToken persists an encrypted OAuth token set (access + optional refresh).
	StoreToken(ctx context.Context, params StoreTokenParams) error

	// GetToken retrieves a stored OAuth token by workspace, user, and connector.
	GetToken(ctx context.Context, workspaceID, userID, connectorKey string) (*StoredOAuthToken, error)

	// UpdateAfterRefresh updates access token ciphertext and metadata after a token refresh.
	UpdateAfterRefresh(ctx context.Context, params RefreshTokenParams) error
}

// StoreTokenParams holds parameters for storing an OAuth token.
type StoreTokenParams struct {
	WorkspaceID      string
	UserID           string
	ConnectorKey     string
	Provider         string
	Ciphertext       []byte
	Nonce            []byte
	KeyVersion       string
	RefreshCiphertext []byte
	RefreshNonce     []byte
	ExpiresAt        *time.Time
}

// RefreshTokenParams holds parameters for updating a token after refresh.
type RefreshTokenParams struct {
	WorkspaceID  string
	UserID       string
	ConnectorKey string
	Ciphertext   []byte
	Nonce        []byte
	KeyVersion   string
	ExpiresAt    time.Time
}

// StoredOAuthToken represents a token retrieved from the database.
type StoredOAuthToken struct {
	WorkspaceID       string
	UserID            string
	ConnectorKey      string
	Provider          string
	Ciphertext        []byte
	Nonce             []byte
	KeyVersion        string
	RefreshCiphertext []byte
	RefreshNonce      []byte
	ExpiresAt         *time.Time
	LastRefreshedAt   *time.Time
	UpdatedAt         time.Time
	CreatedAt         time.Time
}

// PgOAuthTokenRepository implements OAuthTokenRepository using pgx.
type PgOAuthTokenRepository struct {
	db database.Querier
}

var _ OAuthTokenRepository = (*PgOAuthTokenRepository)(nil)

// NewPgOAuthTokenRepository creates a new pgx-backed OAuth token repository.
func NewPgOAuthTokenRepository(db database.Querier) *PgOAuthTokenRepository {
	return &PgOAuthTokenRepository{db: db}
}

func (r *PgOAuthTokenRepository) StoreToken(ctx context.Context, params StoreTokenParams) error {
	now := time.Now().UTC()
	_, err := r.db.Exec(ctx, `
		INSERT INTO user_oauth_tokens (
			workspace_id, user_id, connector_id,
			ciphertext, nonce, key_version,
			refresh_ciphertext, refresh_nonce,
			provider, expires_at, encrypted_at, updated_at, created_at
		) VALUES (
			$1::uuid, $2::uuid, (SELECT id FROM connectors WHERE key = $3 LIMIT 1),
			$4, $5, $6,
			$7, $8,
			$9, $10, $11, $11, $11
		)
		ON CONFLICT (workspace_id, user_id, connector_id) DO UPDATE SET
			ciphertext = EXCLUDED.ciphertext,
			nonce = EXCLUDED.nonce,
			key_version = EXCLUDED.key_version,
			refresh_ciphertext = EXCLUDED.refresh_ciphertext,
			refresh_nonce = EXCLUDED.refresh_nonce,
			provider = EXCLUDED.provider,
			expires_at = EXCLUDED.expires_at,
			encrypted_at = EXCLUDED.encrypted_at,
			updated_at = EXCLUDED.updated_at`,
		params.WorkspaceID, params.UserID, params.ConnectorKey,
		params.Ciphertext, params.Nonce, params.KeyVersion,
		params.RefreshCiphertext, params.RefreshNonce,
		params.Provider, params.ExpiresAt, now,
	)
	if err != nil {
		return fmt.Errorf("store oauth token: %w", err)
	}
	return nil
}

func (r *PgOAuthTokenRepository) GetToken(ctx context.Context, workspaceID, userID, connectorKey string) (*StoredOAuthToken, error) {
	row := r.db.QueryRow(ctx, `
		SELECT
			t.workspace_id::text, t.user_id::text,
			c.key,
			t.ciphertext, t.nonce, t.key_version,
			t.refresh_ciphertext, t.refresh_nonce,
			t.provider, t.expires_at, t.last_refreshed_at,
			t.updated_at, t.created_at
		FROM user_oauth_tokens t
		JOIN connectors c ON c.id = t.connector_id
		WHERE t.workspace_id = $1::uuid
			AND t.user_id = $2::uuid
			AND c.key = $3`,
		workspaceID, userID, connectorKey,
	)

	var token StoredOAuthToken
	err := row.Scan(
		&token.WorkspaceID, &token.UserID,
		&token.ConnectorKey,
		&token.Ciphertext, &token.Nonce, &token.KeyVersion,
		&token.RefreshCiphertext, &token.RefreshNonce,
		&token.Provider, &token.ExpiresAt, &token.LastRefreshedAt,
		&token.UpdatedAt, &token.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("get oauth token: %w", err)
	}
	return &token, nil
}

func (r *PgOAuthTokenRepository) UpdateAfterRefresh(ctx context.Context, params RefreshTokenParams) error {
	now := time.Now().UTC()
	tag, err := r.db.Exec(ctx, `
		UPDATE user_oauth_tokens
		SET ciphertext = $1,
			nonce = $2,
			key_version = $3,
			expires_at = $4,
			last_refreshed_at = $5,
			updated_at = $5,
			encrypted_at = $5
		WHERE workspace_id = $6::uuid
			AND user_id = $7::uuid
			AND connector_id = (SELECT id FROM connectors WHERE key = $8 LIMIT 1)`,
		params.Ciphertext, params.Nonce, params.KeyVersion,
		params.ExpiresAt, now,
		params.WorkspaceID, params.UserID, params.ConnectorKey,
	)
	if err != nil {
		return fmt.Errorf("update oauth token after refresh: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("oauth token not found for refresh")
	}
	return nil
}
