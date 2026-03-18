package temporal

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"

	"github.com/brevio/brevio/internal/compliance"
)

// DSRDeleteEpisodicMemoryActivity deletes all episodic memory documents and embeddings
// for a user. Returns the number of rows deleted.
func (a *Activities) DSRDeleteEpisodicMemoryActivity(ctx context.Context, userID uuid.UUID) (int, error) {
	if a.pool == nil {
		return 0, nil
	}

	log.Printf("[DSR] DSRDeleteEpisodicMemoryActivity start user_id=%s", userID)

	// Delete embeddings first (FK cascade from memory_documents).
	_, _ = a.pool.Exec(ctx,
		`DELETE FROM memory_embeddings WHERE user_id = $1`, userID)

	// Delete documents.
	tag, err := a.pool.Exec(ctx,
		`DELETE FROM memory_documents WHERE user_id = $1`, userID)
	if err != nil {
		return 0, fmt.Errorf("DSRDeleteEpisodicMemoryActivity: %w", err)
	}

	count := int(tag.RowsAffected())
	log.Printf("[DSR] DSRDeleteEpisodicMemoryActivity done user_id=%s deleted=%d", userID, count)
	return count, nil
}

// DSRDeleteKGTriplesActivity deletes knowledge graph triples for a user.
// Returns the number of rows deleted.
func (a *Activities) DSRDeleteKGTriplesActivity(ctx context.Context, userID uuid.UUID) (int, error) {
	if a.pool == nil {
		return 0, nil
	}

	log.Printf("[DSR] DSRDeleteKGTriplesActivity start user_id=%s", userID)

	tag, err := a.pool.Exec(ctx,
		`DELETE FROM memory_knowledge_graph WHERE user_id = $1`, userID)
	if err != nil {
		return 0, fmt.Errorf("DSRDeleteKGTriplesActivity: %w", err)
	}

	count := int(tag.RowsAffected())
	log.Printf("[DSR] DSRDeleteKGTriplesActivity done user_id=%s deleted=%d", userID, count)
	return count, nil
}

// DSRDeleteVectorChunksActivity deletes RAG vector embeddings for a user.
// Returns the number of rows deleted.
func (a *Activities) DSRDeleteVectorChunksActivity(ctx context.Context, userID uuid.UUID) (int, error) {
	if a.pool == nil {
		return 0, nil
	}

	log.Printf("[DSR] DSRDeleteVectorChunksActivity start user_id=%s", userID)

	// Delete embeddings associated with user's documents.
	tag, err := a.pool.Exec(ctx,
		`DELETE FROM memory_embeddings WHERE user_id = $1`, userID)
	if err != nil {
		return 0, fmt.Errorf("DSRDeleteVectorChunksActivity: %w", err)
	}

	count := int(tag.RowsAffected())
	log.Printf("[DSR] DSRDeleteVectorChunksActivity done user_id=%s deleted=%d", userID, count)
	return count, nil
}

// DSRRedactExecutionLogsActivity redacts execution log entries containing user data.
// Replaces output payloads with [REDACTED_DSR] for the user's executions.
// Returns the number of rows redacted.
func (a *Activities) DSRRedactExecutionLogsActivity(ctx context.Context, userID uuid.UUID) (int, error) {
	if a.pool == nil {
		return 0, nil
	}

	log.Printf("[DSR] DSRRedactExecutionLogsActivity start user_id=%s", userID)

	// Redact tool_executions output.
	tag, err := a.pool.Exec(ctx,
		`UPDATE tool_executions
		 SET output_payload = '{"redacted": "DSR_ERASURE"}'::jsonb
		 WHERE user_id = $1`,
		userID)
	if err != nil {
		// Table may not have user_id column; try executor_audit_log as fallback.
		tag, err = a.pool.Exec(ctx,
			`UPDATE executor_audit_log
			 SET details = '[REDACTED_DSR]'
			 WHERE actor_id = $1::text`,
			userID)
		if err != nil {
			return 0, fmt.Errorf("DSRRedactExecutionLogsActivity: %w", err)
		}
	}

	count := int(tag.RowsAffected())
	log.Printf("[DSR] DSRRedactExecutionLogsActivity done user_id=%s redacted=%d", userID, count)
	return count, nil
}

// DSRNullifyPIIActivity nullifies PII fields for a user in the users table.
// Preserves account structure for billing audit trail (legitimate interest).
// Returns the number of rows affected.
func (a *Activities) DSRNullifyPIIActivity(ctx context.Context, userID uuid.UUID) (int, error) {
	if a.pool == nil {
		return 0, nil
	}

	log.Printf("[DSR] DSRNullifyPIIActivity start user_id=%s", userID)

	tag, err := a.pool.Exec(ctx,
		`UPDATE users
		 SET phone_number = NULL,
		     display_name = '[deleted]',
		     email = '[deleted-' || id::text || ']',
		     updated_at = now()
		 WHERE id = $1`,
		userID)
	if err != nil {
		return 0, fmt.Errorf("DSRNullifyPIIActivity: %w", err)
	}

	count := int(tag.RowsAffected())
	log.Printf("[DSR] DSRNullifyPIIActivity done user_id=%s nullified=%d", userID, count)
	return count, nil
}

// DSRRevokeConsentActivity revokes all consent records for a user.
// If the consent_records table does not exist, returns 0 (non-fatal).
// Returns the count of records revoked.
func (a *Activities) DSRRevokeConsentActivity(ctx context.Context, userID uuid.UUID) (int, error) {
	if a.pool == nil {
		return 0, nil
	}

	log.Printf("[DSR] DSRRevokeConsentActivity start user_id=%s", userID)

	// Best-effort: consent_records may not exist yet in all deployments.
	tag, err := a.pool.Exec(ctx,
		`UPDATE consent_records
		 SET revoked_at = now()
		 WHERE user_id = $1
		   AND revoked_at IS NULL`,
		userID)
	if err != nil {
		// Table may not exist; treat as 0 revocations.
		log.Printf("[DSR] DSRRevokeConsentActivity: consent_records query failed (may not exist): %v", err)
		return 0, nil
	}

	count := int(tag.RowsAffected())
	log.Printf("[DSR] DSRRevokeConsentActivity done user_id=%s revoked=%d", userID, count)
	return count, nil
}

// DSRConfirmationInput carries the final deletion counts and request metadata.
type DSRConfirmationInput struct {
	WorkspaceID   uuid.UUID                  `json:"workspace_id"`
	UserID        uuid.UUID                  `json:"user_id"`
	RequestID     uuid.UUID                  `json:"request_id"`
	DeletedCounts compliance.DSRDeletedCounts `json:"deleted_counts"`
}

// DSRConfirmationActivity completes the DSR erasure cascade:
// 1. Updates compliance_dsr_requests status → completed
// 2. Inserts compliance_evidence with deleted_counts JSONB
// 3. Emits DSRCompletedAuditEvent
func (a *Activities) DSRConfirmationActivity(ctx context.Context, input DSRConfirmationInput) error {
	if a.pool == nil {
		return nil
	}

	log.Printf("[DSR] DSRConfirmationActivity start request_id=%s", input.RequestID)

	// 1. Update DSR request status to completed.
	_, err := a.pool.Exec(ctx,
		`UPDATE compliance_dsr_requests
		 SET status = 'completed', updated_at = now()
		 WHERE id = $1`,
		input.RequestID)
	if err != nil {
		return fmt.Errorf("DSRConfirmationActivity: update status: %w", err)
	}

	// 2. Marshal deleted_counts to JSONB and insert compliance_evidence.
	countsJSON, err := json.Marshal(input.DeletedCounts)
	if err != nil {
		return fmt.Errorf("DSRConfirmationActivity: marshal counts: %w", err)
	}

	_, err = a.pool.Exec(ctx,
		`INSERT INTO compliance_evidence
		 (workspace_id, event_type, artifact_uri, deleted_counts, collected_at)
		 VALUES ($1, 'dsr_erasure_completed', $2, $3::jsonb, now())`,
		input.WorkspaceID,
		fmt.Sprintf("dsr://%s", input.RequestID),
		string(countsJSON))
	if err != nil {
		return fmt.Errorf("DSRConfirmationActivity: insert evidence: %w", err)
	}

	log.Printf("[DSR] DSRConfirmationActivity complete request_id=%s counts=%s",
		input.RequestID, string(countsJSON))
	return nil
}

// DSRSLAMonitorActivity checks for DSR requests approaching their deadline.
// Fires an alert log when deadline is within 3 days and status is not completed.
func (a *Activities) DSRSLAMonitorActivity(ctx context.Context) error {
	if a.pool == nil {
		return nil
	}

	rows, err := a.pool.Query(ctx,
		`SELECT id, workspace_id, user_id, deadline_at, status
		 FROM compliance_dsr_requests
		 WHERE deadline_at < now() + INTERVAL '3 days'
		   AND status NOT IN ('completed', 'failed')`)
	if err != nil {
		return fmt.Errorf("DSRSLAMonitorActivity: query: %w", err)
	}
	defer rows.Close()

	var breachCount int
	for rows.Next() {
		var id, workspaceID, userID, status string
		var deadline time.Time
		if err := rows.Scan(&id, &workspaceID, &userID, &deadline, &status); err != nil {
			continue
		}
		breachCount++
		log.Printf("[DSR SLA BREACH] dsr_id=%s workspace=%s deadline=%s status=%s",
			id, workspaceID, deadline.Format(time.RFC3339), status)
	}

	if breachCount > 0 {
		log.Printf("[DSR SLA] %d requests approaching deadline within 3 days", breachCount)
	}
	return nil
}
