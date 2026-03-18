package eu_ai_act

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// RiskRegister is the Article 9 risk management system.
type RiskRegister struct {
	pool *pgxpool.Pool
}

// NewRiskRegister creates a new risk register. Returns error if pool is nil.
func NewRiskRegister(pool *pgxpool.Pool) (*RiskRegister, error) {
	if pool == nil {
		return nil, fmt.Errorf("eu_ai_act.NewRiskRegister: pool must not be nil")
	}
	return &RiskRegister{pool: pool}, nil
}

// RecordRisk inserts a new risk item into the register.
func (r *RiskRegister) RecordRisk(ctx context.Context, item RiskItem) (RiskItem, error) {
	if item.WorkspaceID == uuid.Nil {
		return RiskItem{}, fmt.Errorf("RiskRegister.RecordRisk: workspace_id required")
	}
	if item.ReviewDate.IsZero() {
		item.ReviewDate = time.Now().UTC().Add(90 * 24 * time.Hour)
	}
	if item.MitigationStatus == "" {
		item.MitigationStatus = MitigationOpen
	}
	id := uuid.New()

	_, err := r.pool.Exec(ctx,
		`INSERT INTO eu_ai_act_risks
		 (id, workspace_id, category, description, likelihood, impact,
		  mitigation_status, mitigation_notes, review_date, source_event, source_ref, created_at, updated_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,now(),now())`,
		id, item.WorkspaceID, string(item.Category), item.Description,
		string(item.Likelihood), string(item.Impact),
		string(item.MitigationStatus), item.MitigationNotes,
		item.ReviewDate, item.SourceEvent, item.SourceRef,
	)
	if err != nil {
		return RiskItem{}, fmt.Errorf("RiskRegister.RecordRisk: %w", err)
	}

	item.ID = id
	return item, nil
}

// ListRisks returns all risk items for a workspace, optionally filtered by category.
func (r *RiskRegister) ListRisks(ctx context.Context, workspaceID uuid.UUID, category *RiskCategory) ([]RiskItem, error) {
	query := `SELECT id, workspace_id, category, description, likelihood, impact,
	                 mitigation_status, mitigation_notes, review_date, source_event, source_ref,
	                 created_at, updated_at
	          FROM eu_ai_act_risks
	          WHERE workspace_id = $1`
	args := []interface{}{workspaceID}

	if category != nil {
		query += ` AND category = $2`
		args = append(args, string(*category))
	}
	query += ` ORDER BY created_at DESC`

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("RiskRegister.ListRisks: %w", err)
	}
	defer rows.Close()

	var items []RiskItem
	for rows.Next() {
		var item RiskItem
		var cat, lh, imp, ms string
		if err := rows.Scan(
			&item.ID, &item.WorkspaceID, &cat, &item.Description,
			&lh, &imp, &ms, &item.MitigationNotes,
			&item.ReviewDate, &item.SourceEvent, &item.SourceRef,
			&item.CreatedAt, &item.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("RiskRegister.ListRisks: scan: %w", err)
		}
		item.Category = RiskCategory(cat)
		item.Likelihood = RiskLikelihood(lh)
		item.Impact = RiskImpact(imp)
		item.MitigationStatus = MitigationStatus(ms)
		items = append(items, item)
	}
	if items == nil {
		items = []RiskItem{}
	}
	return items, nil
}

// UpdateMitigationStatus updates the mitigation status and notes for a risk item.
func (r *RiskRegister) UpdateMitigationStatus(ctx context.Context,
	riskID uuid.UUID, status MitigationStatus, notes string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE eu_ai_act_risks
		 SET mitigation_status = $1, mitigation_notes = $2, updated_at = now()
		 WHERE id = $3`,
		string(status), notes, riskID,
	)
	if err != nil {
		return fmt.Errorf("RiskRegister.UpdateMitigationStatus: %w", err)
	}
	return nil
}
