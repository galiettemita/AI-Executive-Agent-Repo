package call

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/brevio/brevio/internal/database"
)

// CallRepository is the interface for DB-backed call lifecycle operations.
type CallRepository interface {
	// Approval lifecycle (T10.1)
	CreateApprovalRequest(ctx context.Context, row ApprovalRequestRow) error
	GetApprovalRequest(ctx context.Context, id string) (*ApprovalRequestRow, error)
	ApproveRequest(ctx context.Context, id, decidedBy, reason string) error
	DenyRequest(ctx context.Context, id, decidedBy, reason string) error
	ExpirePendingRequests(ctx context.Context, workspaceID string) (int, error)
	GetPendingApprovals(ctx context.Context, workspaceID string, limit int) ([]ApprovalRequestRow, error)

	// Call lifecycle (T10.4)
	InsertCall(ctx context.Context, row CallRow) error
	UpdateCallStatus(ctx context.Context, id, status string) error
	UpdateCallProvider(ctx context.Context, id, providerCallID string, failoverCount int) error
	CompleteCall(ctx context.Context, id string, durationSeconds int, costUSD float64) error
	GetCall(ctx context.Context, id string) (*CallRow, error)
	GetCallByProviderID(ctx context.Context, providerCallID string) (*CallRow, error)
	ListCalls(ctx context.Context, workspaceID string, limit int) ([]CallRow, error)

	// Transcript segments (T10.5)
	InsertTranscriptSegment(ctx context.Context, row TranscriptSegmentRow) error
	GetTranscriptSegments(ctx context.Context, callID string) ([]TranscriptSegmentRow, error)

	// Call events
	InsertCallEvent(ctx context.Context, row CallEventRow) error
	GetCallEvents(ctx context.Context, callID string, limit int) ([]CallEventRow, error)

	// Provider health (T10.2)
	RecordProviderHealth(ctx context.Context, row ProviderHealthRow) error
	GetProviderHealth(ctx context.Context, providerID string, limit int) ([]ProviderHealthRow, error)
	GetProvider(ctx context.Context, workspaceID, providerName string) (*ProviderRow, error)
	UpdateProviderStatus(ctx context.Context, id, status string, healthScore float64) error

	// Rate limits
	IncrementRateLimit(ctx context.Context, workspaceID string, windowStart, windowEnd time.Time, maxCalls int) (int, error)

	// Blocklist
	IsNumberBlocked(ctx context.Context, workspaceID, numberHash string) (bool, error)

	// Approval policies
	GetActivePolicy(ctx context.Context, workspaceID string) (*ApprovalPolicyRow, error)
}

// Row types for DB persistence.

type ApprovalRequestRow struct {
	ID               string    `json:"id"`
	WorkspaceID      string    `json:"workspace_id"`
	ReceiptID        string    `json:"receipt_id"`
	PolicyID         string    `json:"policy_id"`
	CallerContext    string    `json:"caller_context"`
	TargetNumberHash string    `json:"target_number_hash"`
	TargetRegion     string    `json:"target_region"`
	Purpose          string    `json:"purpose"`
	Status           string    `json:"status"`
	DecidedBy        string    `json:"decided_by"`
	DecidedAt        *time.Time `json:"decided_at"`
	DecisionReason   string    `json:"decision_reason"`
	ExpiresAt        time.Time `json:"expires_at"`
	CreatedAt        time.Time `json:"created_at"`
}

type CallRow struct {
	ID                string     `json:"id"`
	WorkspaceID       string     `json:"workspace_id"`
	ApprovalRequestID string     `json:"approval_request_id"`
	ProviderID        string     `json:"provider_id"`
	Direction         string     `json:"direction"`
	Status            string     `json:"status"`
	TargetNumberHash  string     `json:"target_number_hash"`
	StartedAt         *time.Time `json:"started_at"`
	AnsweredAt        *time.Time `json:"answered_at"`
	EndedAt           *time.Time `json:"ended_at"`
	DurationSeconds   int        `json:"duration_seconds"`
	ProviderCallID    string     `json:"provider_call_id"`
	FailoverCount     int        `json:"failover_count"`
	CostUSD           float64    `json:"cost_usd"`
	Metadata          string     `json:"metadata"`
	CreatedAt         time.Time  `json:"created_at"`
}

type TranscriptSegmentRow struct {
	ID           string  `json:"id"`
	WorkspaceID  string  `json:"workspace_id"`
	CallID       string  `json:"call_id"`
	SegmentIndex int     `json:"segment_index"`
	SegmentType  string  `json:"segment_type"`
	Speaker      string  `json:"speaker"`
	Content      string  `json:"content"`
	StartedAtMs  int     `json:"started_at_ms"`
	DurationMs   int     `json:"duration_ms"`
	Confidence   float64 `json:"confidence"`
	Language     string  `json:"language"`
}

type CallEventRow struct {
	ID          string `json:"id"`
	WorkspaceID string `json:"workspace_id"`
	CallID      string `json:"call_id"`
	EventType   string `json:"event_type"`
	EventData   string `json:"event_data"`
}

type ProviderHealthRow struct {
	ID           string  `json:"id"`
	WorkspaceID  string  `json:"workspace_id"`
	ProviderID   string  `json:"provider_id"`
	HealthScore  float64 `json:"health_score"`
	LatencyMs    int     `json:"latency_ms"`
	ErrorCount   int     `json:"error_count"`
	SuccessCount int     `json:"success_count"`
	CheckType    string  `json:"check_type"`
	Details      string  `json:"details"`
}

type ProviderRow struct {
	ID                     string  `json:"id"`
	WorkspaceID            string  `json:"workspace_id"`
	ProviderName           string  `json:"provider_name"`
	ProviderType           string  `json:"provider_type"`
	Status                 string  `json:"status"`
	Priority               int     `json:"priority"`
	HealthScore            float64 `json:"health_score"`
	MaxConcurrentCalls     int     `json:"max_concurrent_calls"`
	CurrentConcurrentCalls int     `json:"current_concurrent_calls"`
}

type ApprovalPolicyRow struct {
	ID                       string   `json:"id"`
	WorkspaceID              string   `json:"workspace_id"`
	Name                     string   `json:"name"`
	AutoApproveConditions    string   `json:"auto_approve_conditions"`
	RequireApprovalConditions string  `json:"require_approval_conditions"`
	DenyConditions           string   `json:"deny_conditions"`
	MaxDailyCalls            int      `json:"max_daily_calls"`
	MaxCallDurationSeconds   int      `json:"max_call_duration_seconds"`
	AllowedHoursStart        *string  `json:"allowed_hours_start"`
	AllowedHoursEnd          *string  `json:"allowed_hours_end"`
	AllowedRegions           []string `json:"allowed_regions"`
	BlockedNumbers           []string `json:"blocked_numbers"`
}

// PgCallRepository implements CallRepository backed by pgx.
type PgCallRepository struct {
	q database.Querier
}

// NewPgCallRepository creates a new PgCallRepository.
func NewPgCallRepository(q database.Querier) *PgCallRepository {
	return &PgCallRepository{q: q}
}

// HashPhoneNumber returns a SHA-256 hash of a phone number for privacy-preserving storage.
func HashPhoneNumber(phoneNumber string) string {
	h := sha256.Sum256([]byte(phoneNumber))
	return hex.EncodeToString(h[:])
}

// --- Approval lifecycle ---

func (r *PgCallRepository) CreateApprovalRequest(ctx context.Context, row ApprovalRequestRow) error {
	_, err := r.q.Exec(ctx,
		`INSERT INTO call_approval_requests (workspace_id, receipt_id, policy_id, caller_context, target_number_hash, target_region, purpose, status, expires_at)
		 VALUES ($1::uuid, $2::uuid, $3::uuid, $4::jsonb, $5, $6, $7, $8::call_approval_status, $9)`,
		row.WorkspaceID, nilIfEmpty(row.ReceiptID), row.PolicyID, row.CallerContext,
		row.TargetNumberHash, row.TargetRegion, row.Purpose, row.Status, row.ExpiresAt,
	)
	if err != nil {
		return fmt.Errorf("create approval request: %w", err)
	}
	return nil
}

func (r *PgCallRepository) GetApprovalRequest(ctx context.Context, id string) (*ApprovalRequestRow, error) {
	var row ApprovalRequestRow
	var decidedBy, decisionReason *string
	err := r.q.QueryRow(ctx,
		`SELECT id, workspace_id, COALESCE(receipt_id::text, ''), policy_id, caller_context,
		        target_number_hash, target_region, purpose, status, decided_by, decided_at,
		        decision_reason, expires_at, created_at
		 FROM call_approval_requests WHERE id = $1::uuid`, id,
	).Scan(&row.ID, &row.WorkspaceID, &row.ReceiptID, &row.PolicyID, &row.CallerContext,
		&row.TargetNumberHash, &row.TargetRegion, &row.Purpose, &row.Status,
		&decidedBy, &row.DecidedAt, &decisionReason, &row.ExpiresAt, &row.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("get approval request: %w", err)
	}
	if decidedBy != nil {
		row.DecidedBy = *decidedBy
	}
	if decisionReason != nil {
		row.DecisionReason = *decisionReason
	}
	return &row, nil
}

func (r *PgCallRepository) ApproveRequest(ctx context.Context, id, decidedBy, reason string) error {
	result, err := r.q.Exec(ctx,
		`UPDATE call_approval_requests
		 SET status = 'approved'::call_approval_status, decided_by = $2::uuid, decided_at = now(), decision_reason = $3, updated_at = now()
		 WHERE id = $1::uuid AND status = 'pending'::call_approval_status`,
		id, nilIfEmpty(decidedBy), reason,
	)
	if err != nil {
		return fmt.Errorf("approve request: %w", err)
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("approval request %s not found or not pending", id)
	}
	return nil
}

func (r *PgCallRepository) DenyRequest(ctx context.Context, id, decidedBy, reason string) error {
	result, err := r.q.Exec(ctx,
		`UPDATE call_approval_requests
		 SET status = 'denied'::call_approval_status, decided_by = $2::uuid, decided_at = now(), decision_reason = $3, updated_at = now()
		 WHERE id = $1::uuid AND status = 'pending'::call_approval_status`,
		id, nilIfEmpty(decidedBy), reason,
	)
	if err != nil {
		return fmt.Errorf("deny request: %w", err)
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("approval request %s not found or not pending", id)
	}
	return nil
}

func (r *PgCallRepository) ExpirePendingRequests(ctx context.Context, workspaceID string) (int, error) {
	result, err := r.q.Exec(ctx,
		`UPDATE call_approval_requests
		 SET status = 'expired'::call_approval_status, updated_at = now()
		 WHERE workspace_id = $1::uuid AND status = 'pending'::call_approval_status AND expires_at < now()`,
		workspaceID,
	)
	if err != nil {
		return 0, fmt.Errorf("expire pending requests: %w", err)
	}
	return int(result.RowsAffected()), nil
}

func (r *PgCallRepository) GetPendingApprovals(ctx context.Context, workspaceID string, limit int) ([]ApprovalRequestRow, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := r.q.Query(ctx,
		`SELECT id, workspace_id, COALESCE(receipt_id::text, ''), policy_id, caller_context,
		        target_number_hash, target_region, purpose, status, expires_at, created_at
		 FROM call_approval_requests
		 WHERE workspace_id = $1::uuid AND status = 'pending'::call_approval_status
		 ORDER BY created_at ASC LIMIT $2`,
		workspaceID, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("get pending approvals: %w", err)
	}
	defer rows.Close()

	var result []ApprovalRequestRow
	for rows.Next() {
		var r ApprovalRequestRow
		if err := rows.Scan(&r.ID, &r.WorkspaceID, &r.ReceiptID, &r.PolicyID, &r.CallerContext,
			&r.TargetNumberHash, &r.TargetRegion, &r.Purpose, &r.Status, &r.ExpiresAt, &r.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan approval: %w", err)
		}
		result = append(result, r)
	}
	return result, rows.Err()
}

// --- Call lifecycle ---

func (r *PgCallRepository) InsertCall(ctx context.Context, row CallRow) error {
	metadata := row.Metadata
	if metadata == "" {
		metadata = "{}"
	}
	_, err := r.q.Exec(ctx,
		`INSERT INTO calls (id, workspace_id, approval_request_id, provider_id, direction, status, target_number_hash, metadata)
		 VALUES ($1::uuid, $2::uuid, $3::uuid, $4::uuid, $5::call_direction, $6::call_status, $7, $8::jsonb)`,
		row.ID, row.WorkspaceID, row.ApprovalRequestID, row.ProviderID,
		row.Direction, row.Status, row.TargetNumberHash, metadata,
	)
	if err != nil {
		return fmt.Errorf("insert call: %w", err)
	}
	return nil
}

func (r *PgCallRepository) UpdateCallStatus(ctx context.Context, id, status string) error {
	_, err := r.q.Exec(ctx,
		`UPDATE calls SET status = $2::call_status, updated_at = now() WHERE id = $1::uuid`, id, status)
	if err != nil {
		return fmt.Errorf("update call status: %w", err)
	}
	return nil
}

func (r *PgCallRepository) UpdateCallProvider(ctx context.Context, id, providerCallID string, failoverCount int) error {
	_, err := r.q.Exec(ctx,
		`UPDATE calls SET provider_call_id = $2, failover_count = $3, started_at = now(), updated_at = now()
		 WHERE id = $1::uuid`, id, providerCallID, failoverCount)
	if err != nil {
		return fmt.Errorf("update call provider: %w", err)
	}
	return nil
}

func (r *PgCallRepository) CompleteCall(ctx context.Context, id string, durationSeconds int, costUSD float64) error {
	_, err := r.q.Exec(ctx,
		`UPDATE calls SET status = 'completed'::call_status, ended_at = now(), duration_seconds = $2, cost_usd = $3, updated_at = now()
		 WHERE id = $1::uuid`, id, durationSeconds, costUSD)
	if err != nil {
		return fmt.Errorf("complete call: %w", err)
	}
	return nil
}

func (r *PgCallRepository) GetCall(ctx context.Context, id string) (*CallRow, error) {
	var row CallRow
	err := r.q.QueryRow(ctx,
		`SELECT id, workspace_id, approval_request_id, provider_id, direction, status,
		        target_number_hash, started_at, answered_at, ended_at, duration_seconds,
		        COALESCE(provider_call_id, ''), failover_count, cost_usd, metadata::text, created_at
		 FROM calls WHERE id = $1::uuid`, id,
	).Scan(&row.ID, &row.WorkspaceID, &row.ApprovalRequestID, &row.ProviderID,
		&row.Direction, &row.Status, &row.TargetNumberHash, &row.StartedAt,
		&row.AnsweredAt, &row.EndedAt, &row.DurationSeconds, &row.ProviderCallID,
		&row.FailoverCount, &row.CostUSD, &row.Metadata, &row.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("get call: %w", err)
	}
	return &row, nil
}

func (r *PgCallRepository) GetCallByProviderID(ctx context.Context, providerCallID string) (*CallRow, error) {
	var row CallRow
	err := r.q.QueryRow(ctx,
		`SELECT id, workspace_id, approval_request_id, provider_id, direction, status,
		        target_number_hash, started_at, answered_at, ended_at, duration_seconds,
		        COALESCE(provider_call_id, ''), failover_count, cost_usd, metadata::text, created_at
		 FROM calls WHERE provider_call_id = $1 LIMIT 1`, providerCallID,
	).Scan(&row.ID, &row.WorkspaceID, &row.ApprovalRequestID, &row.ProviderID,
		&row.Direction, &row.Status, &row.TargetNumberHash, &row.StartedAt,
		&row.AnsweredAt, &row.EndedAt, &row.DurationSeconds, &row.ProviderCallID,
		&row.FailoverCount, &row.CostUSD, &row.Metadata, &row.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("get call by provider id: %w", err)
	}
	return &row, nil
}

func (r *PgCallRepository) ListCalls(ctx context.Context, workspaceID string, limit int) ([]CallRow, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := r.q.Query(ctx,
		`SELECT id, workspace_id, approval_request_id, provider_id, direction, status,
		        target_number_hash, started_at, answered_at, ended_at, duration_seconds,
		        COALESCE(provider_call_id, ''), failover_count, cost_usd, metadata::text, created_at
		 FROM calls WHERE workspace_id = $1::uuid ORDER BY created_at DESC LIMIT $2`,
		workspaceID, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("list calls: %w", err)
	}
	defer rows.Close()

	var result []CallRow
	for rows.Next() {
		var c CallRow
		if err := rows.Scan(&c.ID, &c.WorkspaceID, &c.ApprovalRequestID, &c.ProviderID,
			&c.Direction, &c.Status, &c.TargetNumberHash, &c.StartedAt,
			&c.AnsweredAt, &c.EndedAt, &c.DurationSeconds, &c.ProviderCallID,
			&c.FailoverCount, &c.CostUSD, &c.Metadata, &c.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan call: %w", err)
		}
		result = append(result, c)
	}
	return result, rows.Err()
}

// --- Transcript segments ---

func (r *PgCallRepository) InsertTranscriptSegment(ctx context.Context, row TranscriptSegmentRow) error {
	_, err := r.q.Exec(ctx,
		`INSERT INTO call_transcripts (workspace_id, call_id, segment_index, segment_type, speaker, content, started_at_ms, duration_ms, confidence, language)
		 VALUES ($1::uuid, $2::uuid, $3, $4::transcript_segment_type, $5, $6, $7, $8, $9, $10)`,
		row.WorkspaceID, row.CallID, row.SegmentIndex, row.SegmentType,
		row.Speaker, row.Content, row.StartedAtMs, row.DurationMs, row.Confidence, row.Language,
	)
	if err != nil {
		return fmt.Errorf("insert transcript segment: %w", err)
	}
	return nil
}

func (r *PgCallRepository) GetTranscriptSegments(ctx context.Context, callID string) ([]TranscriptSegmentRow, error) {
	rows, err := r.q.Query(ctx,
		`SELECT id, workspace_id, call_id, segment_index, segment_type, speaker, content,
		        started_at_ms, duration_ms, confidence, language
		 FROM call_transcripts WHERE call_id = $1::uuid ORDER BY segment_index ASC`, callID,
	)
	if err != nil {
		return nil, fmt.Errorf("get transcript segments: %w", err)
	}
	defer rows.Close()

	var result []TranscriptSegmentRow
	for rows.Next() {
		var s TranscriptSegmentRow
		if err := rows.Scan(&s.ID, &s.WorkspaceID, &s.CallID, &s.SegmentIndex, &s.SegmentType,
			&s.Speaker, &s.Content, &s.StartedAtMs, &s.DurationMs, &s.Confidence, &s.Language); err != nil {
			return nil, fmt.Errorf("scan transcript segment: %w", err)
		}
		result = append(result, s)
	}
	return result, rows.Err()
}

// --- Call events ---

func (r *PgCallRepository) InsertCallEvent(ctx context.Context, row CallEventRow) error {
	eventData := row.EventData
	if eventData == "" {
		eventData = "{}"
	}
	_, err := r.q.Exec(ctx,
		`INSERT INTO call_events (workspace_id, call_id, event_type, event_data)
		 VALUES ($1::uuid, $2::uuid, $3, $4::jsonb)`,
		row.WorkspaceID, row.CallID, row.EventType, eventData,
	)
	if err != nil {
		return fmt.Errorf("insert call event: %w", err)
	}
	return nil
}

func (r *PgCallRepository) GetCallEvents(ctx context.Context, callID string, limit int) ([]CallEventRow, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := r.q.Query(ctx,
		`SELECT id, workspace_id, call_id, event_type, event_data::text
		 FROM call_events WHERE call_id = $1::uuid ORDER BY created_at ASC LIMIT $2`, callID, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("get call events: %w", err)
	}
	defer rows.Close()

	var result []CallEventRow
	for rows.Next() {
		var e CallEventRow
		if err := rows.Scan(&e.ID, &e.WorkspaceID, &e.CallID, &e.EventType, &e.EventData); err != nil {
			return nil, fmt.Errorf("scan call event: %w", err)
		}
		result = append(result, e)
	}
	return result, rows.Err()
}

// --- Provider health ---

func (r *PgCallRepository) RecordProviderHealth(ctx context.Context, row ProviderHealthRow) error {
	details := row.Details
	if details == "" {
		details = "{}"
	}
	_, err := r.q.Exec(ctx,
		`INSERT INTO call_provider_health_log (workspace_id, provider_id, health_score, latency_ms, error_count, success_count, check_type, details)
		 VALUES ($1::uuid, $2::uuid, $3, $4, $5, $6, $7, $8::jsonb)`,
		row.WorkspaceID, row.ProviderID, row.HealthScore, row.LatencyMs,
		row.ErrorCount, row.SuccessCount, row.CheckType, details,
	)
	if err != nil {
		return fmt.Errorf("record provider health: %w", err)
	}
	return nil
}

func (r *PgCallRepository) GetProviderHealth(ctx context.Context, providerID string, limit int) ([]ProviderHealthRow, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := r.q.Query(ctx,
		`SELECT id, workspace_id, provider_id, health_score, latency_ms, error_count, success_count, check_type, details::text
		 FROM call_provider_health_log WHERE provider_id = $1::uuid ORDER BY created_at DESC LIMIT $2`,
		providerID, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("get provider health: %w", err)
	}
	defer rows.Close()

	var result []ProviderHealthRow
	for rows.Next() {
		var h ProviderHealthRow
		if err := rows.Scan(&h.ID, &h.WorkspaceID, &h.ProviderID, &h.HealthScore,
			&h.LatencyMs, &h.ErrorCount, &h.SuccessCount, &h.CheckType, &h.Details); err != nil {
			return nil, fmt.Errorf("scan provider health: %w", err)
		}
		result = append(result, h)
	}
	return result, rows.Err()
}

func (r *PgCallRepository) GetProvider(ctx context.Context, workspaceID, providerName string) (*ProviderRow, error) {
	var row ProviderRow
	err := r.q.QueryRow(ctx,
		`SELECT id, workspace_id, provider_name, provider_type, status, priority,
		        health_score, max_concurrent_calls, current_concurrent_calls
		 FROM call_providers WHERE workspace_id = $1::uuid AND provider_name = $2`,
		workspaceID, providerName,
	).Scan(&row.ID, &row.WorkspaceID, &row.ProviderName, &row.ProviderType,
		&row.Status, &row.Priority, &row.HealthScore, &row.MaxConcurrentCalls, &row.CurrentConcurrentCalls)
	if err != nil {
		return nil, fmt.Errorf("get provider: %w", err)
	}
	return &row, nil
}

func (r *PgCallRepository) UpdateProviderStatus(ctx context.Context, id, status string, healthScore float64) error {
	_, err := r.q.Exec(ctx,
		`UPDATE call_providers SET status = $2::call_provider_status, health_score = $3, last_health_check_at = now(), updated_at = now()
		 WHERE id = $1::uuid`, id, status, healthScore)
	if err != nil {
		return fmt.Errorf("update provider status: %w", err)
	}
	return nil
}

// --- Rate limits ---

func (r *PgCallRepository) IncrementRateLimit(ctx context.Context, workspaceID string, windowStart, windowEnd time.Time, maxCalls int) (int, error) {
	var count int
	err := r.q.QueryRow(ctx,
		`INSERT INTO call_rate_limits (workspace_id, window_start, window_end, call_count, max_calls)
		 VALUES ($1::uuid, $2, $3, 1, $4)
		 ON CONFLICT (workspace_id, window_start) DO UPDATE SET call_count = call_rate_limits.call_count + 1, updated_at = now()
		 RETURNING call_count`,
		workspaceID, windowStart, windowEnd, maxCalls,
	).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("increment rate limit: %w", err)
	}
	return count, nil
}

// --- Blocklist ---

func (r *PgCallRepository) IsNumberBlocked(ctx context.Context, workspaceID, numberHash string) (bool, error) {
	var exists bool
	err := r.q.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM call_number_blocklist WHERE workspace_id = $1::uuid AND number_hash = $2)`,
		workspaceID, numberHash,
	).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("check blocklist: %w", err)
	}
	return exists, nil
}

// --- Approval policies ---

func (r *PgCallRepository) GetActivePolicy(ctx context.Context, workspaceID string) (*ApprovalPolicyRow, error) {
	var row ApprovalPolicyRow
	var allowedRegionsJSON, blockedNumbersJSON []byte
	err := r.q.QueryRow(ctx,
		`SELECT id, workspace_id, name, auto_approve_conditions::text, require_approval_conditions::text,
		        deny_conditions::text, max_daily_calls, max_call_duration_seconds,
		        allowed_hours_start::text, allowed_hours_end::text,
		        COALESCE(allowed_regions, '{}'), COALESCE(blocked_numbers, '{}')
		 FROM call_approval_policies
		 WHERE workspace_id = $1::uuid AND is_active = true
		 ORDER BY created_at ASC LIMIT 1`,
		workspaceID,
	).Scan(&row.ID, &row.WorkspaceID, &row.Name, &row.AutoApproveConditions,
		&row.RequireApprovalConditions, &row.DenyConditions, &row.MaxDailyCalls,
		&row.MaxCallDurationSeconds, &row.AllowedHoursStart, &row.AllowedHoursEnd,
		&allowedRegionsJSON, &blockedNumbersJSON)
	if err != nil {
		return nil, fmt.Errorf("get active policy: %w", err)
	}
	_ = json.Unmarshal(allowedRegionsJSON, &row.AllowedRegions)
	_ = json.Unmarshal(blockedNumbersJSON, &row.BlockedNumbers)
	return &row, nil
}

// nilIfEmpty returns nil for empty strings (for nullable UUID columns).
func nilIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}
