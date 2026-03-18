package hipaa

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PHIPolicy enforces HIPAA access controls for PHI data.
type PHIPolicy struct {
	db     *pgxpool.Pool
	logger *slog.Logger
}

// NewPHIPolicy creates a new HIPAA policy enforcer.
func NewPHIPolicy(db *pgxpool.Pool, logger *slog.Logger) *PHIPolicy {
	return &PHIPolicy{db: db, logger: logger}
}

// EnforcePHIAccess validates all HIPAA requirements before allowing PHI access.
// If all checks pass, logs the access to hipaa_access_log and returns nil.
func (p *PHIPolicy) EnforcePHIAccess(ctx context.Context, req PHIAccessRequest) error {
	// 1. Check workspace type is HIPAA covered entity.
	if err := p.checkWorkspaceType(ctx, req.WorkspaceID); err != nil {
		return err
	}

	// 2. Check active BAA exists.
	if err := p.checkBAA(ctx, req.WorkspaceID); err != nil {
		return err
	}

	// 3. Check encryption settings.
	if err := p.checkEncryption(ctx, req.WorkspaceID); err != nil {
		return err
	}

	// 4. Check HIPAA consent.
	if err := p.checkHIPAAConsent(ctx, req.WorkspaceID, req.UserID); err != nil {
		return err
	}

	// All checks passed — log access.
	if err := p.logAccess(ctx, req); err != nil {
		p.logger.Error("hipaa_access_log_error", "error", err)
		// Don't fail the request for a logging error.
	}

	p.logger.Info("hipaa_phi_access_granted",
		"workspace_id", req.WorkspaceID,
		"user_id", req.UserID,
		"phi_category", req.PHICategory,
		"tool_key", req.ToolKey,
	)

	return nil
}

func (p *PHIPolicy) checkWorkspaceType(ctx context.Context, workspaceID uuid.UUID) error {
	if p.db == nil {
		return nil // skip in test mode
	}

	var wsType string
	err := p.db.QueryRow(ctx,
		`SELECT COALESCE(workspace_type, '') FROM workspaces WHERE id = $1`,
		workspaceID,
	).Scan(&wsType)
	if err != nil {
		return fmt.Errorf("check workspace type: %w", err)
	}

	if wsType != "hipaa_covered_entity" {
		return ErrWorkspaceNotHIPAACovered
	}
	return nil
}

func (p *PHIPolicy) checkBAA(ctx context.Context, workspaceID uuid.UUID) error {
	if p.db == nil {
		return ErrBAARequired
	}

	var count int
	err := p.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM business_associate_agreements
		 WHERE workspace_id = $1 AND revoked_at IS NULL
		   AND (expires_at IS NULL OR expires_at > NOW())`,
		workspaceID,
	).Scan(&count)
	if err != nil {
		return fmt.Errorf("check BAA: %w", err)
	}

	if count == 0 {
		return ErrBAARequired
	}
	return nil
}

func (p *PHIPolicy) checkEncryption(ctx context.Context, workspaceID uuid.UUID) error {
	if p.db == nil {
		return nil
	}

	var atRest, inTransit bool
	err := p.db.QueryRow(ctx,
		`SELECT COALESCE(encryption_at_rest, false), COALESCE(encryption_in_transit, false)
		 FROM workspace_settings WHERE workspace_id = $1`,
		workspaceID,
	).Scan(&atRest, &inTransit)
	if err != nil {
		// If no settings row, treat as not encrypted.
		return ErrEncryptionRequired
	}

	if !atRest || !inTransit {
		return ErrEncryptionRequired
	}
	return nil
}

func (p *PHIPolicy) checkHIPAAConsent(ctx context.Context, workspaceID, userID uuid.UUID) error {
	if p.db == nil {
		return ErrHIPAAAuthRequired
	}

	var count int
	err := p.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM consent_records
		 WHERE workspace_id = $1 AND user_id = $2
		   AND purpose = 'executive_assistance'
		   AND revoked_at IS NULL
		   AND (expires_at IS NULL OR expires_at > NOW())`,
		workspaceID, userID,
	).Scan(&count)
	if err != nil {
		return fmt.Errorf("check HIPAA consent: %w", err)
	}

	if count == 0 {
		return ErrHIPAAAuthRequired
	}
	return nil
}

func (p *PHIPolicy) logAccess(ctx context.Context, req PHIAccessRequest) error {
	if p.db == nil {
		return nil
	}

	_, err := p.db.Exec(ctx,
		`INSERT INTO hipaa_access_log (user_id, workspace_id, phi_category, data_accessed, purpose, accessed_at)
		 VALUES ($1, $2, $3, $4, $5, NOW())`,
		req.UserID, req.WorkspaceID, req.PHICategory, req.ToolKey, req.Purpose,
	)
	return err
}

// HasActiveBAA checks if a workspace has an active BAA.
func (p *PHIPolicy) HasActiveBAA(ctx context.Context, workspaceID uuid.UUID) (bool, error) {
	if p.db == nil {
		return false, nil
	}

	var count int
	err := p.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM business_associate_agreements
		 WHERE workspace_id = $1 AND revoked_at IS NULL
		   AND (expires_at IS NULL OR expires_at > NOW())`,
		workspaceID,
	).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}
