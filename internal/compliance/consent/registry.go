package consent

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ConsentRegistry manages GDPR-compliant consent records.
type ConsentRegistry struct {
	db     *pgxpool.Pool
	logger *slog.Logger
}

// NewConsentRegistry creates a registry backed by the given database pool.
func NewConsentRegistry(db *pgxpool.Pool, logger *slog.Logger) *ConsentRegistry {
	return &ConsentRegistry{db: db, logger: logger}
}

// GrantConsent creates a consent record. Idempotent: returns existing record if
// a non-revoked record for the same (workspace_id, user_id, purpose) exists.
func (r *ConsentRegistry) GrantConsent(ctx context.Context, req GrantConsentRequest) (*ConsentRecord, error) {
	if !ValidPurposes[req.Purpose] {
		return nil, fmt.Errorf("invalid consent purpose: %s", req.Purpose)
	}
	if !ValidBases[req.LawfulBasis] {
		return nil, fmt.Errorf("invalid lawful basis: %s", req.LawfulBasis)
	}

	// Check for existing active consent.
	existing, err := r.findActiveConsent(ctx, req.WorkspaceID, req.UserID, req.Purpose)
	if err != nil {
		return nil, fmt.Errorf("check existing consent: %w", err)
	}
	if existing != nil {
		return existing, nil
	}

	if r.db == nil {
		// In-memory fallback for tests.
		rec := &ConsentRecord{
			ID:          uuid.New(),
			WorkspaceID: req.WorkspaceID,
			UserID:      req.UserID,
			Purpose:     req.Purpose,
			LawfulBasis: req.LawfulBasis,
			ExpiresAt:   req.ExpiresAt,
		}
		return rec, nil
	}

	var rec ConsentRecord
	err = r.db.QueryRow(ctx,
		`INSERT INTO consent_records (workspace_id, user_id, purpose, lawful_basis, expires_at)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING id, workspace_id, user_id, purpose, lawful_basis, granted_at, expires_at, revoked_at, created_at`,
		req.WorkspaceID, req.UserID, string(req.Purpose), string(req.LawfulBasis), req.ExpiresAt,
	).Scan(&rec.ID, &rec.WorkspaceID, &rec.UserID, &rec.Purpose, &rec.LawfulBasis,
		&rec.GrantedAt, &rec.ExpiresAt, &rec.RevokedAt, &rec.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("insert consent: %w", err)
	}

	r.logger.Info("consent_granted",
		"workspace_id", req.WorkspaceID,
		"user_id", req.UserID,
		"purpose", req.Purpose,
	)

	return &rec, nil
}

// RevokeConsent marks the active consent record for the given purpose as revoked.
func (r *ConsentRegistry) RevokeConsent(ctx context.Context, workspaceID, userID uuid.UUID, purpose ConsentPurpose) error {
	if r.db == nil {
		return ErrConsentNotFound
	}

	tag, err := r.db.Exec(ctx,
		`UPDATE consent_records SET revoked_at = NOW()
		 WHERE workspace_id = $1 AND user_id = $2 AND purpose = $3
		   AND revoked_at IS NULL AND (expires_at IS NULL OR expires_at > NOW())`,
		workspaceID, userID, string(purpose),
	)
	if err != nil {
		return fmt.Errorf("revoke consent: %w", err)
	}

	if tag.RowsAffected() == 0 {
		return ErrConsentNotFound
	}

	r.logger.Info("consent_revoked",
		"workspace_id", workspaceID,
		"user_id", userID,
		"purpose", purpose,
	)

	return nil
}

// HasActiveConsent checks whether an active consent exists for the given purpose.
func (r *ConsentRegistry) HasActiveConsent(ctx context.Context, workspaceID, userID uuid.UUID, purpose ConsentPurpose) (bool, error) {
	if r.db == nil {
		return false, nil
	}

	var count int
	err := r.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM consent_records
		 WHERE workspace_id = $1 AND user_id = $2 AND purpose = $3
		   AND revoked_at IS NULL AND (expires_at IS NULL OR expires_at > NOW())`,
		workspaceID, userID, string(purpose),
	).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("check consent: %w", err)
	}

	return count > 0, nil
}

// GetActiveConsents returns all non-revoked, non-expired consent records for a user.
func (r *ConsentRegistry) GetActiveConsents(ctx context.Context, workspaceID, userID uuid.UUID) ([]ConsentRecord, error) {
	if r.db == nil {
		return nil, nil
	}

	rows, err := r.db.Query(ctx,
		`SELECT id, workspace_id, user_id, purpose, lawful_basis, granted_at, expires_at, revoked_at, created_at
		 FROM consent_records
		 WHERE workspace_id = $1 AND user_id = $2
		   AND revoked_at IS NULL AND (expires_at IS NULL OR expires_at > NOW())
		 ORDER BY granted_at`,
		workspaceID, userID,
	)
	if err != nil {
		return nil, fmt.Errorf("list consents: %w", err)
	}
	defer rows.Close()

	var records []ConsentRecord
	for rows.Next() {
		var rec ConsentRecord
		if err := rows.Scan(&rec.ID, &rec.WorkspaceID, &rec.UserID, &rec.Purpose,
			&rec.LawfulBasis, &rec.GrantedAt, &rec.ExpiresAt, &rec.RevokedAt, &rec.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan consent: %w", err)
		}
		records = append(records, rec)
	}

	return records, rows.Err()
}

// RecordDataAccess inserts an entry into the purpose_audit_log for the given consent.
func (r *ConsentRegistry) RecordDataAccess(ctx context.Context, consentID uuid.UUID, accessType string, dataCategory string) error {
	if r.db == nil {
		return nil
	}

	_, err := r.db.Exec(ctx,
		`INSERT INTO purpose_audit_log (consent_id, access_type, data_category)
		 VALUES ($1, $2, $3)`,
		consentID, accessType, dataCategory,
	)
	if err != nil {
		return fmt.Errorf("record data access: %w", err)
	}

	return nil
}

// FindActiveConsentID returns the ID of the active consent for a purpose, or uuid.Nil.
func (r *ConsentRegistry) FindActiveConsentID(ctx context.Context, workspaceID, userID uuid.UUID, purpose ConsentPurpose) (uuid.UUID, error) {
	rec, err := r.findActiveConsent(ctx, workspaceID, userID, purpose)
	if err != nil {
		return uuid.Nil, err
	}
	if rec == nil {
		return uuid.Nil, nil
	}
	return rec.ID, nil
}

func (r *ConsentRegistry) findActiveConsent(ctx context.Context, workspaceID, userID uuid.UUID, purpose ConsentPurpose) (*ConsentRecord, error) {
	if r.db == nil {
		return nil, nil
	}

	var rec ConsentRecord
	err := r.db.QueryRow(ctx,
		`SELECT id, workspace_id, user_id, purpose, lawful_basis, granted_at, expires_at, revoked_at, created_at
		 FROM consent_records
		 WHERE workspace_id = $1 AND user_id = $2 AND purpose = $3
		   AND revoked_at IS NULL AND (expires_at IS NULL OR expires_at > NOW())
		 LIMIT 1`,
		workspaceID, userID, string(purpose),
	).Scan(&rec.ID, &rec.WorkspaceID, &rec.UserID, &rec.Purpose,
		&rec.LawfulBasis, &rec.GrantedAt, &rec.ExpiresAt, &rec.RevokedAt, &rec.CreatedAt)
	if err != nil {
		if err.Error() == "no rows in result set" {
			return nil, nil
		}
		return nil, err
	}

	return &rec, nil
}
