package control

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	contextlayer "github.com/brevio/brevio/internal/context"
	"github.com/brevio/brevio/internal/feature_flags"
	"github.com/brevio/brevio/internal/guardrails"
	raglayer "github.com/brevio/brevio/internal/rag"
	"github.com/brevio/brevio/internal/sessions"
	"github.com/brevio/brevio/internal/temporal_reasoning"
	"github.com/brevio/brevio/internal/tool_health"
)

func NewMux(service *Service) *http.ServeMux {
	mux := http.NewServeMux()
	flags := feature_flags.NewService()
	contextBudgets := contextlayer.NewService()
	guardrailsSvc := guardrails.NewService()
	ragSvc := raglayer.NewService()
	sessionSvc := sessions.NewService()
	temporalSvc := temporal_reasoning.NewService()
	toolHealthSvc := tool_health.NewService()

	mux.HandleFunc("GET /healthz/ready", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("GET /healthz/live", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			http.NotFound(w, r)
			return
		}

		if strings.HasPrefix(r.URL.Path, "/v1/flags") {
			handleFeatureFlags(w, r, flags)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/v1/context") {
			handleContextBudget(w, r, contextBudgets)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/v1/guardrails") {
			handleGuardrails(w, r, guardrailsSvc)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/v1/rag") {
			handleRAG(w, r, ragSvc)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/v1/tools") {
			handleToolHealth(w, r, toolHealthSvc)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/v1/sessions") {
			handleSessions(w, r, sessionSvc)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/v1/temporal") {
			handleTemporalReasoning(w, r, temporalSvc)
			return
		}

		status := http.StatusOK
		if r.Method == http.MethodPost {
			status = http.StatusAccepted
		}
		writeJSON(w, status, map[string]any{
			"status":  "accepted",
			"service": "control",
			"path":    r.URL.Path,
			"method":  r.Method,
		})
	})

	_ = service
	return mux
}

func handleContextBudget(w http.ResponseWriter, r *http.Request, svc *contextlayer.Service) {
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) != 3 {
		http.NotFound(w, r)
		return
	}

	switch parts[2] {
	case "budget":
		switch r.Method {
		case http.MethodGet:
			workspaceID := r.URL.Query().Get("workspace_id")
			if workspaceID == "" {
				workspaceID = "default"
			}
			budget, ok := svc.GetBudget(workspaceID)
			if !ok {
				budget = contextlayer.Budget{
					WorkspaceID:  workspaceID,
					BudgetTokens: 0,
					Status:       "active",
				}
			}
			writeJSON(w, http.StatusOK, budget)
			return
		case http.MethodPut:
			var payload struct {
				WorkspaceID  string         `json:"workspace_id"`
				BudgetTokens int            `json:"budget_tokens"`
				Status       string         `json:"status"`
				Allocations  map[string]int `json:"allocations"`
			}
			if err := decodeJSON(r, &payload); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			if payload.WorkspaceID == "" {
				payload.WorkspaceID = "default"
			}
			if payload.Status == "" {
				payload.Status = "active"
			}
			budget := svc.SetBudget(payload.WorkspaceID, payload.BudgetTokens, payload.Status)
			if len(payload.Allocations) > 0 {
				svc.SetAllocations(payload.WorkspaceID, payload.Allocations)
			}
			writeJSON(w, http.StatusOK, budget)
			return
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
	case "allocations":
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		workspaceID := r.URL.Query().Get("workspace_id")
		if workspaceID == "" {
			workspaceID = "default"
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"workspace_id": workspaceID,
			"allocations":  svc.GetAllocations(workspaceID),
		})
		return
	default:
		http.NotFound(w, r)
	}
}

func handleFeatureFlags(w http.ResponseWriter, r *http.Request, flags *feature_flags.Service) {
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	// /v1/flags
	if len(parts) == 2 {
		switch r.Method {
		case http.MethodGet:
			writeJSON(w, http.StatusOK, map[string]any{"flags": flags.ListFlags()})
			return
		case http.MethodPost:
			var payload feature_flags.Flag
			if err := decodeJSON(r, &payload); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			if payload.Key == "" {
				http.Error(w, "key is required", http.StatusBadRequest)
				return
			}
			flags.UpsertFlag(payload)
			writeJSON(w, http.StatusAccepted, map[string]any{"status": "accepted"})
			return
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
	}

	if len(parts) < 3 {
		http.NotFound(w, r)
		return
	}

	key := parts[2]
	// /v1/flags/{key}
	if len(parts) == 3 {
		switch r.Method {
		case http.MethodGet:
			flag, ok := flags.GetFlag(key)
			if !ok {
				writeJSON(w, http.StatusOK, map[string]any{
					"key":       key,
					"flag_type": "boolean",
					"enabled":   false,
					"reason":    "FLAG_NOT_FOUND",
				})
				return
			}
			writeJSON(w, http.StatusOK, flag)
			return
		case http.MethodPut:
			var payload feature_flags.Flag
			if err := decodeJSON(r, &payload); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			payload.Key = key
			flags.UpsertFlag(payload)
			writeJSON(w, http.StatusOK, map[string]any{"status": "updated"})
			return
		case http.MethodDelete:
			flags.DeleteFlag(key)
			writeJSON(w, http.StatusOK, map[string]any{"status": "deleted"})
			return
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
	}

	// /v1/flags/{key}/evaluate
	if len(parts) == 4 && parts[3] == "evaluate" {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var payload struct {
			Attributes map[string]string `json:"attributes"`
		}
		if err := decodeJSON(r, &payload); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		enabled, reason := flags.Evaluate(key, payload.Attributes)
		writeJSON(w, http.StatusOK, map[string]any{"enabled": enabled, "reason": reason})
		return
	}

	// /v1/flags/{key}/rules
	if len(parts) == 4 && parts[3] == "rules" {
		switch r.Method {
		case http.MethodGet:
			writeJSON(w, http.StatusOK, map[string]any{"rules": flags.GetRules(key)})
			return
		case http.MethodPost:
			var payload struct {
				Rules []feature_flags.Rule `json:"rules"`
			}
			if err := decodeJSON(r, &payload); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			flags.SetRules(key, payload.Rules)
			writeJSON(w, http.StatusAccepted, map[string]any{"status": "accepted"})
			return
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
	}

	http.NotFound(w, r)
}

func handleRAG(w http.ResponseWriter, r *http.Request, svc *raglayer.Service) {
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) < 3 {
		http.NotFound(w, r)
		return
	}

	switch parts[2] {
	case "collections":
		switch {
		case len(parts) == 3 && r.Method == http.MethodGet:
			workspaceID := r.URL.Query().Get("workspace_id")
			if workspaceID == "" {
				workspaceID = "default"
			}
			writeJSON(w, http.StatusOK, map[string]any{
				"collections": svc.ListCollections(workspaceID),
			})
			return

		case len(parts) == 3 && r.Method == http.MethodPost:
			var payload raglayer.Collection
			if err := decodeJSON(r, &payload); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			writeJSON(w, http.StatusCreated, svc.UpsertCollection(payload))
			return

		case len(parts) == 4 && r.Method == http.MethodGet:
			collectionID := parts[3]
			collection, ok := svc.GetCollection(collectionID)
			if !ok {
				writeJSON(w, http.StatusOK, raglayer.Collection{
					ID:          collectionID,
					WorkspaceID: "default",
					Status:      "not_found",
				})
				return
			}
			writeJSON(w, http.StatusOK, collection)
			return

		case len(parts) == 4 && r.Method == http.MethodPut:
			var payload raglayer.Collection
			if err := decodeJSON(r, &payload); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			payload.ID = parts[3]
			writeJSON(w, http.StatusOK, svc.UpsertCollection(payload))
			return

		case len(parts) == 4 && r.Method == http.MethodDelete:
			if svc.DeleteCollection(parts[3]) {
				writeJSON(w, http.StatusOK, map[string]any{"status": "deleted"})
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"status": "not_found"})
			return

		case len(parts) == 5 && parts[4] == "ingest" && r.Method == http.MethodPost:
			var payload struct {
				Documents []string `json:"documents"`
			}
			if err := decodeJSON(r, &payload); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			collection, ingested, ok := svc.Ingest(parts[3], payload.Documents)
			if !ok {
				writeJSON(w, http.StatusOK, map[string]any{
					"collection_id":   parts[3],
					"status":          "collection_not_found",
					"ingested_chunks": 0,
				})
				return
			}
			writeJSON(w, http.StatusAccepted, map[string]any{
				"collection":      collection,
				"ingested_chunks": ingested,
				"status":          "accepted",
			})
			return

		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

	case "search":
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var payload struct {
			WorkspaceID   string   `json:"workspace_id"`
			TurnID        string   `json:"turn_id"`
			QueryText     string   `json:"query_text"`
			CollectionIDs []string `json:"collection_ids"`
			MaxResults    int      `json:"max_results"`
		}
		if err := decodeJSON(r, &payload); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		retrieval := svc.Search(payload.WorkspaceID, payload.TurnID, payload.QueryText, payload.CollectionIDs, payload.MaxResults)
		writeJSON(w, http.StatusOK, retrieval)
		return

	case "retrievals":
		if len(parts) != 4 || r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		retrieval, ok := svc.GetRetrieval(parts[3])
		if !ok {
			writeJSON(w, http.StatusOK, raglayer.Retrieval{
				TurnID:  parts[3],
				Results: []raglayer.RetrievalResult{},
			})
			return
		}
		writeJSON(w, http.StatusOK, retrieval)
		return

	case "eval":
		if len(parts) != 4 || parts[3] != "scores" || r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		workspaceID := r.URL.Query().Get("workspace_id")
		if workspaceID == "" {
			workspaceID = "default"
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"scores": svc.ListEvalScores(workspaceID),
		})
		return
	default:
		http.NotFound(w, r)
		return
	}
}

func handleGuardrails(w http.ResponseWriter, r *http.Request, svc *guardrails.Service) {
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) < 3 {
		http.NotFound(w, r)
		return
	}

	switch parts[2] {
	case "config":
		switch r.Method {
		case http.MethodGet:
			workspaceID := r.URL.Query().Get("workspace_id")
			if workspaceID == "" {
				workspaceID = "default"
			}
			cfg, ok := svc.GetConfig(workspaceID)
			if !ok {
				cfg = svc.DefaultConfig(workspaceID)
			}
			writeJSON(w, http.StatusOK, cfg)
			return
		case http.MethodPut:
			var payload guardrails.Config
			if err := decodeJSON(r, &payload); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			workspaceID := payload.WorkspaceID
			if workspaceID == "" {
				workspaceID = "default"
			}
			cfg := svc.UpsertConfig(workspaceID, payload)
			svc.RecordEvent(workspaceID, "", "BREVIO.guardrail.config_updated.v1", "allow", "config_put")
			writeJSON(w, http.StatusOK, cfg)
			return
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

	case "rule-sets":
		switch {
		case len(parts) == 3 && r.Method == http.MethodGet:
			workspaceID := r.URL.Query().Get("workspace_id")
			if workspaceID == "" {
				workspaceID = "default"
			}
			writeJSON(w, http.StatusOK, map[string]any{
				"rule_sets": svc.ListRuleSets(workspaceID),
			})
			return

		case len(parts) == 3 && r.Method == http.MethodPost:
			var payload guardrails.RuleSet
			if err := decodeJSON(r, &payload); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			ruleSet := svc.UpsertRuleSet(payload)
			svc.RecordEvent(ruleSet.WorkspaceID, ruleSet.ID, "BREVIO.guardrail.rule_set_created.v1", "allow", strings.Join(ruleSet.Patterns, ","))
			writeJSON(w, http.StatusCreated, ruleSet)
			return

		case len(parts) == 4 && r.Method == http.MethodPut:
			var payload guardrails.RuleSet
			if err := decodeJSON(r, &payload); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			payload.ID = parts[3]
			ruleSet := svc.UpsertRuleSet(payload)
			svc.RecordEvent(ruleSet.WorkspaceID, ruleSet.ID, "BREVIO.guardrail.rule_set_updated.v1", "allow", strings.Join(ruleSet.Patterns, ","))
			writeJSON(w, http.StatusOK, ruleSet)
			return

		case len(parts) == 4 && r.Method == http.MethodDelete:
			if svc.DeleteRuleSet(parts[3]) {
				writeJSON(w, http.StatusOK, map[string]any{"status": "deleted"})
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"status": "not_found"})
			return

		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

	case "events":
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		workspaceID := r.URL.Query().Get("workspace_id")
		if workspaceID == "" {
			workspaceID = "default"
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"events": svc.ListEvents(workspaceID),
		})
		return

	default:
		http.NotFound(w, r)
		return
	}
}

func handleToolHealth(w http.ResponseWriter, r *http.Request, svc *tool_health.Service) {
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) < 3 {
		http.NotFound(w, r)
		return
	}
	workspaceID := r.URL.Query().Get("workspace_id")
	if workspaceID == "" {
		workspaceID = "default"
	}

	switch parts[2] {
	case "health":
		if len(parts) == 3 {
			if r.Method != http.MethodGet {
				http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{
				"scores": svc.ListScores(workspaceID),
			})
			return
		}
		if len(parts) == 4 {
			if r.Method != http.MethodGet {
				http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
				return
			}
			toolKey := parts[3]
			score, ok := svc.GetScore(workspaceID, toolKey)
			if !ok {
				score = svc.UpsertScore(tool_health.ToolScore{
					WorkspaceID:  workspaceID,
					ToolKey:      toolKey,
					Score:        1.0,
					FailureCount: 0,
				})
			}
			writeJSON(w, http.StatusOK, score)
			return
		}
		http.NotFound(w, r)
		return

	case "quarantine":
		if len(parts) == 4 && parts[3] == "rules" {
			switch r.Method {
			case http.MethodGet:
				writeJSON(w, http.StatusOK, map[string]any{
					"rules": svc.ListRules(workspaceID),
				})
				return
			case http.MethodPost:
				var payload tool_health.QuarantineRule
				if err := decodeJSON(r, &payload); err != nil {
					http.Error(w, err.Error(), http.StatusBadRequest)
					return
				}
				if payload.WorkspaceID == "" {
					payload.WorkspaceID = workspaceID
				}
				writeJSON(w, http.StatusCreated, svc.UpsertRule(payload))
				return
			default:
				http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
				return
			}
		}

		if len(parts) == 5 && parts[4] == "override" {
			if r.Method != http.MethodPost {
				http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
				return
			}
			toolKey := parts[3]
			var payload struct {
				WorkspaceID string `json:"workspace_id"`
				Status      string `json:"status"`
			}
			if err := decodeJSON(r, &payload); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			if payload.WorkspaceID == "" {
				payload.WorkspaceID = workspaceID
			}
			score := svc.ApplyOverride(payload.WorkspaceID, toolKey, payload.Status)
			writeJSON(w, http.StatusOK, score)
			return
		}

		http.NotFound(w, r)
		return

	default:
		http.NotFound(w, r)
		return
	}
}

func handleSessions(w http.ResponseWriter, r *http.Request, svc *sessions.Service) {
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) < 3 {
		http.NotFound(w, r)
		return
	}

	if len(parts) == 3 && parts[2] == "active" {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		workspaceID := r.URL.Query().Get("workspace_id")
		if workspaceID == "" {
			workspaceID = "default"
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"sessions": svc.ListActive(workspaceID),
		})
		return
	}

	sessionID := parts[2]
	if len(parts) == 3 {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		session, ok := svc.GetSession(sessionID)
		if !ok {
			workspaceID := r.URL.Query().Get("workspace_id")
			userID := r.URL.Query().Get("user_id")
			if workspaceID == "" {
				workspaceID = "default"
			}
			if userID == "" {
				userID = "unknown"
			}
			session = svc.EnsureSession(sessionID, workspaceID, userID)
		}
		writeJSON(w, http.StatusOK, session)
		return
	}

	if len(parts) == 4 && parts[3] == "entities" {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"session_id": sessionID,
			"entities":   svc.GetEntities(sessionID),
		})
		return
	}

	http.NotFound(w, r)
}

func handleTemporalReasoning(w http.ResponseWriter, r *http.Request, svc *temporal_reasoning.Service) {
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) < 3 {
		http.NotFound(w, r)
		return
	}

	switch parts[2] {
	case "config":
		switch r.Method {
		case http.MethodGet:
			workspaceID := r.URL.Query().Get("workspace_id")
			if workspaceID == "" {
				workspaceID = "default"
			}
			cfg, ok := svc.GetConfig(workspaceID)
			if !ok {
				cfg = svc.DefaultConfig(workspaceID)
			}
			writeJSON(w, http.StatusOK, cfg)
			return
		case http.MethodPut:
			var payload temporal_reasoning.Config
			if err := decodeJSON(r, &payload); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			workspaceID := payload.WorkspaceID
			if workspaceID == "" {
				workspaceID = "default"
			}
			writeJSON(w, http.StatusOK, svc.UpsertConfig(workspaceID, payload))
			return
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

	case "constraints":
		switch {
		case len(parts) == 3 && r.Method == http.MethodGet:
			workspaceID := r.URL.Query().Get("workspace_id")
			if workspaceID == "" {
				workspaceID = "default"
			}
			writeJSON(w, http.StatusOK, map[string]any{
				"workspace_id": workspaceID,
				"constraints":  svc.ListConstraints(workspaceID),
			})
			return

		case len(parts) == 3 && r.Method == http.MethodPost:
			var payload temporal_reasoning.Constraint
			if err := decodeJSON(r, &payload); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			workspaceID := payload.WorkspaceID
			if workspaceID == "" {
				workspaceID = "default"
			}
			constraint := svc.UpsertConstraint(workspaceID, payload)
			writeJSON(w, http.StatusCreated, constraint)
			return

		case len(parts) == 4 && r.Method == http.MethodPut:
			var payload temporal_reasoning.Constraint
			if err := decodeJSON(r, &payload); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			workspaceID := payload.WorkspaceID
			if workspaceID == "" {
				workspaceID = "default"
			}
			payload.ID = parts[3]
			constraint := svc.UpsertConstraint(workspaceID, payload)
			writeJSON(w, http.StatusOK, constraint)
			return

		case len(parts) == 4 && r.Method == http.MethodDelete:
			workspaceID := r.URL.Query().Get("workspace_id")
			if workspaceID == "" {
				workspaceID = "default"
			}
			if svc.DeleteConstraint(workspaceID, parts[3]) {
				writeJSON(w, http.StatusOK, map[string]any{"status": "deleted"})
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"status": "not_found"})
			return

		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

	case "resolve":
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var payload struct {
			WorkspaceID   string `json:"workspace_id"`
			Expression    string `json:"expression"`
			ReferenceDate string `json:"reference_date"`
			Timezone      string `json:"timezone"`
		}
		if err := decodeJSON(r, &payload); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		resolution := svc.ResolveExpression(payload.WorkspaceID, payload.Expression, payload.ReferenceDate, payload.Timezone)
		writeJSON(w, http.StatusOK, resolution)
		return

	case "conflicts":
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var payload struct {
			WorkspaceID   string `json:"workspace_id"`
			ProposedStart string `json:"proposed_start"`
			ProposedEnd   string `json:"proposed_end"`
		}
		if err := decodeJSON(r, &payload); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"conflicts": svc.DetectConflicts(payload.WorkspaceID, payload.ProposedStart, payload.ProposedEnd),
		})
		return

	case "travel-time":
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var payload struct {
			WorkspaceID string  `json:"workspace_id"`
			Origin      string  `json:"origin"`
			Destination string  `json:"destination"`
			DistanceKM  float64 `json:"distance_km"`
		}
		if err := decodeJSON(r, &payload); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"workspace_id": payload.WorkspaceID,
			"minutes":      svc.EstimateTravelMinutes(payload.WorkspaceID, payload.Origin, payload.Destination, payload.DistanceKM),
		})
		return

	default:
		http.NotFound(w, r)
		return
	}
}

func decodeJSON(r *http.Request, out any) error {
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(out); err != nil {
		return err
	}
	if decoder.More() {
		return errors.New("multiple json objects are not allowed")
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
