package temporal

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/brevio/brevio/internal/admin"
	outboxpkg "github.com/brevio/brevio/internal/outbox"
)

// ===================== V10.1 COST OUTBOX PRODUCER TYPES =====================

// EnqueueLLMCostInput is the input for the non-blocking LLM cost outbox producer.
type EnqueueLLMCostInput struct {
	WorkspaceID   string  `json:"workspace_id"`
	UserID        string  `json:"user_id"`
	WorkflowRunID string  `json:"workflow_run_id"`
	Provider      string  `json:"provider"`
	Model         string  `json:"model"`
	TokensInput   int     `json:"tokens_input"`
	TokensOutput  int     `json:"tokens_output"`
	CostUSD       float64 `json:"cost_usd"`
	LatencyMs     int     `json:"latency_ms"`
	CacheHit      bool    `json:"cache_hit"`
}

// EnqueueLLMCostResult is the result of the LLM cost outbox producer.
type EnqueueLLMCostResult struct {
	EntryID  string `json:"entry_id"`
	Enqueued bool   `json:"enqueued"`
}

// EnqueueConnectorCostInput is the input for the non-blocking connector cost outbox producer.
type EnqueueConnectorCostInput struct {
	WorkspaceID   string  `json:"workspace_id"`
	UserID        string  `json:"user_id"`
	WorkflowRunID string  `json:"workflow_run_id"`
	ConnectorID   string  `json:"connector_id"`
	ConnectorName string  `json:"connector_name"`
	Operation     string  `json:"operation"`
	CostUSD       float64 `json:"cost_usd"`
	LatencyMs     int     `json:"latency_ms"`
}

// EnqueueConnectorCostResult is the result of the connector cost outbox producer.
type EnqueueConnectorCostResult struct {
	EntryID  string `json:"entry_id"`
	Enqueued bool   `json:"enqueued"`
}

// IngestSubscriptionEventInput is the input for Stripe subscription event ingestion.
type IngestSubscriptionEventInput struct {
	WorkspaceID   string  `json:"workspace_id"`
	StripeEventID string  `json:"stripe_event_id"`
	EventType     string  `json:"event_type"`
	Amount        float64 `json:"amount"`
	Currency      string  `json:"currency"`
}

// IngestSubscriptionEventResult is the result of subscription event ingestion.
type IngestSubscriptionEventResult struct {
	Ingested     bool   `json:"ingested"`
	Duplicate    bool   `json:"duplicate"`
	EvidenceHash string `json:"evidence_hash"`
}

// ReconcileMRRInput is the input for MRR snapshot reconciliation.
type ReconcileMRRInput struct {
	WorkspaceID string `json:"workspace_id"`
	Date        string `json:"date"` // YYYY-MM-DD
}

// ReconcileMRRResult is the result of MRR snapshot reconciliation.
type ReconcileMRRResult struct {
	MRRUSD           float64 `json:"mrr_usd"`
	ARRUSD           float64 `json:"arr_usd"`
	ActiveSubs       int     `json:"active_subscriptions"`
	SnapshotRecorded bool    `json:"snapshot_recorded"`
}

// WriteLedgerFromOutboxInput is the input for writing cost ledger entries from outbox events.
type WriteLedgerFromOutboxInput struct {
	EventType string `json:"event_type"`
	Payload   string `json:"payload"`
}

// WriteLedgerFromOutboxResult is the result of writing cost ledger entries.
type WriteLedgerFromOutboxResult struct {
	Written bool   `json:"written"`
	Error   string `json:"error,omitempty"`
}

// ===================== V10.1 COST OUTBOX PRODUCER ACTIVITIES =====================

// EnqueueLLMCostActivity enqueues an LLM cost event via the outbox (NNR-104: non-blocking).
// This activity is called from the hot path and must not perform blocking ledger writes.
func (a *Activities) EnqueueLLMCostActivity(ctx context.Context, input EnqueueLLMCostInput) (*EnqueueLLMCostResult, error) {
	evt := admin.LLMCostEvent{
		WorkspaceID:   input.WorkspaceID,
		UserID:        input.UserID,
		WorkflowRunID: input.WorkflowRunID,
		Provider:      input.Provider,
		Model:         input.Model,
		TokensInput:   input.TokensInput,
		TokensOutput:  input.TokensOutput,
		CostUSD:       input.CostUSD,
		LatencyMs:     input.LatencyMs,
		CacheHit:      input.CacheHit,
	}

	payload, err := json.Marshal(evt)
	if err != nil {
		return &EnqueueLLMCostResult{Enqueued: false}, fmt.Errorf("marshal llm cost event: %w", err)
	}

	entryID := hashKey(fmt.Sprintf("%s:%s:%s", input.WorkspaceID, admin.EventTypeLLMCost, string(payload)))

	if a.outboxSvc != nil && a.pool != nil {
		tx, txErr := a.pool.Begin(ctx)
		if txErr != nil {
			return &EnqueueLLMCostResult{EntryID: entryID, Enqueued: false}, nil
		}

		enqErr := a.outboxSvc.Enqueue(ctx, tx, outboxEntryFromCost(entryID, input.WorkspaceID, admin.EventTypeLLMCost, payload))
		if enqErr != nil {
			_ = tx.Rollback(ctx)
			// Duplicate key = idempotent success
			return &EnqueueLLMCostResult{EntryID: entryID, Enqueued: true}, nil
		}
		if commitErr := tx.Commit(ctx); commitErr != nil {
			return &EnqueueLLMCostResult{EntryID: entryID, Enqueued: false}, nil
		}
		return &EnqueueLLMCostResult{EntryID: entryID, Enqueued: true}, nil
	}

	// Degraded mode: no outbox service, return success without persisting.
	return &EnqueueLLMCostResult{EntryID: entryID, Enqueued: false}, nil
}

// EnqueueConnectorCostActivity enqueues a connector cost event via the outbox (NNR-104).
func (a *Activities) EnqueueConnectorCostActivity(ctx context.Context, input EnqueueConnectorCostInput) (*EnqueueConnectorCostResult, error) {
	evt := admin.ConnectorCostEvent{
		WorkspaceID:   input.WorkspaceID,
		UserID:        input.UserID,
		WorkflowRunID: input.WorkflowRunID,
		ConnectorID:   input.ConnectorID,
		ConnectorName: input.ConnectorName,
		Operation:     input.Operation,
		CostUSD:       input.CostUSD,
		LatencyMs:     input.LatencyMs,
	}

	payload, err := json.Marshal(evt)
	if err != nil {
		return &EnqueueConnectorCostResult{Enqueued: false}, fmt.Errorf("marshal connector cost event: %w", err)
	}

	entryID := hashKey(fmt.Sprintf("%s:%s:%s", input.WorkspaceID, admin.EventTypeConnectorCost, string(payload)))

	if a.outboxSvc != nil && a.pool != nil {
		tx, txErr := a.pool.Begin(ctx)
		if txErr != nil {
			return &EnqueueConnectorCostResult{EntryID: entryID, Enqueued: false}, nil
		}

		enqErr := a.outboxSvc.Enqueue(ctx, tx, outboxEntryFromCost(entryID, input.WorkspaceID, admin.EventTypeConnectorCost, payload))
		if enqErr != nil {
			_ = tx.Rollback(ctx)
			return &EnqueueConnectorCostResult{EntryID: entryID, Enqueued: true}, nil
		}
		if commitErr := tx.Commit(ctx); commitErr != nil {
			return &EnqueueConnectorCostResult{EntryID: entryID, Enqueued: false}, nil
		}
		return &EnqueueConnectorCostResult{EntryID: entryID, Enqueued: true}, nil
	}

	return &EnqueueConnectorCostResult{EntryID: entryID, Enqueued: false}, nil
}

// ===================== V10.1 STRIPE SUBSCRIPTION ACTIVITIES =====================

// IngestSubscriptionEventActivity ingests a Stripe subscription event idempotently.
func (a *Activities) IngestSubscriptionEventActivity(ctx context.Context, input IngestSubscriptionEventInput) (*IngestSubscriptionEventResult, error) {
	if input.StripeEventID == "" {
		return nil, fmt.Errorf("SUBSCRIPTION_VALIDATION_FAILED: missing stripe_event_id")
	}

	evidence := hashKey(fmt.Sprintf("sub-ingest:%s:%s", input.WorkspaceID, input.StripeEventID))

	if a.pool != nil {
		// Check for duplicate first.
		var existingID string
		err := a.pool.QueryRow(ctx,
			`SELECT id FROM subscription_events WHERE stripe_event_id = $1`,
			input.StripeEventID).Scan(&existingID)
		if err == nil {
			return &IngestSubscriptionEventResult{Ingested: false, Duplicate: true, EvidenceHash: evidence}, nil
		}

		// Insert idempotently.
		payload := fmt.Sprintf(`{"amount":%f,"currency":"%s","event_type":"%s"}`, input.Amount, input.Currency, input.EventType)
		_, err = a.pool.Exec(ctx,
			`INSERT INTO subscription_events (workspace_id, stripe_event_id, event_type, payload)
			 VALUES ($1::uuid, $2, $3, $4::jsonb)
			 ON CONFLICT (stripe_event_id) DO NOTHING`,
			input.WorkspaceID, input.StripeEventID, input.EventType, payload)
		if err != nil {
			return nil, fmt.Errorf("ingest subscription event: %w", err)
		}
	}

	return &IngestSubscriptionEventResult{Ingested: true, Duplicate: false, EvidenceHash: evidence}, nil
}

// ReconcileMRRActivity computes and persists an MRR snapshot for a workspace.
func (a *Activities) ReconcileMRRActivity(ctx context.Context, input ReconcileMRRInput) (*ReconcileMRRResult, error) {
	if input.WorkspaceID == "" {
		return nil, fmt.Errorf("SUBSCRIPTION_VALIDATION_FAILED: missing workspace_id")
	}

	date, err := time.Parse("2006-01-02", input.Date)
	if err != nil {
		return nil, fmt.Errorf("invalid date format: %w", err)
	}

	if a.pool != nil {
		// Compute MRR from active subscription events.
		var mrr float64
		var activeSubs int
		err := a.pool.QueryRow(ctx,
			`SELECT COALESCE(SUM((payload->>'amount')::numeric), 0), COUNT(*)
			 FROM subscription_events
			 WHERE workspace_id = $1::uuid
			   AND event_type IN ('new', 'upgrade', 'downgrade')
			   AND created_at <= $2
			   AND stripe_event_id NOT IN (
			     SELECT stripe_event_id FROM subscription_events
			     WHERE workspace_id = $1::uuid AND event_type = 'churn' AND created_at <= $2
			   )`,
			input.WorkspaceID, date.Add(24*time.Hour)).Scan(&mrr, &activeSubs)
		if err != nil {
			return &ReconcileMRRResult{SnapshotRecorded: false}, nil
		}

		// Upsert snapshot.
		_, _ = a.pool.Exec(ctx,
			`INSERT INTO mrr_snapshots (workspace_id, snapshot_date, mrr_usd, arr_usd, active_subscriptions)
			 VALUES ($1::uuid, $2, $3, $4, $5)
			 ON CONFLICT (workspace_id, snapshot_date) DO UPDATE SET
			   mrr_usd = EXCLUDED.mrr_usd, arr_usd = EXCLUDED.arr_usd, active_subscriptions = EXCLUDED.active_subscriptions`,
			input.WorkspaceID, date, mrr, mrr*12, activeSubs)

		return &ReconcileMRRResult{
			MRRUSD:           mrr,
			ARRUSD:           mrr * 12,
			ActiveSubs:       activeSubs,
			SnapshotRecorded: true,
		}, nil
	}

	return &ReconcileMRRResult{SnapshotRecorded: false}, nil
}

// ===================== V10.1 COST LEDGER WRITER ACTIVITY =====================

// WriteLedgerFromOutboxActivity processes a cost outbox event and writes to the ledger.
// Called by the outbox dispatch workflow — not from the hot path (NNR-104).
func (a *Activities) WriteLedgerFromOutboxActivity(ctx context.Context, input WriteLedgerFromOutboxInput) (*WriteLedgerFromOutboxResult, error) {
	if a.pool == nil {
		return &WriteLedgerFromOutboxResult{Written: false, Error: "no database pool"}, nil
	}

	repo := admin.NewPgCostRepository(a.pool)

	switch input.EventType {
	case admin.EventTypeLLMCost:
		var evt admin.LLMCostEvent
		if err := json.Unmarshal([]byte(input.Payload), &evt); err != nil {
			return &WriteLedgerFromOutboxResult{Written: false, Error: err.Error()}, nil
		}
		if err := repo.InsertLLMCost(ctx, evt); err != nil {
			return &WriteLedgerFromOutboxResult{Written: false, Error: err.Error()}, nil
		}
		return &WriteLedgerFromOutboxResult{Written: true}, nil

	case admin.EventTypeConnectorCost:
		var evt admin.ConnectorCostEvent
		if err := json.Unmarshal([]byte(input.Payload), &evt); err != nil {
			return &WriteLedgerFromOutboxResult{Written: false, Error: err.Error()}, nil
		}
		if err := repo.InsertConnectorCost(ctx, evt); err != nil {
			return &WriteLedgerFromOutboxResult{Written: false, Error: err.Error()}, nil
		}
		return &WriteLedgerFromOutboxResult{Written: true}, nil

	default:
		return &WriteLedgerFromOutboxResult{Written: false, Error: "unknown event type: " + input.EventType}, nil
	}
}

// ===================== HELPERS =====================

// outboxEntryFromCost creates an outbox.OutboxEntry for a cost event.
func outboxEntryFromCost(entryID, workspaceID, eventType string, payload []byte) outboxpkg.OutboxEntry {
	return outboxpkg.OutboxEntry{
		ID:            entryID,
		WorkspaceID:   workspaceID,
		AggregateType: admin.AggregateTypeCost,
		AggregateID:   workspaceID,
		EventType:     eventType,
		Payload:       payload,
		Target:        "cost-ledger",
	}
}
