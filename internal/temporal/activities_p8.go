package temporal

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/brevio/brevio/internal/compliance/eu_ai_act"
	"github.com/google/uuid"
)

// ===================== FEDERATION ACTIVITY TYPES =====================

type FederationPolicyCheckInput struct {
	WorkspaceID          string   `json:"workspace_id"`
	PeerWorkspaceID      string   `json:"peer_workspace_id"`
	RequestedPermissions []string `json:"requested_permissions"`
}

type FederationPolicyCheckResult struct {
	Allowed            bool     `json:"allowed"`
	AllowedPermissions []string `json:"allowed_permissions"`
	DeniedPermissions  []string `json:"denied_permissions"`
	Reason             string   `json:"reason,omitempty"`
	EvidenceHash       string   `json:"evidence_hash"`
}

type FederationNegotiateInput struct {
	WorkspaceID        string   `json:"workspace_id"`
	PeerWorkspaceID    string   `json:"peer_workspace_id"`
	AllowedPermissions []string `json:"allowed_permissions"`
}

type FederationNegotiateResult struct {
	NegotiationID       string   `json:"negotiation_id"`
	Status              string   `json:"status"` // accepted, rejected
	AcceptedPermissions []string `json:"accepted_permissions"`
	EvidenceHash        string   `json:"evidence_hash"`
}

type FederationCompensateInput struct {
	WorkspaceID     string `json:"workspace_id"`
	PeerWorkspaceID string `json:"peer_workspace_id"`
	NegotiationID   string `json:"negotiation_id"`
	Reason          string `json:"reason"`
}

type FederationCompensateResult struct {
	Compensated  bool   `json:"compensated"`
	EvidenceHash string `json:"evidence_hash"`
}

// ===================== EDGE SYNC ACTIVITY TYPES =====================

type EdgeFetchTasksInput struct {
	WorkspaceID string `json:"workspace_id"`
	AgentID     string `json:"agent_id"`
	BatchSize   int    `json:"batch_size"`
}

type EdgeTask struct {
	ID              string `json:"id"`
	TaskType        string `json:"task_type"`
	Payload         string `json:"payload"`
	Priority        int    `json:"priority"`
	IdempotencyKey  string `json:"idempotency_key"`
}

type EdgeFetchTasksResult struct {
	Tasks []EdgeTask `json:"tasks"`
}

type EdgeConflictDetectInput struct {
	WorkspaceID string     `json:"workspace_id"`
	Tasks       []EdgeTask `json:"tasks"`
}

type EdgeConflict struct {
	TaskID          string `json:"task_id"`
	ConflictType    string `json:"conflict_type"`
	ExistingTaskID  string `json:"existing_task_id"`
	Resolution      string `json:"resolution"` // server_wins, client_wins, merge
}

type EdgeConflictDetectResult struct {
	ConflictsFound int            `json:"conflicts_found"`
	Conflicts      []EdgeConflict `json:"conflicts"`
	ResolvedKeys   []string       `json:"resolved_keys"`
}

type EdgeConflictResolveInput struct {
	WorkspaceID string         `json:"workspace_id"`
	Conflicts   []EdgeConflict `json:"conflicts"`
}

type EdgeConflictResolveResult struct {
	Resolved int `json:"resolved"`
}

type EdgeExecuteTasksInput struct {
	WorkspaceID  string     `json:"workspace_id"`
	Tasks        []EdgeTask `json:"tasks"`
	ResolvedKeys []string   `json:"resolved_keys"`
}

type EdgeExecuteTasksResult struct {
	Executed     int    `json:"executed"`
	Failed       int    `json:"failed"`
	EvidenceHash string `json:"evidence_hash"`
}

// ===================== BROWSER ACTIVITY TYPES =====================

type BrowserReceiptCheckInput struct {
	WorkspaceID string `json:"workspace_id"`
	ReceiptID   string `json:"receipt_id"`
	SessionType string `json:"session_type"`
}

type BrowserReceiptCheckResult struct {
	Valid  bool   `json:"valid"`
	Reason string `json:"reason,omitempty"`
}

type BrowserSessionInput struct {
	WorkspaceID string `json:"workspace_id"`
	SessionType string `json:"session_type"`
	URL         string `json:"url"`
	Parameters  string `json:"parameters"`
}

type BrowserSessionResult struct {
	SessionID string `json:"session_id"`
	Status    string `json:"status"`
}

type BrowserTaskInput struct {
	WorkspaceID string `json:"workspace_id"`
	SessionID   string `json:"session_id"`
	SessionType string `json:"session_type"`
	URL         string `json:"url"`
	Parameters  string `json:"parameters"`
}

type BrowserTaskResult struct {
	Result       string `json:"result"`
	EvidenceHash string `json:"evidence_hash"`
}

type BrowserCloseInput struct {
	SessionID string `json:"session_id"`
}

type BrowserCloseResult struct {
	Closed bool `json:"closed"`
}

// ===================== FAST-PATH ACTIVITY TYPES =====================

type FastPathMatchInput struct {
	WorkspaceID     string `json:"workspace_id"`
	MessageID       string `json:"message_id"`
	Payload         string `json:"payload"`
	LatencyBudgetMs int    `json:"latency_budget_ms"`
}

type FastPathMatchResult struct {
	Matched      bool    `json:"matched"`
	Response     string  `json:"response,omitempty"`
	RouteID      string  `json:"route_id,omitempty"`
	LatencyMs    float64 `json:"latency_ms"`
	Confidence   float64 `json:"confidence"`
	EvidenceHash string  `json:"evidence_hash"`
}

type FastPathMetricInput struct {
	WorkspaceID string  `json:"workspace_id"`
	RouteID     string  `json:"route_id"`
	LatencyMs   float64 `json:"latency_ms"`
	Hit         bool    `json:"hit"`
}

type FastPathMetricResult struct {
	Recorded bool `json:"recorded"`
}

// ===================== EXPERIMENT ACTIVITY TYPES =====================

type ExperimentExistingInput struct {
	WorkspaceID  string `json:"workspace_id"`
	ExperimentID string `json:"experiment_id"`
	SubjectID    string `json:"subject_id"`
}

type ExperimentExistingResult struct {
	Found        bool   `json:"found"`
	AssignmentID string `json:"assignment_id,omitempty"`
	VariantID    string `json:"variant_id,omitempty"`
	VariantName  string `json:"variant_name,omitempty"`
	EvidenceHash string `json:"evidence_hash"`
}

type ExperimentDeterministicInput struct {
	WorkspaceID  string `json:"workspace_id"`
	ExperimentID string `json:"experiment_id"`
	SubjectID    string `json:"subject_id"`
}

type ExperimentDeterministicResult struct {
	VariantID    string `json:"variant_id"`
	VariantName  string `json:"variant_name"`
	EvidenceHash string `json:"evidence_hash"`
}

type ExperimentPersistInput struct {
	WorkspaceID  string `json:"workspace_id"`
	ExperimentID string `json:"experiment_id"`
	SubjectID    string `json:"subject_id"`
	VariantID    string `json:"variant_id"`
	VariantName  string `json:"variant_name"`
}

type ExperimentPersistResult struct {
	AssignmentID string `json:"assignment_id"`
	Persisted    bool   `json:"persisted"`
}

// ===================== ONBOARDING ACTIVITY TYPES =====================

type OnboardingInitInput struct {
	WorkspaceID string `json:"workspace_id"`
	PlanID      string `json:"plan_id"`
	OperatorID  string `json:"operator_id"`
}

type OnboardingInitResult struct {
	SessionID string `json:"session_id"`
}

type OnboardingStageExecInput struct {
	WorkspaceID string `json:"workspace_id"`
	SessionID   string `json:"session_id"`
	Stage       string `json:"stage"`
	PlanID      string `json:"plan_id"`
}

type OnboardingStageExecResult struct {
	Stage   string `json:"stage"`
	Success bool   `json:"success"`
}

type OnboardingFinalizeInput struct {
	WorkspaceID        string   `json:"workspace_id"`
	SessionID          string   `json:"session_id"`
	CompletedStages    []string `json:"completed_stages"`
	FirstValueVerified bool     `json:"first_value_verified"`
	Status             string   `json:"status"`
}

type OnboardingFinalizeResult struct {
	Finalized    bool   `json:"finalized"`
	EvidenceHash string `json:"evidence_hash"`
}

// ===================== BILLING ACTIVITY TYPES =====================

type BillingIngestInput struct {
	WorkspaceID string `json:"workspace_id"`
	Provider    string `json:"provider"`
	EventType   string `json:"event_type"`
	EventID     string `json:"event_id"`
	Payload     string `json:"payload"`
}

type BillingIngestResult struct {
	Ingested     bool   `json:"ingested"`
	Duplicate    bool   `json:"duplicate"`
	EvidenceHash string `json:"evidence_hash"`
}

type BillingLedgerInput struct {
	WorkspaceID string `json:"workspace_id"`
	EventType   string `json:"event_type"`
	EventID     string `json:"event_id"`
	Payload     string `json:"payload"`
}

type BillingLedgerResult struct {
	EntryID      string `json:"entry_id"`
	AmountCents  int64  `json:"amount_cents"`
	EntryType    string `json:"entry_type"`
	EvidenceHash string `json:"evidence_hash"`
}

type BillingPolicyInput struct {
	WorkspaceID string `json:"workspace_id"`
	EventType   string `json:"event_type"`
	AmountCents int64  `json:"amount_cents"`
}

type BillingPolicyResult struct {
	Enforced     bool   `json:"enforced"`
	Action       string `json:"action"` // downgrade, suspend, notify
	EvidenceHash string `json:"evidence_hash"`
}

// ===================== LOAD SHEDDING ACTIVITY TYPES =====================

type LoadSheddingEvalInput struct {
	WorkspaceID string  `json:"workspace_id"`
	CPUPercent  float64 `json:"cpu_percent"`
	ErrorRate   float64 `json:"error_rate"`
	DBPoolUsage float64 `json:"db_pool_usage"`
}

type LoadSheddingEvalResult struct {
	NewTier      string `json:"new_tier"`
	PreviousTier string `json:"previous_tier"`
	Changed      bool   `json:"changed"`
	Reason       string `json:"reason"`
	EvidenceHash string `json:"evidence_hash"`
}

type LoadSheddingPropagateInput struct {
	WorkspaceID  string `json:"workspace_id"`
	NewTier      string `json:"new_tier"`
	PreviousTier string `json:"previous_tier"`
	Reason       string `json:"reason"`
}

type LoadSheddingPropagateResult struct {
	Propagated bool `json:"propagated"`
}

// ===================== FEDERATION ACTIVITY IMPLEMENTATIONS =====================

// CheckFederationPolicyActivity validates federation permissions against policy gates.
func (a *Activities) CheckFederationPolicyActivity(ctx context.Context, input FederationPolicyCheckInput) (*FederationPolicyCheckResult, error) {
	if input.WorkspaceID == "" || input.PeerWorkspaceID == "" {
		return &FederationPolicyCheckResult{Allowed: false, Reason: "missing_workspace"}, nil
	}

	// Validate each requested permission against allowed federation types.
	allowedTypes := map[string]bool{
		"calendar_query":    true,
		"calendar_write":    true,
		"routing_negotiate": true,
		"task_delegate":     true,
		"knowledge_share":   true,
		"status_query":      true,
	}

	var allowed, denied []string
	for _, perm := range input.RequestedPermissions {
		if allowedTypes[perm] {
			allowed = append(allowed, perm)
		} else {
			denied = append(denied, perm)
		}
	}
	sort.Strings(allowed)
	sort.Strings(denied)

	evidence := hashKey(fmt.Sprintf("fed-policy:%s:%s:%s",
		input.WorkspaceID, input.PeerWorkspaceID, strings.Join(allowed, ",")))

	return &FederationPolicyCheckResult{
		Allowed:            len(allowed) > 0,
		AllowedPermissions: allowed,
		DeniedPermissions:  denied,
		EvidenceHash:       evidence,
	}, nil
}

// ExecuteFederationNegotiateActivity executes the negotiation with a peer workspace.
func (a *Activities) ExecuteFederationNegotiateActivity(ctx context.Context, input FederationNegotiateInput) (*FederationNegotiateResult, error) {
	if input.WorkspaceID == "" || input.PeerWorkspaceID == "" {
		return nil, fmt.Errorf("FEDERATION_VALIDATION_FAILED: missing workspace IDs")
	}

	negotiationID := hashKey(fmt.Sprintf("neg:%s:%s:%d",
		input.WorkspaceID, input.PeerWorkspaceID, time.Now().UnixNano()))

	// Accept permissions that both sides agree on.
	accepted := make([]string, len(input.AllowedPermissions))
	copy(accepted, input.AllowedPermissions)
	sort.Strings(accepted)

	status := "accepted"
	if len(accepted) == 0 {
		status = "rejected"
	}

	evidence := hashKey(fmt.Sprintf("neg-result:%s:%s:%s",
		negotiationID, status, strings.Join(accepted, ",")))

	return &FederationNegotiateResult{
		NegotiationID:       negotiationID,
		Status:              status,
		AcceptedPermissions: accepted,
		EvidenceHash:        evidence,
	}, nil
}

// CompensateFederationActivity rolls back a failed federation sync.
func (a *Activities) CompensateFederationActivity(ctx context.Context, input FederationCompensateInput) (*FederationCompensateResult, error) {
	evidence := hashKey(fmt.Sprintf("comp:%s:%s:%s",
		input.NegotiationID, input.WorkspaceID, input.Reason))

	// In production, this would revoke federation permissions and roll back synced data.
	// With pgx pool, we would persist compensation records.
	if a.pool != nil {
		_, err := a.pool.Exec(ctx, `
			UPDATE federation_sync_log
			SET status = 'compensated', compensated = TRUE, completed_at = now()
			WHERE workspace_id = $1::uuid AND peer_workspace_id = $2::uuid AND status = 'running'`,
			input.WorkspaceID, input.PeerWorkspaceID)
		if err != nil {
			return &FederationCompensateResult{Compensated: false, EvidenceHash: evidence}, nil
		}
	}

	return &FederationCompensateResult{
		Compensated:  true,
		EvidenceHash: evidence,
	}, nil
}

// ===================== EDGE SYNC ACTIVITY IMPLEMENTATIONS =====================

// FetchEdgeTasksActivity retrieves pending offline tasks for an edge agent.
func (a *Activities) FetchEdgeTasksActivity(ctx context.Context, input EdgeFetchTasksInput) (*EdgeFetchTasksResult, error) {
	if input.WorkspaceID == "" || input.AgentID == "" {
		return nil, fmt.Errorf("EDGE_VALIDATION_FAILED: missing workspace or agent ID")
	}

	batchSize := input.BatchSize
	if batchSize <= 0 {
		batchSize = 50
	}

	if a.pool != nil {
		rows, err := a.pool.Query(ctx, `
			SELECT id, task_type, payload::text, priority, idempotency_key
			FROM edge_sync_tasks
			WHERE workspace_id = $1::uuid AND agent_id = $2 AND status = 'queued'
			ORDER BY priority DESC, created_at ASC
			LIMIT $3
			FOR UPDATE SKIP LOCKED`,
			input.WorkspaceID, input.AgentID, batchSize)
		if err != nil {
			return nil, fmt.Errorf("fetch edge tasks: %w", err)
		}
		defer rows.Close()

		var tasks []EdgeTask
		for rows.Next() {
			var t EdgeTask
			if err := rows.Scan(&t.ID, &t.TaskType, &t.Payload, &t.Priority, &t.IdempotencyKey); err != nil {
				return nil, fmt.Errorf("scan edge task: %w", err)
			}
			tasks = append(tasks, t)
		}
		return &EdgeFetchTasksResult{Tasks: tasks}, nil
	}

	// Degraded mode: return empty task list.
	return &EdgeFetchTasksResult{Tasks: []EdgeTask{}}, nil
}

// DetectEdgeConflictsActivity checks for conflicts between edge tasks and server state.
func (a *Activities) DetectEdgeConflictsActivity(ctx context.Context, input EdgeConflictDetectInput) (*EdgeConflictDetectResult, error) {
	var conflicts []EdgeConflict
	var resolvedKeys []string

	for _, task := range input.Tasks {
		if a.pool != nil {
			// Check for idempotency conflicts.
			var existingID string
			err := a.pool.QueryRow(ctx, `
				SELECT id FROM edge_sync_tasks
				WHERE workspace_id = $1::uuid AND idempotency_key = $2 AND status IN ('synced', 'executed')`,
				input.WorkspaceID, task.IdempotencyKey).Scan(&existingID)
			if err == nil {
				conflicts = append(conflicts, EdgeConflict{
					TaskID:         task.ID,
					ConflictType:   "idempotency_duplicate",
					ExistingTaskID: existingID,
					Resolution:     "server_wins",
				})
				resolvedKeys = append(resolvedKeys, task.IdempotencyKey)
			}
		}
	}

	sort.Strings(resolvedKeys)

	return &EdgeConflictDetectResult{
		ConflictsFound: len(conflicts),
		Conflicts:      conflicts,
		ResolvedKeys:   resolvedKeys,
	}, nil
}

// ResolveEdgeConflictsActivity resolves detected edge conflicts.
func (a *Activities) ResolveEdgeConflictsActivity(ctx context.Context, input EdgeConflictResolveInput) (*EdgeConflictResolveResult, error) {
	resolved := 0
	for _, conflict := range input.Conflicts {
		if a.pool != nil {
			_, err := a.pool.Exec(ctx, `
				UPDATE edge_sync_tasks
				SET status = 'conflict', conflict_resolution = $2
				WHERE id = $1::uuid`,
				conflict.TaskID, conflict.Resolution)
			if err == nil {
				resolved++
			}
		} else {
			resolved++ // Degraded mode: consider all resolved.
		}
	}
	return &EdgeConflictResolveResult{Resolved: resolved}, nil
}

// ExecuteEdgeTasksActivity executes synced edge tasks with idempotency.
func (a *Activities) ExecuteEdgeTasksActivity(ctx context.Context, input EdgeExecuteTasksInput) (*EdgeExecuteTasksResult, error) {
	resolvedSet := make(map[string]bool)
	for _, k := range input.ResolvedKeys {
		resolvedSet[k] = true
	}

	executed, failed := 0, 0
	for _, task := range input.Tasks {
		if resolvedSet[task.IdempotencyKey] {
			continue // Skip conflicted tasks.
		}

		if a.pool != nil {
			_, err := a.pool.Exec(ctx, `
				UPDATE edge_sync_tasks
				SET status = 'executed', executed_at = now()
				WHERE id = $1::uuid AND status IN ('queued', 'syncing')`,
				task.ID)
			if err != nil {
				failed++
				continue
			}
		}
		executed++
	}

	evidence := hashKey(fmt.Sprintf("edge-exec:%s:%d:%d",
		input.WorkspaceID, executed, failed))

	return &EdgeExecuteTasksResult{
		Executed:     executed,
		Failed:       failed,
		EvidenceHash: evidence,
	}, nil
}

// ===================== BROWSER ACTIVITY IMPLEMENTATIONS =====================

// ValidateBrowserReceiptActivity validates an authorization receipt for browser automation.
func (a *Activities) ValidateBrowserReceiptActivity(ctx context.Context, input BrowserReceiptCheckInput) (*BrowserReceiptCheckResult, error) {
	if input.ReceiptID == "" {
		return &BrowserReceiptCheckResult{Valid: false, Reason: "missing_receipt"}, nil
	}
	if input.WorkspaceID == "" {
		return &BrowserReceiptCheckResult{Valid: false, Reason: "missing_workspace"}, nil
	}

	validTypes := map[string]bool{
		"scrape": true, "form_fill": true, "booking": true,
		"price_watch": true, "screenshot": true,
	}
	if !validTypes[input.SessionType] {
		return &BrowserReceiptCheckResult{Valid: false, Reason: "invalid_session_type"}, nil
	}

	// In production, validate receipt against control plane.
	return &BrowserReceiptCheckResult{Valid: true}, nil
}

// StartBrowserSessionActivity starts a browser automation session.
func (a *Activities) StartBrowserSessionActivity(ctx context.Context, input BrowserSessionInput) (*BrowserSessionResult, error) {
	if input.URL == "" {
		return nil, fmt.Errorf("BROWSER_VALIDATION_FAILED: missing URL")
	}

	// Validate URL against sandbox profile allowlist (Go-side defense-in-depth).
	if a.browserSandboxSvc != nil {
		profile, err := a.browserSandboxSvc.GetProfile("strict")
		if err == nil && profile != nil && !profile.AllowsHost(input.URL) {
			return nil, fmt.Errorf("BROWSER_URL_DENIED: %s not in strict sandbox allowlist", input.URL)
		}
	}

	sessionID := hashKey(fmt.Sprintf("browser:%s:%s:%s",
		input.WorkspaceID, input.SessionType, input.URL))

	// Call browser-mcp service to start a real Playwright session.
	if a.browserClient != nil {
		if err := a.browserClient.StartSession(ctx, sessionID, input.WorkspaceID, input.URL, input.SessionType); err != nil {
			return nil, fmt.Errorf("StartBrowserSessionActivity: browser-mcp start: %w", err)
		}
	}

	if a.pool != nil {
		_, _ = a.pool.Exec(ctx, `
			INSERT INTO browser_sessions (id, workspace_id, url, status, session_type, started_at, created_at)
			VALUES (gen_random_uuid(), $1::uuid, $2, 'active', $3, now(), now())
			ON CONFLICT DO NOTHING`,
			input.WorkspaceID, input.URL, input.SessionType)
	}

	return &BrowserSessionResult{
		SessionID: sessionID,
		Status:    "active",
	}, nil
}

// ExecuteBrowserTaskActivity executes a browser automation task within a session.
func (a *Activities) ExecuteBrowserTaskActivity(ctx context.Context, input BrowserTaskInput) (*BrowserTaskResult, error) {
	var resultJSON string
	var execErr error

	var params map[string]any
	if input.Parameters != "" {
		_ = json.Unmarshal([]byte(input.Parameters), &params)
	}
	if params == nil {
		params = make(map[string]any)
	}

	if a.browserClient != nil {
		switch input.SessionType {
		case "scrape":
			selectors := extractStringMapFromParams(params, "selectors")
			sr, e := a.browserClient.Scrape(ctx, input.SessionID, input.URL, selectors)
			if e != nil {
				execErr = e
			} else {
				b, _ := json.Marshal(sr)
				resultJSON = string(b)
			}
		case "form_fill":
			fields := extractStringMapFromParams(params, "fields")
			submitSel, _ := params["submit_selector"].(string)
			if len(fields) == 0 {
				return nil, fmt.Errorf("BROWSER_VALIDATION_FAILED: form_fill requires fields parameter")
			}
			fr, e := a.browserClient.FormFill(ctx, input.SessionID, input.URL, fields, submitSel)
			if e != nil {
				execErr = e
			} else {
				b, _ := json.Marshal(fr)
				resultJSON = string(b)
			}
		case "booking":
			nr, e := a.browserClient.Navigate(ctx, input.SessionID, input.WorkspaceID, input.URL, "booking")
			if e != nil {
				execErr = e
			} else {
				b, _ := json.Marshal(nr)
				resultJSON = string(b)
			}
		case "price_watch":
			sr, e := a.browserClient.Scrape(ctx, input.SessionID, input.URL, nil)
			if e != nil {
				execErr = e
			} else {
				bodyPreview := sr.BodyText
				if len(bodyPreview) > 2000 {
					bodyPreview = bodyPreview[:2000]
				}
				b, _ := json.Marshal(map[string]any{
					"url": sr.URL, "content": bodyPreview, "scraped_at": time.Now().UTC(),
				})
				resultJSON = string(b)
			}
		case "screenshot":
			ss, e := a.browserClient.Screenshot(ctx, input.SessionID)
			if e != nil {
				execErr = e
			} else {
				b, _ := json.Marshal(ss)
				resultJSON = string(b)
			}
		default:
			return nil, fmt.Errorf("BROWSER_UNKNOWN_TYPE: %s", input.SessionType)
		}
	} else {
		if input.SessionType != "scrape" && input.SessionType != "form_fill" &&
			input.SessionType != "booking" && input.SessionType != "price_watch" &&
			input.SessionType != "screenshot" {
			return nil, fmt.Errorf("BROWSER_UNKNOWN_TYPE: %s", input.SessionType)
		}
		resultJSON = fmt.Sprintf(`{"session_type":%q,"url":%q,"status":"browser_client_not_configured"}`,
			input.SessionType, input.URL)
	}

	if execErr != nil {
		return nil, fmt.Errorf("ExecuteBrowserTaskActivity [%s]: %w", input.SessionType, execErr)
	}

	evidence := hashKey(fmt.Sprintf("browser-task:%s:%s:%s",
		input.WorkspaceID, input.SessionID, input.SessionType))

	return &BrowserTaskResult{
		Result:       resultJSON,
		EvidenceHash: evidence,
	}, nil
}

func extractStringMapFromParams(params map[string]any, key string) map[string]string {
	v, ok := params[key]
	if !ok {
		return nil
	}
	raw, ok := v.(map[string]any)
	if !ok {
		return nil
	}
	result := make(map[string]string, len(raw))
	for k, val := range raw {
		if s, ok := val.(string); ok {
			result[k] = s
		}
	}
	return result
}

// CloseBrowserSessionActivity closes a browser session and persists the result.
func (a *Activities) CloseBrowserSessionActivity(ctx context.Context, input BrowserCloseInput) (*BrowserCloseResult, error) {
	if a.browserClient != nil && input.SessionID != "" {
		_ = a.browserClient.CloseSession(ctx, input.SessionID)
	}
	if a.pool != nil {
		_, _ = a.pool.Exec(ctx, `
			UPDATE browser_sessions SET status = 'completed', completed_at = now()
			WHERE status = 'active'`)
	}
	return &BrowserCloseResult{Closed: true}, nil
}

// ===================== FAST-PATH ACTIVITY IMPLEMENTATIONS =====================

// FastPathMatchActivity attempts to match a message against precomputed fast-path routes.
func (a *Activities) FastPathMatchActivity(ctx context.Context, input FastPathMatchInput) (*FastPathMatchResult, error) {
	start := time.Now()

	if a.pool != nil {
		// Query enabled routes for the workspace.
		rows, err := a.pool.Query(ctx, `
			SELECT id, pattern, response, confidence_threshold, precomputed_answer
			FROM fast_path_routes
			WHERE workspace_id = $1::uuid AND enabled = TRUE
			ORDER BY hit_count DESC
			LIMIT 100`,
			input.WorkspaceID)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var routeID, pattern, response string
				var threshold float64
				var precomputed *string
				if err := rows.Scan(&routeID, &pattern, &response, &threshold, &precomputed); err != nil {
					continue
				}
				if strings.Contains(strings.ToLower(input.Payload), strings.ToLower(pattern)) {
					latency := float64(time.Since(start).Milliseconds())
					resp := response
					if precomputed != nil && *precomputed != "" {
						resp = *precomputed
					}
					evidence := hashKey(fmt.Sprintf("fp:%s:%s:%s", input.WorkspaceID, routeID, input.MessageID))

					// Update hit count.
					_, _ = a.pool.Exec(ctx, `
						UPDATE fast_path_routes
						SET hit_count = hit_count + 1,
							avg_latency_ms = (avg_latency_ms * hit_count + $2) / (hit_count + 1),
							updated_at = now()
						WHERE id = $1::uuid`,
						routeID, latency)

					return &FastPathMatchResult{
						Matched:      true,
						Response:     resp,
						RouteID:      routeID,
						LatencyMs:    latency,
						Confidence:   threshold,
						EvidenceHash: evidence,
					}, nil
				}
			}
		}
	}

	latency := float64(time.Since(start).Milliseconds())
	evidence := hashKey(fmt.Sprintf("fp-miss:%s:%s", input.WorkspaceID, input.MessageID))

	return &FastPathMatchResult{
		Matched:      false,
		LatencyMs:    latency,
		EvidenceHash: evidence,
	}, nil
}

// RecordFastPathMetricActivity records fast-path hit/miss metrics.
func (a *Activities) RecordFastPathMetricActivity(ctx context.Context, input FastPathMetricInput) (*FastPathMetricResult, error) {
	// Metric recording — in production writes to metrics/observability store.
	return &FastPathMetricResult{Recorded: true}, nil
}

// ===================== EXPERIMENT ACTIVITY IMPLEMENTATIONS =====================

// CheckExistingAssignmentActivity checks if a subject already has an experiment assignment.
func (a *Activities) CheckExistingAssignmentActivity(ctx context.Context, input ExperimentExistingInput) (*ExperimentExistingResult, error) {
	if a.pool != nil {
		var assignmentID, variantID, variantName string
		err := a.pool.QueryRow(ctx, `
			SELECT ea.id, ea.variant_id, ed.name
			FROM experiment_assignments ea
			JOIN experiment_definitions ed ON ed.id = ea.experiment_id
			WHERE ea.workspace_id = $1::uuid AND ea.experiment_id = $2::uuid AND ea.subject_id = $3`,
			input.WorkspaceID, input.ExperimentID, input.SubjectID).
			Scan(&assignmentID, &variantID, &variantName)
		if err == nil {
			return &ExperimentExistingResult{
				Found:        true,
				AssignmentID: assignmentID,
				VariantID:    variantID,
				VariantName:  variantName,
				EvidenceHash: hashKey(fmt.Sprintf("exp-existing:%s", assignmentID)),
			}, nil
		}
	}
	return &ExperimentExistingResult{Found: false}, nil
}

// DeterministicAssignActivity assigns a variant deterministically using FNV hash.
func (a *Activities) DeterministicAssignActivity(ctx context.Context, input ExperimentDeterministicInput) (*ExperimentDeterministicResult, error) {
	// Deterministic assignment via FNV-1a hash of subject+experiment.
	seed := fmt.Sprintf("%s:%s:%s", input.WorkspaceID, input.ExperimentID, input.SubjectID)
	hash := fnvHash64(seed)

	// Default 2-variant A/B split.
	variantIndex := hash % 2
	variants := []struct{ id, name string }{
		{"control", "Control"},
		{"treatment", "Treatment"},
	}

	// If pool available, fetch actual variant definitions.
	if a.pool != nil {
		rows, err := a.pool.Query(ctx, `
			SELECT variants FROM experiment_definitions
			WHERE id = $1::uuid AND workspace_id = $2::uuid AND status = 'running'`,
			input.ExperimentID, input.WorkspaceID)
		if err == nil {
			defer rows.Close()
			// Parse variants from JSONB if available — for now use default split.
		}
	}

	selected := variants[variantIndex]
	evidence := hashKey(fmt.Sprintf("exp-assign:%s:%s:%d", seed, selected.id, hash))

	return &ExperimentDeterministicResult{
		VariantID:    selected.id,
		VariantName:  selected.name,
		EvidenceHash: evidence,
	}, nil
}

// PersistAssignmentActivity persists an experiment assignment to the database.
func (a *Activities) PersistAssignmentActivity(ctx context.Context, input ExperimentPersistInput) (*ExperimentPersistResult, error) {
	assignmentID := hashKey(fmt.Sprintf("assign:%s:%s:%s",
		input.ExperimentID, input.SubjectID, input.VariantID))

	if a.pool != nil {
		_, err := a.pool.Exec(ctx, `
			INSERT INTO experiment_assignments (workspace_id, experiment_id, subject_id, variant_id, assigned_at)
			VALUES ($1::uuid, $2::uuid, $3, $4, now())
			ON CONFLICT (workspace_id, experiment_id, subject_id) DO NOTHING`,
			input.WorkspaceID, input.ExperimentID, input.SubjectID, input.VariantID)
		if err != nil {
			return &ExperimentPersistResult{AssignmentID: assignmentID, Persisted: false}, nil
		}
		return &ExperimentPersistResult{AssignmentID: assignmentID, Persisted: true}, nil
	}

	return &ExperimentPersistResult{AssignmentID: assignmentID, Persisted: false}, nil
}

// ===================== ONBOARDING ACTIVITY IMPLEMENTATIONS =====================

// InitOnboardingSessionActivity creates a new onboarding session.
func (a *Activities) InitOnboardingSessionActivity(ctx context.Context, input OnboardingInitInput) (*OnboardingInitResult, error) {
	if input.WorkspaceID == "" {
		return nil, fmt.Errorf("ONBOARDING_VALIDATION_FAILED: missing workspace ID")
	}

	sessionID := hashKey(fmt.Sprintf("onboard:%s:%s", input.WorkspaceID, input.PlanID))

	if a.pool != nil {
		_, err := a.pool.Exec(ctx, `
			INSERT INTO onboarding_sessions (workspace_id, current_stage, status, started_at, created_at)
			VALUES ($1::uuid, 'workspace_setup', 'in_progress', now(), now())
			ON CONFLICT (workspace_id) DO UPDATE SET current_stage = 'workspace_setup', status = 'in_progress'`,
			input.WorkspaceID)
		if err != nil {
			return nil, fmt.Errorf("init onboarding session: %w", err)
		}
	}

	return &OnboardingInitResult{SessionID: sessionID}, nil
}

// ExecuteProvisioningStageActivity executes a single provisioning stage.
func (a *Activities) ExecuteProvisioningStageActivity(ctx context.Context, input OnboardingStageExecInput) (*OnboardingStageExecResult, error) {
	if input.WorkspaceID == "" || input.Stage == "" {
		return nil, fmt.Errorf("ONBOARDING_STAGE_FAILED: missing workspace or stage")
	}

	validStages := map[string]bool{
		"workspace_setup": true, "policy_defaults": true,
		"integration_check": true, "first_value": true,
	}
	if !validStages[input.Stage] {
		return nil, fmt.Errorf("ONBOARDING_UNKNOWN_STAGE: %s", input.Stage)
	}

	if a.pool != nil {
		_, err := a.pool.Exec(ctx, `
			UPDATE onboarding_sessions
			SET current_stage = $2,
				completed_stages = array_append(completed_stages, $2)
			WHERE workspace_id = $1::uuid`,
			input.WorkspaceID, input.Stage)
		if err != nil {
			return &OnboardingStageExecResult{Stage: input.Stage, Success: false}, nil
		}
	}

	return &OnboardingStageExecResult{Stage: input.Stage, Success: true}, nil
}

// FinalizeOnboardingActivity finalizes the onboarding session.
func (a *Activities) FinalizeOnboardingActivity(ctx context.Context, input OnboardingFinalizeInput) (*OnboardingFinalizeResult, error) {
	if a.pool != nil {
		_, _ = a.pool.Exec(ctx, `
			UPDATE onboarding_sessions
			SET status = $2, first_value_verified = $3, completed_at = now()
			WHERE workspace_id = $1::uuid`,
			input.WorkspaceID, input.Status, input.FirstValueVerified)
	}

	evidence := hashKey(fmt.Sprintf("onboard-final:%s:%s:%v",
		input.WorkspaceID, input.Status, input.FirstValueVerified))

	return &OnboardingFinalizeResult{
		Finalized:    true,
		EvidenceHash: evidence,
	}, nil
}

// ===================== BILLING ACTIVITY IMPLEMENTATIONS =====================

// IngestBillingWebhookActivity ingests a billing webhook event idempotently.
func (a *Activities) IngestBillingWebhookActivity(ctx context.Context, input BillingIngestInput) (*BillingIngestResult, error) {
	if input.EventID == "" {
		return nil, fmt.Errorf("BILLING_VALIDATION_FAILED: missing event ID")
	}

	idempotencyKey := fmt.Sprintf("%s:%s:%s", input.Provider, input.EventType, input.EventID)
	evidence := hashKey(fmt.Sprintf("billing-ingest:%s", idempotencyKey))

	if a.pool != nil {
		var existingID string
		err := a.pool.QueryRow(ctx, `
			SELECT id FROM billing_webhook_events WHERE idempotency_key = $1`,
			idempotencyKey).Scan(&existingID)
		if err == nil {
			return &BillingIngestResult{Ingested: false, Duplicate: true, EvidenceHash: evidence}, nil
		}

		_, err = a.pool.Exec(ctx, `
			INSERT INTO billing_webhook_events (workspace_id, provider, event_type, event_id, payload, status, idempotency_key, created_at)
			VALUES ($1::uuid, $2, $3, $4, $5::jsonb, 'received', $6, now())`,
			input.WorkspaceID, input.Provider, input.EventType, input.EventID, input.Payload, idempotencyKey)
		if err != nil {
			return nil, fmt.Errorf("ingest billing webhook: %w", err)
		}
	}

	return &BillingIngestResult{Ingested: true, Duplicate: false, EvidenceHash: evidence}, nil
}

// UpdateBillingLedgerActivity updates the billing ledger based on a webhook event.
func (a *Activities) UpdateBillingLedgerActivity(ctx context.Context, input BillingLedgerInput) (*BillingLedgerResult, error) {
	entryType := "charge"
	var amountCents int64
	description := input.EventType

	switch input.EventType {
	case "invoice.paid":
		entryType = "charge"
		amountCents = 2900 // Default pro plan amount.
		description = "Invoice payment received"
	case "invoice.payment_failed":
		entryType = "adjustment"
		amountCents = 0
		description = "Invoice payment failed"
	case "customer.subscription.deleted":
		entryType = "credit"
		amountCents = 0
		description = "Subscription cancelled"
	case "charge.refunded":
		entryType = "refund"
		amountCents = -2900
		description = "Charge refunded"
	}

	period := time.Now().UTC().Format("2006-01")
	entryID := hashKey(fmt.Sprintf("ledger:%s:%s:%s", input.WorkspaceID, input.EventID, entryType))

	if a.pool != nil {
		_, err := a.pool.Exec(ctx, `
			INSERT INTO billing_ledger_entries (workspace_id, entry_type, amount_cents, description, reference_id, period, created_at)
			VALUES ($1::uuid, $2, $3, $4, $5, $6, now())`,
			input.WorkspaceID, entryType, amountCents, description, input.EventID, period)
		if err != nil {
			return nil, fmt.Errorf("update billing ledger: %w", err)
		}
	}

	evidence := hashKey(fmt.Sprintf("ledger-entry:%s:%s", entryID, entryType))

	return &BillingLedgerResult{
		EntryID:      entryID,
		AmountCents:  amountCents,
		EntryType:    entryType,
		EvidenceHash: evidence,
	}, nil
}

// EnforceBillingPolicyActivity enforces policy gates for billing events.
func (a *Activities) EnforceBillingPolicyActivity(ctx context.Context, input BillingPolicyInput) (*BillingPolicyResult, error) {
	var action string
	enforced := false

	switch input.EventType {
	case "customer.subscription.deleted":
		action = "downgrade"
		enforced = true
	case "invoice.payment_failed":
		action = "suspend"
		enforced = true
	}

	if a.pool != nil && enforced {
		// Persist policy enforcement record.
		_, _ = a.pool.Exec(ctx, `
			UPDATE billing_webhook_events
			SET status = 'processed', processed_at = now()
			WHERE workspace_id = $1::uuid AND event_type = $2 AND status = 'received'`,
			input.WorkspaceID, input.EventType)
	}

	evidence := hashKey(fmt.Sprintf("billing-policy:%s:%s:%s",
		input.WorkspaceID, input.EventType, action))

	return &BillingPolicyResult{
		Enforced:     enforced,
		Action:       action,
		EvidenceHash: evidence,
	}, nil
}

// ===================== LOAD SHEDDING ACTIVITY IMPLEMENTATIONS =====================

// EvaluateLoadSheddingTierActivity evaluates system metrics and determines the load shedding tier.
func (a *Activities) EvaluateLoadSheddingTierActivity(ctx context.Context, input LoadSheddingEvalInput) (*LoadSheddingEvalResult, error) {
	// Tier thresholds matching D0-D5 from control/load_shedding_controller.go.
	newTier := "D0"
	reason := "nominal"

	if input.CPUPercent > 95 || input.ErrorRate > 10 || input.DBPoolUsage > 90 {
		newTier = "D4"
		reason = fmt.Sprintf("critical: cpu=%.1f%% err=%.1f%% db=%.1f%%",
			input.CPUPercent, input.ErrorRate, input.DBPoolUsage)
	} else if input.CPUPercent > 90 || input.ErrorRate > 5 {
		newTier = "D3"
		reason = fmt.Sprintf("orange: cpu=%.1f%% err=%.1f%%", input.CPUPercent, input.ErrorRate)
	} else if input.CPUPercent > 85 || input.ErrorRate > 3 {
		newTier = "D2"
		reason = fmt.Sprintf("yellow: cpu=%.1f%% err=%.1f%%", input.CPUPercent, input.ErrorRate)
	} else if input.CPUPercent > 80 || input.ErrorRate > 2 {
		newTier = "D1"
		reason = fmt.Sprintf("elevated: cpu=%.1f%% err=%.1f%%", input.CPUPercent, input.ErrorRate)
	}

	// EU AI Act Art. 73: record incident when error rate exceeds 15%.
	if input.ErrorRate > 15 && a.euIncidentLog != nil {
		wsID, parseErr := uuid.Parse(input.WorkspaceID)
		if parseErr == nil {
			go func() {
				_, _ = a.euIncidentLog.RecordIncident(context.Background(), eu_ai_act.IncidentEntry{
					WorkspaceID:   wsID,
					IncidentType:  "high_error_rate",
					TriggerMetric: fmt.Sprintf("error_rate=%.2f%%", input.ErrorRate),
					Severity:      "high",
					Description:   fmt.Sprintf("Error rate %.1f%% exceeded 15%% threshold", input.ErrorRate),
				})
			}()
		}
	}

	// Get previous tier from DB.
	previousTier := "D0"
	if a.pool != nil {
		var prev string
		err := a.pool.QueryRow(ctx, `
			SELECT current_tier FROM load_shedding_state
			WHERE workspace_id = $1::uuid`,
			input.WorkspaceID).Scan(&prev)
		if err == nil {
			previousTier = prev
		}
	}

	changed := newTier != previousTier
	evidence := hashKey(fmt.Sprintf("ls:%s:%s:%s:%s",
		input.WorkspaceID, previousTier, newTier, reason))

	return &LoadSheddingEvalResult{
		NewTier:      newTier,
		PreviousTier: previousTier,
		Changed:      changed,
		Reason:       reason,
		EvidenceHash: evidence,
	}, nil
}

// PropagateLoadSheddingTierActivity persists and propagates a tier change.
func (a *Activities) PropagateLoadSheddingTierActivity(ctx context.Context, input LoadSheddingPropagateInput) (*LoadSheddingPropagateResult, error) {
	if a.pool != nil {
		_, err := a.pool.Exec(ctx, `
			INSERT INTO load_shedding_state (workspace_id, current_tier, reason, escalated_at, created_at)
			VALUES ($1::uuid, $2, $3, now(), now())
			ON CONFLICT (workspace_id) DO UPDATE
			SET current_tier = $2, reason = $3, escalated_at = now()`,
			input.WorkspaceID, input.NewTier, input.Reason)
		if err != nil {
			return &LoadSheddingPropagateResult{Propagated: false}, nil
		}
	}
	return &LoadSheddingPropagateResult{Propagated: true}, nil
}
