package admin

import (
	"context"
	"fmt"
	"time"

	"github.com/brevio/brevio/internal/database"
)

// RevenueReadRepository provides read-only access to subscription/MRR data.
type RevenueReadRepository interface {
	GetMRRSnapshot(ctx context.Context, workspaceID string, date time.Time) (RevOpsMRRSnapshot, error)
	GetLatestMRRSnapshot(ctx context.Context, workspaceID string) (RevOpsMRRSnapshot, error)
	GetMarginReport(ctx context.Context, workspaceID string, date time.Time) (RevOpsOperatorMarginReport, error)
}

// SubscriptionWriter writes subscription events (NNR-105: only via Temporal).
type SubscriptionWriter interface {
	InsertSubscriptionEvent(ctx context.Context, evt SubscriptionEventPayload) error
	UpsertMRRSnapshot(ctx context.Context, workspaceID string, date time.Time, mrr, arr, newMRR, churnedMRR, expansionMRR float64, activeSubs int) error
}

// PgRevenueRepository implements both RevenueReadRepository and SubscriptionWriter.
type PgRevenueRepository struct {
	q database.Querier
}

// NewPgRevenueRepository creates a new PgRevenueRepository.
func NewPgRevenueRepository(q database.Querier) *PgRevenueRepository {
	return &PgRevenueRepository{q: q}
}

// InsertSubscriptionEvent writes a Stripe event idempotently (keyed by stripe_event_id).
func (r *PgRevenueRepository) InsertSubscriptionEvent(ctx context.Context, evt SubscriptionEventPayload) error {
	_, err := r.q.Exec(ctx,
		`INSERT INTO subscription_events (workspace_id, stripe_event_id, event_type, payload)
		 VALUES ($1::uuid, $2, $3, $4::jsonb)
		 ON CONFLICT (stripe_event_id) DO NOTHING`,
		evt.WorkspaceID, evt.StripeEventID, evt.EventType,
		fmt.Sprintf(`{"amount":%f,"currency":"%s"}`, evt.Amount, evt.Currency),
	)
	return err
}

// UpsertMRRSnapshot upserts a daily MRR snapshot.
func (r *PgRevenueRepository) UpsertMRRSnapshot(ctx context.Context, workspaceID string, date time.Time, mrr, arr, newMRR, churnedMRR, expansionMRR float64, activeSubs int) error {
	_, err := r.q.Exec(ctx,
		`INSERT INTO mrr_snapshots (workspace_id, snapshot_date, mrr_usd, arr_usd, new_mrr_usd, churned_mrr_usd, expansion_mrr_usd, active_subscriptions)
		 VALUES ($1::uuid, $2, $3, $4, $5, $6, $7, $8)
		 ON CONFLICT (workspace_id, snapshot_date) DO UPDATE SET
		   mrr_usd = EXCLUDED.mrr_usd,
		   arr_usd = EXCLUDED.arr_usd,
		   new_mrr_usd = EXCLUDED.new_mrr_usd,
		   churned_mrr_usd = EXCLUDED.churned_mrr_usd,
		   expansion_mrr_usd = EXCLUDED.expansion_mrr_usd,
		   active_subscriptions = EXCLUDED.active_subscriptions`,
		workspaceID, date, mrr, arr, newMRR, churnedMRR, expansionMRR, activeSubs,
	)
	return err
}

// GetMRRSnapshot reads a specific date's MRR snapshot.
func (r *PgRevenueRepository) GetMRRSnapshot(ctx context.Context, workspaceID string, date time.Time) (RevOpsMRRSnapshot, error) {
	var s RevOpsMRRSnapshot
	err := r.q.QueryRow(ctx,
		`SELECT id, workspace_id, snapshot_date, mrr_usd, arr_usd, active_subscriptions, created_at
		 FROM mrr_snapshots
		 WHERE workspace_id = $1::uuid AND snapshot_date = $2`,
		workspaceID, date,
	).Scan(&s.ID, &s.WorkspaceID, &s.Date, &s.MRR, &s.ARR, &s.UserCount, &s.CreatedAt)
	if err != nil {
		return s, fmt.Errorf("get mrr snapshot: %w", err)
	}
	return s, nil
}

// GetLatestMRRSnapshot reads the most recent MRR snapshot.
func (r *PgRevenueRepository) GetLatestMRRSnapshot(ctx context.Context, workspaceID string) (RevOpsMRRSnapshot, error) {
	var s RevOpsMRRSnapshot
	err := r.q.QueryRow(ctx,
		`SELECT id, workspace_id, snapshot_date, mrr_usd, arr_usd, active_subscriptions, created_at
		 FROM mrr_snapshots
		 WHERE workspace_id = $1::uuid
		 ORDER BY snapshot_date DESC
		 LIMIT 1`,
		workspaceID,
	).Scan(&s.ID, &s.WorkspaceID, &s.Date, &s.MRR, &s.ARR, &s.UserCount, &s.CreatedAt)
	if err != nil {
		return s, fmt.Errorf("get latest mrr snapshot: %w", err)
	}
	return s, nil
}

// GetMarginReport reads the operator margin report for a specific date.
func (r *PgRevenueRepository) GetMarginReport(ctx context.Context, workspaceID string, date time.Time) (RevOpsOperatorMarginReport, error) {
	var m RevOpsOperatorMarginReport
	err := r.q.QueryRow(ctx,
		`SELECT workspace_id, report_date, revenue_usd, cogs_usd, gross_margin_usd, gross_margin_pct
		 FROM operator_margin_report
		 WHERE workspace_id = $1::uuid AND report_date = $2`,
		workspaceID, date,
	).Scan(&m.WorkspaceID, &m.Date, &m.Revenue, &m.COGS, &m.Margin, &m.MarginPercent)
	if err != nil {
		return m, fmt.Errorf("get margin report: %w", err)
	}
	return m, nil
}
