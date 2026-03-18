package admin

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// HarmBenchPool is an optional pgxpool.Pool used by the HarmBench admin endpoint.
// Set this before calling RegisterRoutes if a database is available.
var HarmBenchPool *pgxpool.Pool

// ProvenancePool is an optional pgxpool.Pool used by the provenance admin endpoint.
// Set this before calling RegisterRoutes if a database is available.
var ProvenancePool *pgxpool.Pool

// RegisterRoutes binds admin HTTP handlers to the provided ServeMux.
// Every handler validates that the caller has admin privileges via the
// X-User-Role header; non-admin callers receive HTTP 403.
func RegisterRoutes(mux *http.ServeMux, svc *Service) {
	// Operations
	mux.HandleFunc("GET /v1/admin/operations/dashboard", adminOnly(handleDashboard(svc)))
	mux.HandleFunc("GET /v1/admin/operations/workflows", adminOnly(handleWorkflows(svc)))
	mux.HandleFunc("GET /v1/admin/operations/queues", adminOnly(handleQueues(svc)))

	// Costs
	mux.HandleFunc("GET /v1/admin/costs/summary", adminOnly(handleCostSummary(svc)))
	mux.HandleFunc("GET /v1/admin/costs/anomalies", adminOnly(handleCostAnomalies(svc)))
	mux.HandleFunc("GET /v1/admin/costs/budgets", adminOnly(handleBudgets(svc)))

	// Security
	mux.HandleFunc("GET /v1/admin/security/audit-log", adminOnly(handleAuditLog(svc)))
	mux.HandleFunc("GET /v1/admin/security/trust-scores", adminOnly(handleTrustScores(svc)))
	mux.HandleFunc("GET /v1/admin/security/failed-auth", adminOnly(handleFailedAuth(svc)))

	// Config
	mux.HandleFunc("GET /v1/admin/config/system", adminOnly(handleSystemConfig(svc)))

	// Alerts
	mux.HandleFunc("GET /v1/admin/alerts/rules", adminOnly(handleListAlertRules(svc)))
	mux.HandleFunc("POST /v1/admin/alerts/rules", adminOnly(handleCreateAlertRule(svc)))
	mux.HandleFunc("GET /v1/admin/alerts/channels", adminOnly(handleListAlertChannels(svc)))
	mux.HandleFunc("GET /v1/admin/alerts/history", adminOnly(handleAlertHistory(svc)))

	// Users
	mux.HandleFunc("GET /v1/admin/users", adminOnly(handleListUsers(svc)))

	// MCP
	mux.HandleFunc("GET /v1/admin/mcp-servers", adminOnly(handleMCPServers(svc)))

	// Red-team HarmBench (P3-01).
	mux.HandleFunc("GET /v1/admin/security/harmbench/latest", adminOnly(handleLatestHarmBench(svc)))

	// Provenance forensic replay (P3-02).
	mux.HandleFunc("GET /v1/admin/provenance/{requestID}", adminOnly(handleProvenance(svc)))

	// Compliance (P3-05).
	mux.HandleFunc("POST /v1/admin/compliance/reports/soc2", adminOnly(handleSOC2Report(svc)))
	mux.HandleFunc("GET /v1/admin/compliance/evidence", adminOnly(handleComplianceEvidence(svc)))
	mux.HandleFunc("GET /v1/admin/compliance/summary", adminOnly(handleComplianceSummary(svc)))

	// MT-Bench & model promotions (P3-12).
	mux.HandleFunc("GET /v1/admin/benchmark/mt-bench/latest", adminOnly(handleMTBenchLatest(svc)))
	mux.HandleFunc("GET /v1/admin/models/promotions", adminOnly(handleModelPromotions(svc)))
	mux.HandleFunc("POST /v1/admin/models/promotions/{promotionID}/approve", adminOnly(handleModelPromotionApprove(svc)))

	// CAI principles (P3-13).
	mux.HandleFunc("GET /v1/admin/cai/principles", adminOnly(handleCAIPrinciples(svc)))
	mux.HandleFunc("GET /v1/admin/cai/principles/proposed", adminOnly(handleCAIProposed(svc)))
	mux.HandleFunc("POST /v1/admin/cai/principles/{principleID}/approve", adminOnly(handleCAIPrincipleApprove(svc)))
	mux.HandleFunc("POST /v1/admin/cai/principles/{principleID}/promote", adminOnly(handleCAIPrinciplePromote(svc)))

	// Federated rounds (P3-14).
	mux.HandleFunc("GET /v1/admin/federated/rounds", adminOnly(handleFederatedRounds(svc)))

	// Cost dashboard (P3-07).
	RegisterCostRoutes(mux, svc)

	// HIPAA (P3-06).
	mux.HandleFunc("GET /v1/admin/hipaa/access-log", adminOnly(handleHIPAAAccessLog(svc)))
	mux.HandleFunc("GET /v1/admin/hipaa/breaches", adminOnly(handleHIPAABreaches(svc)))
	mux.HandleFunc("POST /v1/admin/hipaa/breach/{breachID}/acknowledge", adminOnly(handleHIPAABreachAcknowledge(svc)))

	// Privacy / DP budget (P3-03).
	mux.HandleFunc("GET /v1/admin/privacy/budget/{workspaceID}", adminOnly(handlePrivacyBudget(svc)))
	mux.HandleFunc("POST /v1/admin/privacy/budget/{workspaceID}/reset", adminOnly(handlePrivacyBudgetReset(svc)))
	mux.HandleFunc("GET /v1/admin/privacy/membership-inference/{workspaceID}/latest", adminOnly(handleMembershipInferenceLatest(svc)))

	// V10.1: Kill switch
	killSwitchSvc := NewKillSwitchService()
	mux.HandleFunc("POST /v1/admin/kill-switch/activate", adminOnly(handleKillSwitchActivate(killSwitchSvc)))
	mux.HandleFunc("POST /v1/admin/kill-switch/deactivate", adminOnly(handleKillSwitchDeactivate(killSwitchSvc)))
	mux.HandleFunc("GET /v1/admin/kill-switch", adminOnly(handleKillSwitchList(killSwitchSvc)))

	// V10.1: Skill ACL
	skillACLSvc := NewSkillACLOverrideService()
	mux.HandleFunc("POST /v1/admin/skill-acl/override", adminOnly(handleSkillACLSet(skillACLSvc)))
	mux.HandleFunc("DELETE /v1/admin/skill-acl/override", adminOnly(handleSkillACLRemove(skillACLSvc)))
	mux.HandleFunc("GET /v1/admin/skill-acl/overrides", adminOnly(handleSkillACLList(skillACLSvc)))
}

// adminOnly wraps a handler and rejects requests without admin authorization.
// Checks session-based auth first (via AdminAuthMiddleware context), then falls
// back to X-User-Role header for backwards compatibility during migration.
func adminOnly(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Check session-based auth first (injected by AdminAuthMiddleware).
		if session := SessionFromContext(r.Context()); session != nil {
			if session.Role == "admin" || session.Role == "owner" {
				next(w, r)
				return
			}
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "admin role required"})
			return
		}

		// Fallback: X-User-Role header (legacy, will be removed).
		role := r.Header.Get("X-User-Role")
		if role != "admin" {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "admin role required"})
			return
		}
		next(w, r)
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// --- Operations ---

func handleDashboard(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, svc.Dashboard())
	}
}

func handleWorkflows(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, svc.Workflows())
	}
}

func handleQueues(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, svc.Queues())
	}
}

// --- Costs ---

func handleCostSummary(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, svc.CostSummary())
	}
}

func handleCostAnomalies(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, svc.CostAnomalies())
	}
}

func handleBudgets(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, svc.GetBudget())
	}
}

// --- Security ---

func handleAuditLog(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		page := queryInt(r, "page", 1)
		pageSize := queryInt(r, "page_size", 50)
		if pageSize > 200 {
			pageSize = 200
		}

		// Audit log is derived from alert events as a placeholder.
		events := svc.ListAlertEvents()
		total := len(events)
		start := (page - 1) * pageSize
		if start > total {
			start = total
		}
		end := start + pageSize
		if end > total {
			end = total
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"page":      page,
			"page_size": pageSize,
			"total":     total,
			"entries":   events[start:end],
		})
	}
}

func handleTrustScores(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, svc.RecalculateTrustScores())
	}
}

func handleFailedAuth(_ *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		// Placeholder: no persistent auth-failure store yet.
		writeJSON(w, http.StatusOK, map[string]any{
			"failed_attempts": []any{},
			"total":           0,
		})
	}
}

// --- Config ---

func handleSystemConfig(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		config := svc.GetDashboardConfig("default")
		budget := svc.GetBudget()
		writeJSON(w, http.StatusOK, map[string]any{
			"dashboard": config,
			"budget":    budget,
		})
	}
}

// --- Alerts ---

func handleListAlertRules(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, svc.ListAlertRules())
	}
}

func handleCreateAlertRule(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var rule AlertRule
		if err := json.NewDecoder(r.Body).Decode(&rule); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
			return
		}
		created := svc.UpsertAlertRule(rule)
		writeJSON(w, http.StatusCreated, created)
	}
}

func handleListAlertChannels(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, svc.ListAlertChannels())
	}
}

func handleAlertHistory(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, svc.ListAlertEvents())
	}
}

// --- Users ---

func handleListUsers(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		page := queryInt(r, "page", 1)
		pageSize := queryInt(r, "page_size", 50)
		if pageSize > 200 {
			pageSize = 200
		}

		users := svc.ListUsers()
		total := len(users)
		start := (page - 1) * pageSize
		if start > total {
			start = total
		}
		end := start + pageSize
		if end > total {
			end = total
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"page":      page,
			"page_size": pageSize,
			"total":     total,
			"users":     users[start:end],
		})
	}
}

// --- HarmBench (P3-01) ---

func handleLatestHarmBench(_ *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pool := HarmBenchPool
		if pool == nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "database not available"})
			return
		}

		// Find the most recent run_id.
		var runID string
		var runAt time.Time
		err := pool.QueryRow(r.Context(),
			`SELECT run_id, run_at FROM pg_security_scores ORDER BY run_at DESC LIMIT 1`,
		).Scan(&runID, &runAt)
		if err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "no harmbench results found"})
			return
		}

		// Fetch all category scores for that run.
		rows, err := pool.Query(r.Context(),
			`SELECT category, pass_rate FROM pg_security_scores WHERE run_id = $1`, runID,
		)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "query failed"})
			return
		}
		defer rows.Close()

		categoryScores := make(map[string]float64)
		overallPassRate := 0.0
		for rows.Next() {
			var category string
			var passRate float64
			if err := rows.Scan(&category, &passRate); err != nil {
				continue
			}
			if category == "overall" {
				overallPassRate = passRate
			} else {
				categoryScores[category] = passRate
			}
		}
		if err := rows.Err(); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "row iteration failed"})
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"run_id":            runID,
			"run_at":            runAt,
			"overall_pass_rate": overallPassRate,
			"category_scores":   categoryScores,
		})
	}
}

// --- CAI Principles (P3-13) & Federated Rounds (P3-14) ---

func handleCAIPrinciples(_ *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pool := CompliancePool
		if pool == nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "database not available"})
			return
		}

		rows, err := pool.Query(r.Context(),
			`SELECT id, principle_id, version, text, status, activated_at, created_at
			 FROM constitutional_principles
			 WHERE status IN ('active', 'testing')
			 ORDER BY principle_id`,
		)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "query failed"})
			return
		}
		defer rows.Close()

		type principle struct {
			ID          string     `json:"id"`
			PrincipleID string     `json:"principle_id"`
			Version     int        `json:"version"`
			Text        string     `json:"text"`
			Status      string     `json:"status"`
			ActivatedAt *time.Time `json:"activated_at,omitempty"`
			CreatedAt   time.Time  `json:"created_at"`
		}
		var principles []principle
		for rows.Next() {
			var p principle
			if err := rows.Scan(&p.ID, &p.PrincipleID, &p.Version, &p.Text, &p.Status, &p.ActivatedAt, &p.CreatedAt); err != nil {
				continue
			}
			principles = append(principles, p)
		}
		writeJSON(w, http.StatusOK, map[string]any{"principles": principles})
	}
}

func handleCAIProposed(_ *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pool := CompliancePool
		if pool == nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "database not available"})
			return
		}

		rows, err := pool.Query(r.Context(),
			`SELECT id, description, failure_examples, coverage_rate, status, proposed_at
			 FROM proposed_principles
			 WHERE status IN ('draft', 'admin_review')
			 ORDER BY proposed_at DESC`,
		)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "query failed"})
			return
		}
		defer rows.Close()

		type proposal struct {
			ID              string          `json:"id"`
			Description     string          `json:"description"`
			FailureExamples json.RawMessage `json:"failure_examples"`
			CoverageRate    float64         `json:"coverage_rate"`
			Status          string          `json:"status"`
			ProposedAt      time.Time       `json:"proposed_at"`
		}
		var proposals []proposal
		for rows.Next() {
			var p proposal
			if err := rows.Scan(&p.ID, &p.Description, &p.FailureExamples, &p.CoverageRate, &p.Status, &p.ProposedAt); err != nil {
				continue
			}
			proposals = append(proposals, p)
		}
		writeJSON(w, http.StatusOK, map[string]any{"proposals": proposals})
	}
}

func handleCAIPrincipleApprove(_ *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pool := CompliancePool
		if pool == nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "database not available"})
			return
		}

		principleID := r.PathValue("principleID")
		if principleID == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "principle_id required"})
			return
		}

		tag, err := pool.Exec(r.Context(),
			`UPDATE proposed_principles SET status='admin_review', approved_at=NOW()
			 WHERE id=$1 AND status='draft'`,
			principleID,
		)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "update failed"})
			return
		}
		if tag.RowsAffected() == 0 {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "proposal not found or not in draft status"})
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{"approved": true, "principle_id": principleID})
	}
}

func handleCAIPrinciplePromote(_ *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pool := CompliancePool
		if pool == nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "database not available"})
			return
		}

		principleID := r.PathValue("principleID")
		if principleID == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "principle_id required"})
			return
		}

		tag, err := pool.Exec(r.Context(),
			`UPDATE constitutional_principles SET status='active', activated_at=NOW()
			 WHERE principle_id=$1 AND status='testing'`,
			principleID,
		)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "update failed"})
			return
		}
		if tag.RowsAffected() == 0 {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "principle not found or not in testing status"})
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{"promoted": true, "principle_id": principleID})
	}
}

func handleFederatedRounds(_ *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pool := CompliancePool
		if pool == nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "database not available"})
			return
		}

		rows, err := pool.Query(r.Context(),
			`SELECT id, participating_count, gradient_dimensions, max_epsilon_used, run_at
			 FROM federated_rounds
			 ORDER BY run_at DESC LIMIT 10`,
		)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "query failed"})
			return
		}
		defer rows.Close()

		type round struct {
			ID                  string    `json:"id"`
			ParticipatingCount  int       `json:"participating_count"`
			GradientDimensions  int       `json:"gradient_dimensions"`
			MaxEpsilonUsed      float64   `json:"max_epsilon_used"`
			RunAt               time.Time `json:"run_at"`
		}
		var rounds []round
		for rows.Next() {
			var rd round
			if err := rows.Scan(&rd.ID, &rd.ParticipatingCount, &rd.GradientDimensions, &rd.MaxEpsilonUsed, &rd.RunAt); err != nil {
				continue
			}
			rounds = append(rounds, rd)
		}
		writeJSON(w, http.StatusOK, map[string]any{"rounds": rounds})
	}
}

// --- MT-Bench & Model Promotions (P3-12) ---

// EvalPool is an optional pgxpool.Pool used by evaluation admin endpoints.
var EvalPool *pgxpool.Pool

func handleMTBenchLatest(_ *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pool := EvalPool
		if pool == nil {
			pool = CompliancePool
		}
		if pool == nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "database not available"})
			return
		}

		var runID, model, triggeredBy string
		var runAt time.Time
		var overallScore float64
		err := pool.QueryRow(r.Context(),
			`SELECT id, run_at, overall_score, model, triggered_by FROM mt_bench_runs ORDER BY run_at DESC LIMIT 1`,
		).Scan(&runID, &runAt, &overallScore, &model, &triggeredBy)
		if err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "no MT-Bench runs found"})
			return
		}

		rows, qErr := pool.Query(r.Context(),
			`SELECT category, avg_score FROM mt_bench_scores WHERE run_id = $1`, runID,
		)
		categoryScores := map[string]float64{}
		if qErr == nil {
			defer rows.Close()
			for rows.Next() {
				var cat string
				var score float64
				if scanErr := rows.Scan(&cat, &score); scanErr == nil {
					categoryScores[cat] = score
				}
			}
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"run_id":          runID,
			"run_at":          runAt,
			"model":           model,
			"overall_score":   overallScore,
			"category_scores": categoryScores,
		})
	}
}

func handleModelPromotions(_ *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pool := EvalPool
		if pool == nil {
			pool = CompliancePool
		}
		if pool == nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "database not available"})
			return
		}

		rows, err := pool.Query(r.Context(),
			`SELECT id, challenger_model, metrics, status, requested_at
			 FROM model_promotion_requests
			 ORDER BY requested_at DESC LIMIT 50`,
		)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "query failed"})
			return
		}
		defer rows.Close()

		type promotion struct {
			ID              string          `json:"id"`
			ChallengerModel string          `json:"challenger_model"`
			Metrics         json.RawMessage `json:"metrics"`
			Status          string          `json:"status"`
			RequestedAt     time.Time       `json:"requested_at"`
		}
		var promotions []promotion
		for rows.Next() {
			var p promotion
			if scanErr := rows.Scan(&p.ID, &p.ChallengerModel, &p.Metrics, &p.Status, &p.RequestedAt); scanErr == nil {
				promotions = append(promotions, p)
			}
		}

		writeJSON(w, http.StatusOK, map[string]any{"promotions": promotions})
	}
}

func handleModelPromotionApprove(_ *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pool := EvalPool
		if pool == nil {
			pool = CompliancePool
		}
		if pool == nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "database not available"})
			return
		}

		promotionID := r.PathValue("promotionID")
		if promotionID == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "promotion_id required"})
			return
		}

		tag, err := pool.Exec(r.Context(),
			`UPDATE model_promotion_requests SET status='approved', reviewed_at=NOW()
			 WHERE id=$1 AND status='pending'`,
			promotionID,
		)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "update failed"})
			return
		}
		if tag.RowsAffected() == 0 {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "promotion not found or already reviewed"})
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"approved":     true,
			"promotion_id": promotionID,
		})
	}
}

// --- HIPAA (P3-06) ---

// HIPAAPool is an optional pgxpool.Pool used by the HIPAA admin endpoints.
var HIPAAPool *pgxpool.Pool

func handleHIPAAAccessLog(_ *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pool := HIPAAPool
		if pool == nil {
			pool = CompliancePool
		}
		if pool == nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "database not available"})
			return
		}

		workspaceID := r.URL.Query().Get("workspace_id")
		userID := r.URL.Query().Get("user_id")
		limit := queryInt(r, "limit", 100)
		if limit > 500 {
			limit = 500
		}

		query := `SELECT id, user_id, workspace_id, phi_category, data_accessed, purpose, accessed_at
		          FROM hipaa_access_log WHERE 1=1`
		args := []any{}
		argIdx := 1

		if workspaceID != "" {
			query += fmt.Sprintf(" AND workspace_id = $%d", argIdx)
			args = append(args, workspaceID)
			argIdx++
		}
		if userID != "" {
			query += fmt.Sprintf(" AND user_id = $%d", argIdx)
			args = append(args, userID)
			argIdx++
		}
		query += fmt.Sprintf(" ORDER BY accessed_at DESC LIMIT $%d", argIdx)
		args = append(args, limit)

		rows, err := pool.Query(r.Context(), query, args...)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "query failed"})
			return
		}
		defer rows.Close()

		type logEntry struct {
			ID           string    `json:"id"`
			UserID       string    `json:"user_id"`
			WorkspaceID  string    `json:"workspace_id"`
			PHICategory  string    `json:"phi_category"`
			DataAccessed string    `json:"data_accessed"`
			Purpose      string    `json:"purpose"`
			AccessedAt   time.Time `json:"accessed_at"`
		}
		var entries []logEntry
		for rows.Next() {
			var e logEntry
			if err := rows.Scan(&e.ID, &e.UserID, &e.WorkspaceID, &e.PHICategory,
				&e.DataAccessed, &e.Purpose, &e.AccessedAt); err != nil {
				continue
			}
			entries = append(entries, e)
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"entries": entries,
			"count":   len(entries),
		})
	}
}

func handleHIPAABreaches(_ *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pool := HIPAAPool
		if pool == nil {
			pool = CompliancePool
		}
		if pool == nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "database not available"})
			return
		}

		workspaceID := r.URL.Query().Get("workspace_id")
		query := `SELECT id, workspace_id, user_id, phi_category, breach_type, detected_at,
		                 acknowledged_at, notified_at, details, created_at
		          FROM hipaa_breach_log`
		args := []any{}

		if workspaceID != "" {
			query += " WHERE workspace_id = $1"
			args = append(args, workspaceID)
		}
		query += " ORDER BY detected_at DESC LIMIT 100"

		rows, err := pool.Query(r.Context(), query, args...)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "query failed"})
			return
		}
		defer rows.Close()

		type breachEntry struct {
			ID             string     `json:"id"`
			WorkspaceID    string     `json:"workspace_id"`
			UserID         *string    `json:"user_id"`
			PHICategory    string     `json:"phi_category"`
			BreachType     string     `json:"breach_type"`
			DetectedAt     time.Time  `json:"detected_at"`
			AcknowledgedAt *time.Time `json:"acknowledged_at"`
			NotifiedAt     *time.Time `json:"notified_at"`
			Details        *string    `json:"details"`
			CreatedAt      time.Time  `json:"created_at"`
		}
		var entries []breachEntry
		for rows.Next() {
			var e breachEntry
			if err := rows.Scan(&e.ID, &e.WorkspaceID, &e.UserID, &e.PHICategory,
				&e.BreachType, &e.DetectedAt, &e.AcknowledgedAt, &e.NotifiedAt,
				&e.Details, &e.CreatedAt); err != nil {
				continue
			}
			entries = append(entries, e)
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"breaches": entries,
			"count":    len(entries),
		})
	}
}

func handleHIPAABreachAcknowledge(_ *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pool := HIPAAPool
		if pool == nil {
			pool = CompliancePool
		}
		if pool == nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "database not available"})
			return
		}

		breachID := r.PathValue("breachID")
		if breachID == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "breach_id is required"})
			return
		}

		tag, err := pool.Exec(r.Context(),
			`UPDATE hipaa_breach_log SET acknowledged_at = NOW()
			 WHERE id = $1 AND acknowledged_at IS NULL`,
			breachID,
		)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "update failed"})
			return
		}
		if tag.RowsAffected() == 0 {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "breach not found or already acknowledged"})
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"acknowledged": true,
			"breach_id":    breachID,
		})
	}
}

// --- Compliance (P3-05) ---

// CompliancePool is an optional pgxpool.Pool used by compliance admin endpoints.
var CompliancePool *pgxpool.Pool

func handleSOC2Report(_ *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pool := CompliancePool
		if pool == nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "database not available"})
			return
		}

		var req struct {
			StartDate string `json:"start_date"`
			EndDate   string `json:"end_date"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
			return
		}

		startDate, err := time.Parse("2006-01-02", req.StartDate)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid start_date format (expected YYYY-MM-DD)"})
			return
		}
		endDate, err := time.Parse("2006-01-02", req.EndDate)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid end_date format (expected YYYY-MM-DD)"})
			return
		}

		// Import the report generator inline to avoid import cycle.
		// Query evidence and build a simple PDF response.
		rows, qErr := pool.Query(r.Context(),
			`SELECT control_id, framework, evidence_type, collected_at, pass, details
			 FROM compliance_evidence
			 WHERE collected_at >= $1 AND collected_at <= $2
			 ORDER BY framework, control_id, collected_at DESC`,
			startDate, endDate.Add(24*time.Hour),
		)
		if qErr != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "query failed"})
			return
		}
		defer rows.Close()

		type evRow struct {
			ControlID   string    `json:"control_id"`
			Framework   string    `json:"framework"`
			EvidenceType string   `json:"evidence_type"`
			CollectedAt time.Time `json:"collected_at"`
			Pass        bool      `json:"pass"`
		}
		var evidence []evRow
		for rows.Next() {
			var ev evRow
			var details json.RawMessage
			if err := rows.Scan(&ev.ControlID, &ev.Framework, &ev.EvidenceType, &ev.CollectedAt, &ev.Pass, &details); err != nil {
				continue
			}
			evidence = append(evidence, ev)
		}

		// Return JSON report (PDF generation available via soc2.SOC2ReportGenerator).
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"report_type": "soc2_type_ii",
			"start_date":  req.StartDate,
			"end_date":    req.EndDate,
			"generated_at": time.Now().UTC(),
			"evidence_count": len(evidence),
			"evidence":    evidence,
		})
	}
}

func handleComplianceEvidence(_ *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pool := CompliancePool
		if pool == nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "database not available"})
			return
		}

		framework := r.URL.Query().Get("framework")
		controlID := r.URL.Query().Get("control_id")
		limit := queryInt(r, "limit", 30)
		if limit > 100 {
			limit = 100
		}

		query := `SELECT control_id, framework, evidence_type, collected_at, pass, details
		          FROM compliance_evidence WHERE 1=1`
		args := []any{}
		argIdx := 1

		if framework != "" {
			query += fmt.Sprintf(" AND framework = $%d", argIdx)
			args = append(args, framework)
			argIdx++
		}
		if controlID != "" {
			query += fmt.Sprintf(" AND control_id = $%d", argIdx)
			args = append(args, controlID)
			argIdx++
		}
		query += fmt.Sprintf(" ORDER BY collected_at DESC LIMIT $%d", argIdx)
		args = append(args, limit)

		rows, err := pool.Query(r.Context(), query, args...)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "query failed"})
			return
		}
		defer rows.Close()

		type evRecord struct {
			ControlID    string          `json:"control_id"`
			Framework    string          `json:"framework"`
			EvidenceType string          `json:"evidence_type"`
			CollectedAt  time.Time       `json:"collected_at"`
			Pass         bool            `json:"pass"`
			Details      json.RawMessage `json:"details"`
		}
		var records []evRecord
		for rows.Next() {
			var ev evRecord
			if err := rows.Scan(&ev.ControlID, &ev.Framework, &ev.EvidenceType,
				&ev.CollectedAt, &ev.Pass, &ev.Details); err != nil {
				continue
			}
			records = append(records, ev)
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"evidence": records,
			"count":    len(records),
		})
	}
}

func handleComplianceSummary(_ *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pool := CompliancePool
		if pool == nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "database not available"})
			return
		}

		// Get the latest run timestamp.
		var lastRunAt time.Time
		_ = pool.QueryRow(r.Context(),
			`SELECT MAX(collected_at) FROM compliance_evidence`,
		).Scan(&lastRunAt)

		// Compute pass rates per framework from the most recent collection.
		type frameworkStats struct {
			PassRate       float64 `json:"pass_rate"`
			Passed         int     `json:"passed"`
			Failed         int     `json:"failed"`
		}

		stats := map[string]*frameworkStats{}
		rows, err := pool.Query(r.Context(),
			`SELECT DISTINCT ON (framework, control_id) framework, control_id, pass
			 FROM compliance_evidence
			 ORDER BY framework, control_id, collected_at DESC`,
		)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var fw, cid string
				var pass bool
				if scanErr := rows.Scan(&fw, &cid, &pass); scanErr != nil {
					continue
				}
				if stats[fw] == nil {
					stats[fw] = &frameworkStats{}
				}
				if pass {
					stats[fw].Passed++
				} else {
					stats[fw].Failed++
				}
			}
		}

		for _, s := range stats {
			total := s.Passed + s.Failed
			if total > 0 {
				s.PassRate = float64(s.Passed) / float64(total)
			}
		}

		totalPassed := 0
		totalFailed := 0
		for _, s := range stats {
			totalPassed += s.Passed
			totalFailed += s.Failed
		}

		soc2Rate := 0.0
		if s, ok := stats["soc2"]; ok {
			soc2Rate = s.PassRate
		}
		isoRate := 0.0
		if s, ok := stats["iso27001"]; ok {
			isoRate = s.PassRate
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"soc2_pass_rate":     soc2Rate,
			"iso27001_pass_rate": isoRate,
			"last_run_at":        lastRunAt,
			"controls_passed":    totalPassed,
			"controls_failed":    totalFailed,
		})
	}
}

// --- Privacy / DP Budget (P3-03) ---

// DPBudgetPool is an optional pgxpool.Pool used by the DP budget admin endpoints.
var DPBudgetPool *pgxpool.Pool

func handlePrivacyBudget(_ *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pool := DPBudgetPool
		if pool == nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "database not available"})
			return
		}

		workspaceID := r.PathValue("workspaceID")
		if workspaceID == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "workspace_id is required"})
			return
		}

		var rec struct {
			WorkspaceID       string    `json:"workspace_id"`
			CumulativeEpsilon float64   `json:"cumulative_epsilon"`
			EpsilonMax        float64   `json:"epsilon_max"`
			DeltaTarget       float64   `json:"delta_target"`
			RoundsCompleted   int       `json:"rounds_completed"`
			Halted            bool      `json:"halted"`
			LastUpdatedAt     time.Time `json:"last_updated_at"`
		}

		err := pool.QueryRow(r.Context(),
			`SELECT workspace_id, cumulative_epsilon, epsilon_max, delta_target,
			        rounds_completed, halted, last_updated_at
			 FROM workspace_dp_budgets WHERE workspace_id = $1`,
			workspaceID,
		).Scan(&rec.WorkspaceID, &rec.CumulativeEpsilon, &rec.EpsilonMax, &rec.DeltaTarget,
			&rec.RoundsCompleted, &rec.Halted, &rec.LastUpdatedAt)
		if err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "privacy budget not found"})
			return
		}

		writeJSON(w, http.StatusOK, rec)
	}
}

func handlePrivacyBudgetReset(_ *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pool := DPBudgetPool
		if pool == nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "database not available"})
			return
		}

		workspaceID := r.PathValue("workspaceID")
		if workspaceID == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "workspace_id is required"})
			return
		}

		_, err := pool.Exec(r.Context(),
			`UPDATE workspace_dp_budgets
			 SET cumulative_epsilon = 0, halted = false, rounds_completed = 0, last_updated_at = NOW()
			 WHERE workspace_id = $1`,
			workspaceID,
		)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "reset failed"})
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"reset":        true,
			"workspace_id": workspaceID,
		})
	}
}

func handleMembershipInferenceLatest(_ *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pool := DPBudgetPool
		if pool == nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "database not available"})
			return
		}

		workspaceID := r.PathValue("workspaceID")
		if workspaceID == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "workspace_id is required"})
			return
		}

		var rec struct {
			WorkspaceID string    `json:"workspace_id"`
			AUC         float64   `json:"auc"`
			AlertFired  bool      `json:"alert_fired"`
			RunAt       time.Time `json:"run_at"`
		}

		err := pool.QueryRow(r.Context(),
			`SELECT workspace_id, auc, alert_fired, run_at
			 FROM membership_inference_results
			 WHERE workspace_id = $1
			 ORDER BY run_at DESC LIMIT 1`,
			workspaceID,
		).Scan(&rec.WorkspaceID, &rec.AUC, &rec.AlertFired, &rec.RunAt)
		if err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "no membership inference results found"})
			return
		}

		writeJSON(w, http.StatusOK, rec)
	}
}

// --- Provenance (P3-02) ---

func handleProvenance(_ *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pool := ProvenancePool
		if pool == nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "database not available"})
			return
		}

		requestID := r.PathValue("requestID")
		if requestID == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "request_id is required"})
			return
		}

		var rec struct {
			RequestID     string    `json:"request_id"`
			WorkspaceID   string    `json:"workspace_id"`
			ModelID       string    `json:"model_id"`
			Timestamp     time.Time `json:"timestamp"`
			WatermarkHash string    `json:"watermark_hash"`
			ContentHash   string    `json:"content_hash"`
		}

		err := pool.QueryRow(r.Context(),
			`SELECT request_id, workspace_id, model_id, timestamp, watermark_hash, content_hash
			 FROM ai_content_provenance WHERE request_id = $1`,
			requestID,
		).Scan(&rec.RequestID, &rec.WorkspaceID, &rec.ModelID, &rec.Timestamp, &rec.WatermarkHash, &rec.ContentHash)
		if err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "provenance record not found"})
			return
		}

		writeJSON(w, http.StatusOK, rec)
	}
}

// --- MCP ---

func handleMCPServers(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, svc.ListMCPServerHealth())
	}
}

// --- Kill Switch (V10.1) ---

func handleKillSwitchActivate(svc *KillSwitchService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			WorkspaceID string `json:"workspace_id"`
			UserID      string `json:"user_id,omitempty"`
			ActivatedBy string `json:"activated_by"`
			Reason      string `json:"reason"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
			return
		}
		ks, err := svc.Activate(req.WorkspaceID, req.UserID, req.ActivatedBy, req.Reason)
		if err != nil {
			writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, ks)
	}
}

func handleKillSwitchDeactivate(svc *KillSwitchService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			WorkspaceID string `json:"workspace_id"`
			UserID      string `json:"user_id,omitempty"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
			return
		}
		if err := svc.Deactivate(req.WorkspaceID, req.UserID); err != nil {
			writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "deactivated"})
	}
}

func handleKillSwitchList(svc *KillSwitchService) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"kill_switches": svc.GetAll()})
	}
}

// --- Skill ACL (V10.1) ---

func handleSkillACLSet(svc *SkillACLOverrideService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var override SkillACLOverride
		if err := json.NewDecoder(r.Body).Decode(&override); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
			return
		}
		created, err := svc.SetOverride(override)
		if err != nil {
			writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusCreated, created)
	}
}

func handleSkillACLRemove(svc *SkillACLOverrideService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		workspaceID := r.URL.Query().Get("workspace_id")
		userID := r.URL.Query().Get("user_id")
		skillID := r.URL.Query().Get("skill_id")
		if err := svc.RemoveOverride(workspaceID, userID, skillID); err != nil {
			writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "removed"})
	}
}

func handleSkillACLList(svc *SkillACLOverrideService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		workspaceID := r.URL.Query().Get("workspace_id")
		userID := r.URL.Query().Get("user_id")
		overrides := svc.GetUserOverrides(workspaceID, userID)
		writeJSON(w, http.StatusOK, map[string]any{"overrides": overrides})
	}
}

// queryInt extracts an integer query parameter with a default fallback.
func queryInt(r *http.Request, key string, defaultVal int) int {
	raw := r.URL.Query().Get(key)
	if raw == "" {
		return defaultVal
	}
	v, err := strconv.Atoi(raw)
	if err != nil || v < 1 {
		return defaultVal
	}
	return v
}
