package eu_ai_act

import (
	"context"
	"fmt"
	"log"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// IncidentLog implements the Article 73 serious incident reporting requirement.
type IncidentLog struct {
	pool *pgxpool.Pool
}

// NewIncidentLog creates a new incident log. Returns error if pool is nil.
func NewIncidentLog(pool *pgxpool.Pool) (*IncidentLog, error) {
	if pool == nil {
		return nil, fmt.Errorf("eu_ai_act.NewIncidentLog: pool must not be nil")
	}
	return &IncidentLog{pool: pool}, nil
}

// RecordIncident inserts a new Art. 73 incident.
func (l *IncidentLog) RecordIncident(ctx context.Context, entry IncidentEntry) (IncidentEntry, error) {
	if entry.WorkspaceID == uuid.Nil {
		return IncidentEntry{}, fmt.Errorf("IncidentLog.RecordIncident: workspace_id required")
	}

	id := uuid.New()
	_, err := l.pool.Exec(ctx,
		`INSERT INTO eu_ai_act_incidents
		 (id, workspace_id, incident_type, trigger_metric, severity, description,
		  dsr_request_id, reported_at, created_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,now(),now())`,
		id, entry.WorkspaceID, entry.IncidentType, entry.TriggerMetric,
		entry.Severity, entry.Description, entry.DSRRequestID,
	)
	if err != nil {
		return IncidentEntry{}, fmt.Errorf("IncidentLog.RecordIncident: insert: %w", err)
	}
	entry.ID = id

	// Auto-create DSR incident record for high/critical severity.
	if entry.Severity == "high" || entry.Severity == "critical" {
		dsrID := uuid.New()
		_, dsrErr := l.pool.Exec(ctx,
			`INSERT INTO compliance_dsr_requests
			 (id, workspace_id, user_id, request_type, status, deadline_at, created_at, updated_at)
			 VALUES ($1, $2, $3, 'incident', 'pending', now() + INTERVAL '28 days', now(), now())
			 ON CONFLICT DO NOTHING`,
			dsrID, entry.WorkspaceID, uuid.Nil,
		)
		if dsrErr != nil {
			log.Printf("IncidentLog.RecordIncident: auto-DSR failed: %v", dsrErr)
		}
	}

	return entry, nil
}

// ListIncidents returns all incidents for a workspace.
func (l *IncidentLog) ListIncidents(ctx context.Context, workspaceID uuid.UUID) ([]IncidentEntry, error) {
	rows, err := l.pool.Query(ctx,
		`SELECT id, workspace_id, incident_type, trigger_metric, severity, description,
		        dsr_request_id, resolved_at, reported_at, created_at
		 FROM eu_ai_act_incidents
		 WHERE workspace_id = $1
		 ORDER BY reported_at DESC`,
		workspaceID,
	)
	if err != nil {
		return nil, fmt.Errorf("IncidentLog.ListIncidents: %w", err)
	}
	defer rows.Close()

	var entries []IncidentEntry
	for rows.Next() {
		var e IncidentEntry
		if err := rows.Scan(&e.ID, &e.WorkspaceID, &e.IncidentType, &e.TriggerMetric,
			&e.Severity, &e.Description, &e.DSRRequestID, &e.ResolvedAt,
			&e.ReportedAt, &e.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("IncidentLog.ListIncidents: scan: %w", err)
		}
		entries = append(entries, e)
	}
	if entries == nil {
		entries = []IncidentEntry{}
	}
	return entries, nil
}
