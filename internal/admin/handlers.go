package admin

import (
	"encoding/json"
	"net/http"
	"strconv"
)

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
