package control

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/brevio/brevio/internal/admin"
	"github.com/brevio/brevio/internal/caching"
	"github.com/brevio/brevio/internal/capture"
	"github.com/brevio/brevio/internal/codebase_intel"
	"github.com/brevio/brevio/internal/compliance"
	contextlayer "github.com/brevio/brevio/internal/context"
	errorlayer "github.com/brevio/brevio/internal/errors"
	"github.com/brevio/brevio/internal/event_schemas"
	"github.com/brevio/brevio/internal/exploration"
	"github.com/brevio/brevio/internal/feature_flags"
	"github.com/brevio/brevio/internal/goals"
	"github.com/brevio/brevio/internal/guardrails"
	"github.com/brevio/brevio/internal/learning"
	"github.com/brevio/brevio/internal/model_tiers"
	raglayer "github.com/brevio/brevio/internal/rag"
	"github.com/brevio/brevio/internal/self_modification"
	"github.com/brevio/brevio/internal/sessions"
	"github.com/brevio/brevio/internal/streaming"
	"github.com/brevio/brevio/internal/temporal_reasoning"
	"github.com/brevio/brevio/internal/tool_health"
	"github.com/brevio/brevio/internal/trust"
)

func NewMux(service *Service) *http.ServeMux {
	mux := http.NewServeMux()
	adminSvc := admin.NewService()
	cacheSvc := caching.NewService()
	captureSvc := capture.NewService()
	codebaseSvc := codebase_intel.NewService()
	complianceSvc := compliance.NewService()
	eventSchemaSvc := event_schemas.NewService()
	explorationSvc := exploration.NewService()
	flags := feature_flags.NewService()
	goalsSvc := goals.NewService()
	errorSvc := errorlayer.NewService()
	contextBudgets := contextlayer.NewService()
	guardrailsSvc := guardrails.NewService()
	learningSvc := learning.NewService()
	modelTierSvc := model_tiers.NewService()
	ragSvc := raglayer.NewService()
	selfModificationSvc := self_modification.NewService()
	sessionSvc := sessions.NewService()
	streamingSvc := streaming.NewService()
	temporalSvc := temporal_reasoning.NewService()
	toolHealthSvc := tool_health.NewService()
	trustSvc := trust.NewService()

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
		if strings.HasPrefix(r.URL.Path, "/v1/goals") {
			handleGoals(w, r, goalsSvc)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/v1/mission-control") {
			handleMissionControl(w, r, goalsSvc)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/v1/admin") {
			handleAdmin(w, r, adminSvc)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/v1/autonomy") {
			handleTrust(w, r, trustSvc)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/v1/capabilities") {
			handleExploration(w, r, explorationSvc)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/v1/captures") {
			handleCaptures(w, r, captureSvc)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/v1/compliance") {
			handleCompliance(w, r, complianceSvc)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/v1/codebase") {
			handleCodebaseIntel(w, r, codebaseSvc)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/v1/errors") {
			handleErrors(w, r, errorSvc)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/v1/event-schemas") {
			handleEventSchemas(w, r, eventSchemaSvc)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/v1/cache") {
			handleCaching(w, r, cacheSvc)
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
			handleRAG(w, r, ragSvc, guardrailsSvc)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/v1/tools") {
			handleToolHealth(w, r, toolHealthSvc)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/v1/model-tiers") {
			handleModelTiers(w, r, modelTierSvc)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/v1/learning") {
			handleLearning(w, r, learningSvc)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/v1/self-modification") {
			handleSelfModification(w, r, selfModificationSvc)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/v1/sessions") {
			handleSessions(w, r, sessionSvc)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/v1/streaming") {
			handleStreaming(w, r, streamingSvc)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/v1/temporal") {
			handleTemporalReasoning(w, r, temporalSvc)
			return
		}

		http.NotFound(w, r)
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
					WorkspaceID:            workspaceID,
					Tier:                   "T2",
					MaxContextTokens:       0,
					ReservedResponseTokens: 256,
					MaxRAGTokens:           0,
					BudgetTokens:           0,
					Status:                 "active",
				}
			}
			writeJSON(w, http.StatusOK, budget)
			return
		case http.MethodPut:
			var payload struct {
				WorkspaceID            string         `json:"workspace_id"`
				Tier                   string         `json:"tier"`
				MaxContextTokens       int            `json:"max_context_tokens"`
				ReservedResponseTokens int            `json:"reserved_response_tokens"`
				MaxRAGTokens           int            `json:"max_rag_tokens"`
				BudgetTokens           int            `json:"budget_tokens"`
				Status                 string         `json:"status"`
				IngressTurnID          string         `json:"ingress_turn_id"`
				PromptRequestedTokens  int            `json:"prompt_requested_tokens"`
				RAGRequestedTokens     int            `json:"rag_requested_tokens"`
				HistoryRequestedTokens int            `json:"history_requested_tokens"`
				Allocations            map[string]int `json:"allocations"`
			}
			if err := decodeJSON(r, &payload); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			if payload.WorkspaceID == "" {
				payload.WorkspaceID = "default"
			}
			if payload.MaxContextTokens == 0 {
				payload.MaxContextTokens = payload.BudgetTokens
			}
			if payload.Status == "" {
				payload.Status = "active"
			}
			budget := svc.UpsertBudgetConfig(
				payload.WorkspaceID,
				payload.Tier,
				payload.MaxContextTokens,
				payload.ReservedResponseTokens,
				payload.MaxRAGTokens,
				payload.Status,
			)
			if len(payload.Allocations) > 0 {
				svc.SetAllocations(payload.WorkspaceID, payload.Allocations)
			}
			if payload.PromptRequestedTokens > 0 || payload.RAGRequestedTokens > 0 || payload.HistoryRequestedTokens > 0 {
				if _, err := svc.AllocateContext(payload.WorkspaceID, payload.IngressTurnID, payload.PromptRequestedTokens, payload.RAGRequestedTokens, payload.HistoryRequestedTokens); err != nil {
					http.Error(w, err.Error(), http.StatusTooManyRequests)
					return
				}
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
		report, ok := svc.GetAllocationReport(workspaceID)
		if !ok {
			report = contextlayer.AllocationReport{
				IngressTurnID:          "context_unset",
				TotalBudgetTokens:      0,
				AllocatedPromptTokens:  0,
				AllocatedRAGTokens:     0,
				AllocatedHistoryTokens: 0,
				Overflowed:             false,
			}
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"workspace_id":             workspaceID,
			"allocations":              svc.GetAllocations(workspaceID),
			"ingress_turn_id":          report.IngressTurnID,
			"total_budget_tokens":      report.TotalBudgetTokens,
			"allocated_prompt_tokens":  report.AllocatedPromptTokens,
			"allocated_rag_tokens":     report.AllocatedRAGTokens,
			"allocated_history_tokens": report.AllocatedHistoryTokens,
			"overflowed":               report.Overflowed,
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
			WorkspaceID string            `json:"workspace_id"`
			Attributes  map[string]string `json:"attributes"`
		}
		if err := decodeJSON(r, &payload); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		evaluation := flags.EvaluateForWorkspace(key, payload.WorkspaceID, payload.Attributes)
		writeJSON(w, http.StatusOK, evaluation)
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

func handleErrors(w http.ResponseWriter, r *http.Request, svc *errorlayer.Service) {
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) != 3 {
		http.NotFound(w, r)
		return
	}

	switch parts[2] {
	case "taxonomy":
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"errors": svc.ListTaxonomy(),
		})
		return

	case "templates":
		switch r.Method {
		case http.MethodGet:
			workspaceID := r.URL.Query().Get("workspace_id")
			if workspaceID == "" {
				workspaceID = "default"
			}
			persona := r.URL.Query().Get("persona")
			if persona == "" {
				persona = "default"
			}
			errorCode := r.URL.Query().Get("error_code")
			if errorCode != "" {
				writeJSON(w, http.StatusOK, svc.RenderMessage(workspaceID, persona, errorCode, r.URL.Query().Get("detail")))
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{
				"templates": svc.ListTemplates(workspaceID),
			})
			return
		case http.MethodPost:
			var payload struct {
				ID          string `json:"id"`
				WorkspaceID string `json:"workspace_id"`
				Persona     string `json:"persona"`
				CodePattern string `json:"code_pattern"`
				Template    string `json:"template"`
				Status      string `json:"status"`

				ErrorCode   string `json:"error_code"`
				UserMessage string `json:"user_message"`
				Retryable   *bool  `json:"retryable"`
				NextAction  string `json:"next_action"`
			}
			if err := decodeJSON(r, &payload); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}

			if payload.Template != "" || payload.CodePattern != "" {
				writeJSON(w, http.StatusCreated, svc.UpsertTemplate(errorlayer.Template{
					ID:          payload.ID,
					WorkspaceID: payload.WorkspaceID,
					Persona:     payload.Persona,
					CodePattern: payload.CodePattern,
					Template:    payload.Template,
					Status:      payload.Status,
				}))
				return
			}

			if payload.ErrorCode == "" || payload.UserMessage == "" {
				http.Error(w, "template or error_message payload required", http.StatusBadRequest)
				return
			}
			workspaceID := payload.WorkspaceID
			if workspaceID == "" {
				workspaceID = "default"
			}
			persona := payload.Persona
			if persona == "" {
				persona = "default"
			}
			svc.UpsertTemplate(errorlayer.Template{
				WorkspaceID: workspaceID,
				Persona:     persona,
				CodePattern: payload.ErrorCode,
				Template:    payload.UserMessage,
				Status:      "active",
			})

			message := svc.RenderMessage(workspaceID, persona, payload.ErrorCode, "")
			if payload.Retryable != nil {
				message.Retryable = *payload.Retryable
			}
			if payload.NextAction != "" {
				message.NextAction = payload.NextAction
			}
			writeJSON(w, http.StatusCreated, message)
			return
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

	default:
		http.NotFound(w, r)
		return
	}
}

func handleCaching(w http.ResponseWriter, r *http.Request, svc *caching.Service) {
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) != 3 {
		http.NotFound(w, r)
		return
	}

	switch parts[2] {
	case "policies":
		switch r.Method {
		case http.MethodGet:
			workspaceID := r.URL.Query().Get("workspace_id")
			if workspaceID == "" {
				workspaceID = "default"
			}
			writeJSON(w, http.StatusOK, map[string]any{
				"policies": svc.ListPolicies(workspaceID),
			})
			return
		case http.MethodPost:
			var payload caching.Policy
			if err := decodeJSON(r, &payload); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			policy := svc.UpsertPolicy(payload)
			svc.PutEntry(policy.WorkspaceID, policy.CacheKey, "seed")
			writeJSON(w, http.StatusCreated, policy)
			return
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

	case "stats":
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		workspaceID := r.URL.Query().Get("workspace_id")
		if workspaceID == "" {
			workspaceID = "default"
		}
		writeJSON(w, http.StatusOK, svc.Stats(workspaceID))
		return

	case "invalidate":
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var payload struct {
			WorkspaceID string `json:"workspace_id"`
			CacheKey    string `json:"cache_key"`
		}
		if err := decodeJSON(r, &payload); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if payload.WorkspaceID == "" {
			payload.WorkspaceID = "default"
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"invalidated": svc.Invalidate(payload.WorkspaceID, payload.CacheKey),
		})
		return

	default:
		http.NotFound(w, r)
		return
	}
}

func handleEventSchemas(w http.ResponseWriter, r *http.Request, svc *event_schemas.Service) {
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) < 2 {
		http.NotFound(w, r)
		return
	}

	if len(parts) == 2 {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"types": svc.ListTypes(),
		})
		return
	}

	if len(parts) < 4 {
		http.NotFound(w, r)
		return
	}
	eventType := parts[2]
	if len(parts) == 4 && parts[3] == "versions" {
		switch r.Method {
		case http.MethodGet:
			writeJSON(w, http.StatusOK, map[string]any{
				"versions": svc.ListVersions(eventType),
			})
			return
		case http.MethodPost:
			var payload struct {
				Schema string `json:"schema"`
				Status string `json:"status"`
			}
			if err := decodeJSON(r, &payload); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			version, err := svc.RegisterVersionStrict(eventType, payload.Schema, payload.Status)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			writeJSON(w, http.StatusCreated, version)
			return
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
	}

	if len(parts) == 4 && parts[3] == "validate" {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var payload struct {
			Event map[string]any `json:"event"`
		}
		if err := decodeJSON(r, &payload); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		writeJSON(w, http.StatusOK, svc.Validate(eventType, payload.Event))
		return
	}

	http.NotFound(w, r)
}

func handleModelTiers(w http.ResponseWriter, r *http.Request, svc *model_tiers.Service) {
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) != 3 {
		http.NotFound(w, r)
		return
	}
	workspaceID := r.URL.Query().Get("workspace_id")
	if workspaceID == "" {
		workspaceID = "default"
	}

	switch parts[2] {
	case "policies":
		switch r.Method {
		case http.MethodGet:
			writeJSON(w, http.StatusOK, map[string]any{
				"policies": svc.ListPolicies(workspaceID),
			})
			return
		case http.MethodPost:
			var payload model_tiers.Policy
			if err := decodeJSON(r, &payload); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			if payload.WorkspaceID == "" {
				payload.WorkspaceID = workspaceID
			}
			writeJSON(w, http.StatusCreated, svc.UpsertPolicy(payload))
			return
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
	case "overrides":
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if requestedTier := strings.TrimSpace(r.URL.Query().Get("requested_tier")); requestedTier != "" {
			complexityScore := 0
			if rawComplexity := strings.TrimSpace(r.URL.Query().Get("complexity_score")); rawComplexity != "" {
				parsedComplexity, err := strconv.Atoi(rawComplexity)
				if err != nil {
					http.Error(w, "invalid complexity_score", http.StatusBadRequest)
					return
				}
				complexityScore = parsedComplexity
			}
			decision := svc.EnforceTier(workspaceID, requestedTier, complexityScore)
			writeJSON(w, http.StatusOK, map[string]any{
				"overrides": svc.ListOverrides(workspaceID),
				"decision":  decision,
			})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"overrides": svc.ListOverrides(workspaceID),
		})
		return
	default:
		http.NotFound(w, r)
		return
	}
}

func handleRAG(w http.ResponseWriter, r *http.Request, svc *raglayer.Service, guardrailsSvc *guardrails.Service) {
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
					ID:           collectionID,
					CollectionID: collectionID,
					WorkspaceID:  "default",
					Status:       "not_found",
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
			WorkspaceID       string   `json:"workspace_id"`
			TurnID            string   `json:"turn_id"`
			QueryText         string   `json:"query_text"`
			Query             string   `json:"query"`
			CollectionID      string   `json:"collection_id"`
			CollectionIDs     []string `json:"collection_ids"`
			MaxResults        int      `json:"max_results"`
			TopK              int      `json:"top_k"`
			IncludeProvenance bool     `json:"include_provenance"`
		}
		if err := decodeJSON(r, &payload); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		queryText := payload.QueryText
		if strings.TrimSpace(queryText) == "" {
			queryText = payload.Query
		}
		collectionIDs := payload.CollectionIDs
		if len(collectionIDs) == 0 && strings.TrimSpace(payload.CollectionID) != "" {
			collectionIDs = []string{payload.CollectionID}
		}
		maxResults := payload.MaxResults
		if maxResults == 0 {
			maxResults = payload.TopK
		}
		decision := guardrailsSvc.EvaluateInput(payload.WorkspaceID, queryText)
		if decision.Blocked {
			http.Error(w, decision.Reason, http.StatusForbidden)
			return
		}
		queryText = decision.RedactedText
		retrieval := svc.Search(payload.WorkspaceID, payload.TurnID, queryText, collectionIDs, maxResults)
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
				RetrievalID: parts[3],
				TurnID:      parts[3],
				Results:     []raglayer.RetrievalResult{},
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
			"scores":           svc.ListEvalScores(workspaceID),
			"retrieval_scores": svc.ListRetrievalEvalScores(workspaceID),
			"reranker_config":  svc.GetRerankerConfig(workspaceID),
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
		activeSessions := svc.ListActive(workspaceID)
		sessionContexts := make([]map[string]any, 0, len(activeSessions))
		for _, activeSession := range activeSessions {
			context, ok := svc.SessionContext(activeSession.ID)
			if !ok {
				continue
			}
			sessionContexts = append(sessionContexts, context)
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"sessions": sessionContexts,
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
		if intent := strings.TrimSpace(r.URL.Query().Get("intent")); intent != "" {
			session = svc.UpsertIntent(sessionID, intent, 0.8)
			_ = session
		}
		context, ok := svc.SessionContext(sessionID)
		if ok {
			writeJSON(w, http.StatusOK, context)
			return
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
			ReferenceTS   string `json:"reference_ts"`
			Timezone      string `json:"timezone"`
		}
		if err := decodeJSON(r, &payload); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if strings.TrimSpace(payload.ReferenceDate) == "" && strings.TrimSpace(payload.ReferenceTS) != "" {
			if parsed, err := time.Parse(time.RFC3339, payload.ReferenceTS); err == nil {
				payload.ReferenceDate = parsed.UTC().Format("2006-01-02")
			}
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
		writeJSON(w, http.StatusOK, svc.BuildConflictReport(payload.WorkspaceID, payload.ProposedStart, payload.ProposedEnd))
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

func handleGoals(w http.ResponseWriter, r *http.Request, svc *goals.Service) {
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) < 2 {
		http.NotFound(w, r)
		return
	}

	switch {
	case len(parts) == 2:
		switch r.Method {
		case http.MethodGet:
			workspaceID := r.URL.Query().Get("workspace_id")
			if workspaceID == "" {
				workspaceID = "default"
			}
			writeJSON(w, http.StatusOK, map[string]any{"goals": svc.ListGoals(workspaceID)})
			return
		case http.MethodPost:
			var payload goals.Goal
			if err := decodeJSON(r, &payload); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			goal, err := svc.CreateGoal(payload, time.Now().UTC())
			if err != nil {
				http.Error(w, err.Error(), http.StatusTooManyRequests)
				return
			}
			svc.AddProgress(goal.ID, goals.ProgressLog{Summary: "goal created"})
			writeJSON(w, http.StatusCreated, goal)
			return
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

	case len(parts) == 3:
		goalID := parts[2]
		switch r.Method {
		case http.MethodGet:
			goal, ok := svc.GetGoal(goalID)
			if !ok {
				writeJSON(w, http.StatusOK, map[string]any{"id": goalID, "status": "not_found"})
				return
			}
			writeJSON(w, http.StatusOK, goal)
			return
		case http.MethodPut:
			var payload goals.Goal
			if err := decodeJSON(r, &payload); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			payload.ID = goalID
			goal := svc.UpsertGoal(payload)
			svc.AddProgress(goal.ID, goals.ProgressLog{Summary: "goal updated"})
			writeJSON(w, http.StatusOK, goal)
			return
		case http.MethodDelete:
			writeJSON(w, http.StatusOK, map[string]any{"deleted": svc.DeleteGoal(goalID)})
			return
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

	case len(parts) == 4 && parts[3] == "milestones":
		goalID := parts[2]
		switch r.Method {
		case http.MethodGet:
			writeJSON(w, http.StatusOK, map[string]any{"milestones": svc.ListMilestones(goalID)})
			return
		case http.MethodPost:
			var payload goals.Milestone
			if err := decodeJSON(r, &payload); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			milestone := svc.AddMilestone(goalID, payload)
			svc.AddProgress(goalID, goals.ProgressLog{Summary: "milestone added"})
			writeJSON(w, http.StatusCreated, milestone)
			return
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

	case len(parts) == 4 && parts[3] == "progress":
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"progress": svc.ListProgress(parts[2])})
		return

	default:
		http.NotFound(w, r)
		return
	}
}

func handleMissionControl(w http.ResponseWriter, r *http.Request, svc *goals.Service) {
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) != 3 {
		http.NotFound(w, r)
		return
	}
	workspaceID := r.URL.Query().Get("workspace_id")
	if workspaceID == "" {
		workspaceID = "default"
	}

	switch parts[2] {
	case "config":
		if r.Method == http.MethodGet {
			cfg, ok := svc.GetMissionControlConfig(workspaceID)
			if !ok {
				cfg = goals.MissionControlConfig{WorkspaceID: workspaceID, RefreshCadenceMinutes: 30}
			}
			writeJSON(w, http.StatusOK, cfg)
			return
		}
		if r.Method == http.MethodPut {
			var payload goals.MissionControlConfig
			if err := decodeJSON(r, &payload); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			writeJSON(w, http.StatusOK, svc.UpsertMissionControlConfig(workspaceID, payload))
			return
		}
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return

	case "widgets":
		if r.Method == http.MethodGet {
			writeJSON(w, http.StatusOK, map[string]any{"widgets": svc.GetMissionControlWidgets(workspaceID)})
			return
		}
		if r.Method == http.MethodPut {
			var payload struct {
				Widgets []goals.MissionControlWidget `json:"widgets"`
			}
			if err := decodeJSON(r, &payload); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"widgets": svc.SetMissionControlWidgets(workspaceID, payload.Widgets)})
			return
		}
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return

	case "snapshot":
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		writeJSON(w, http.StatusOK, svc.MissionControlSnapshot(workspaceID))
		return
	default:
		http.NotFound(w, r)
		return
	}
}

func handleTrust(w http.ResponseWriter, r *http.Request, svc *trust.Service) {
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) < 3 {
		http.NotFound(w, r)
		return
	}

	switch parts[2] {
	case "trust-scores":
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if len(svc.ListScores()) == 0 {
			svc.RecalculateScore("default", 25, 0, 0, 0, "A1")
		}
		writeJSON(w, http.StatusOK, map[string]any{"trust_scores": svc.ListScores()})
		return
	case "promotions":
		switch {
		case len(parts) == 3 && r.Method == http.MethodGet:
			if len(svc.ListPromotions()) == 0 {
				svc.RecalculateScore("default", 25, 0, 0, 0, "A1")
			}
			writeJSON(w, http.StatusOK, map[string]any{"promotions": svc.ListPromotions()})
			return
		case len(parts) == 5 && parts[4] == "decide" && r.Method == http.MethodPost:
			var payload struct {
				Decision string `json:"decision"`
			}
			if err := decodeJSON(r, &payload); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			updated, ok := svc.DecidePromotion(parts[3], payload.Decision)
			if !ok {
				writeJSON(w, http.StatusOK, map[string]any{"id": parts[3], "status": "not_found"})
				return
			}
			writeJSON(w, http.StatusOK, updated)
			return
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
	default:
		http.NotFound(w, r)
		return
	}
}

func handleLearning(w http.ResponseWriter, r *http.Request, svc *learning.Service) {
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
	case "config":
		if r.Method == http.MethodGet {
			cfg, ok := svc.GetConfig(workspaceID)
			if !ok {
				cfg = learning.Config{WorkspaceID: workspaceID, MaxActiveLessons: 20}
			}
			writeJSON(w, http.StatusOK, cfg)
			return
		}
		if r.Method == http.MethodPut {
			var payload learning.Config
			if err := decodeJSON(r, &payload); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			writeJSON(w, http.StatusOK, svc.UpsertConfig(workspaceID, payload))
			return
		}
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return

	case "feedback":
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var payload learning.Feedback
		if err := decodeJSON(r, &payload); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		payload.WorkspaceID = workspaceID
		feedback, err := svc.SubmitFeedback(payload)
		if err != nil {
			http.Error(w, err.Error(), http.StatusTooManyRequests)
			return
		}
		writeJSON(w, http.StatusCreated, feedback)
		return

	case "lessons":
		switch {
		case len(parts) == 3 && r.Method == http.MethodGet:
			writeJSON(w, http.StatusOK, map[string]any{"lessons": svc.ListLessons(workspaceID)})
			return
		case len(parts) == 5 && parts[4] == "confirm" && r.Method == http.MethodPost:
			lesson, ok := svc.ConfirmLesson(parts[3])
			if !ok {
				writeJSON(w, http.StatusOK, map[string]any{"id": parts[3], "status": "not_found"})
				return
			}
			writeJSON(w, http.StatusOK, lesson)
			return
		case len(parts) == 5 && parts[4] == "retire" && r.Method == http.MethodPost:
			lesson, ok := svc.RetireLesson(parts[3])
			if !ok {
				writeJSON(w, http.StatusOK, map[string]any{"id": parts[3], "status": "not_found"})
				return
			}
			writeJSON(w, http.StatusOK, lesson)
			return
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

	default:
		http.NotFound(w, r)
		return
	}
}

func handleCaptures(w http.ResponseWriter, r *http.Request, svc *capture.Service) {
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) < 3 || parts[2] != "daily" {
		http.NotFound(w, r)
		return
	}
	workspaceID := r.URL.Query().Get("workspace_id")
	if workspaceID == "" {
		workspaceID = "default"
	}

	if len(parts) == 3 && r.Method == http.MethodGet {
		writeJSON(w, http.StatusOK, map[string]any{"captures": svc.List(workspaceID)})
		return
	}
	if len(parts) == 4 && r.Method == http.MethodGet {
		entry := svc.CompleteDailyCapture(workspaceID, parts[3])
		writeJSON(w, http.StatusOK, entry)
		return
	}
	http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
}

func handleCodebaseIntel(w http.ResponseWriter, r *http.Request, svc *codebase_intel.Service) {
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
	case "dependencies":
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if r.URL.Query().Get("recompute") == "true" {
			svc.AnalyzeCrossRepo(workspaceID)
		}
		report := svc.GetCrossRepoReport(workspaceID)
		writeJSON(w, http.StatusOK, map[string]any{
			"dependencies":        svc.ListDependencies(workspaceID),
			"shared_dependencies": report.SharedDependencies,
		})
		return

	case "patterns":
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if r.URL.Query().Get("recompute") == "true" {
			svc.AnalyzeCrossRepo(workspaceID)
		}
		report := svc.GetCrossRepoReport(workspaceID)
		writeJSON(w, http.StatusOK, map[string]any{
			"patterns":        svc.ListPatterns(workspaceID),
			"shared_patterns": report.SharedPatterns,
		})
		return

	case "debt":
		switch {
		case len(parts) == 3 && r.Method == http.MethodGet:
			writeJSON(w, http.StatusOK, map[string]any{"debt_items": svc.ListDebt(workspaceID)})
			return
		case len(parts) == 4 && r.Method == http.MethodPut:
			var payload codebase_intel.DebtItem
			if err := decodeJSON(r, &payload); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			payload.WorkspaceID = workspaceID
			writeJSON(w, http.StatusOK, svc.UpsertDebt(parts[3], payload))
			return
		case len(parts) == 5 && parts[4] == "tasks" && r.Method == http.MethodGet:
			writeJSON(w, http.StatusOK, map[string]any{"tasks": svc.ListDebtTasks(parts[3])})
			return
		case len(parts) == 5 && parts[4] == "tasks" && r.Method == http.MethodPost:
			var payload codebase_intel.DebtTask
			if err := decodeJSON(r, &payload); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			payload.WorkspaceID = workspaceID
			writeJSON(w, http.StatusCreated, svc.AddDebtTask(parts[3], payload))
			return
		case len(parts) == 6 && parts[4] == "tasks" && r.Method == http.MethodGet:
			task, ok := svc.GetDebtTask(parts[3], parts[5])
			if !ok {
				writeJSON(w, http.StatusOK, map[string]any{"id": parts[5], "status": "not_found"})
				return
			}
			writeJSON(w, http.StatusOK, task)
			return
		case len(parts) == 6 && parts[4] == "tasks" && r.Method == http.MethodPut:
			var payload codebase_intel.DebtTask
			if err := decodeJSON(r, &payload); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			payload.WorkspaceID = workspaceID
			writeJSON(w, http.StatusOK, svc.UpsertDebtTask(parts[3], parts[5], payload))
			return
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

	case "templates":
		switch r.Method {
		case http.MethodGet:
			writeJSON(w, http.StatusOK, map[string]any{"templates": svc.ListTemplates(workspaceID)})
			return
		case http.MethodPost:
			var payload codebase_intel.ProjectTemplate
			if err := decodeJSON(r, &payload); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			payload.WorkspaceID = workspaceID
			writeJSON(w, http.StatusCreated, svc.AddTemplate(payload))
			return
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

	case "context-export":
		switch {
		case len(parts) == 3 && r.Method == http.MethodPost:
			var payload codebase_intel.ContextExport
			if err := decodeJSON(r, &payload); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			payload.WorkspaceID = workspaceID
			created, err := svc.CreateContextExportStrict(payload)
			if err != nil {
				status := http.StatusBadRequest
				if err.Error() == "EXPORT_RATE_LIMIT" {
					status = http.StatusTooManyRequests
				}
				http.Error(w, err.Error(), status)
				return
			}
			writeJSON(w, http.StatusCreated, created)
			return
		case len(parts) == 4 && r.Method == http.MethodGet:
			export, ok := svc.GetContextExport(parts[3])
			if !ok {
				writeJSON(w, http.StatusOK, map[string]any{"id": parts[3], "status": "not_found"})
				return
			}
			writeJSON(w, http.StatusOK, export)
			return
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

	default:
		http.NotFound(w, r)
		return
	}
}

func handleExploration(w http.ResponseWriter, r *http.Request, svc *exploration.Service) {
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) < 3 || parts[2] != "recommendations" {
		http.NotFound(w, r)
		return
	}
	workspaceID := r.URL.Query().Get("workspace_id")
	if workspaceID == "" {
		workspaceID = "default"
	}

	if len(parts) == 3 && r.Method == http.MethodGet {
		writeJSON(w, http.StatusOK, map[string]any{"recommendations": svc.ListRecommendations(workspaceID)})
		return
	}
	if len(parts) == 5 && parts[4] == "decide" && r.Method == http.MethodPost {
		var payload struct {
			Decision string `json:"decision"`
		}
		if err := decodeJSON(r, &payload); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		updated, ok, err := svc.DecideRecommendation(parts[3], payload.Decision)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if !ok {
			writeJSON(w, http.StatusOK, map[string]any{"id": parts[3], "status": "not_found"})
			return
		}
		writeJSON(w, http.StatusOK, updated)
		return
	}
	http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
}

func handleSelfModification(w http.ResponseWriter, r *http.Request, svc *self_modification.Service) {
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) != 3 || parts[2] != "policy" {
		http.NotFound(w, r)
		return
	}
	workspaceID := r.URL.Query().Get("workspace_id")
	if workspaceID == "" {
		workspaceID = "default"
	}
	if r.Method == http.MethodGet {
		policy, ok := svc.GetPolicy(workspaceID)
		if !ok {
			policy = self_modification.Policy{
				WorkspaceID:     workspaceID,
				Enabled:         true,
				RequireApproval: true,
				MaxAllowedRisk:  "elevated",
			}
		}
		writeJSON(w, http.StatusOK, policy)
		return
	}
	if r.Method == http.MethodPut {
		var payload self_modification.Policy
		if err := decodeJSON(r, &payload); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		stored, err := svc.UpsertPolicyStrict(workspaceID, payload)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		writeJSON(w, http.StatusOK, stored)
		return
	}
	http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
}

func handleStreaming(w http.ResponseWriter, r *http.Request, svc *streaming.Service) {
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) != 3 || parts[2] != "config" {
		http.NotFound(w, r)
		return
	}

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
		var payload streaming.Config
		if err := decodeJSON(r, &payload); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if payload.FirstByteSLAMillis > 500 {
			http.Error(w, "first_byte_sla_ms must be <= 500", http.StatusBadRequest)
			return
		}
		if payload.FirstByteSLAMillis < 0 {
			http.Error(w, "first_byte_sla_ms must be >= 0", http.StatusBadRequest)
			return
		}
		if payload.ChunkSizeBytes < 0 {
			http.Error(w, "chunk_size_bytes must be >= 0", http.StatusBadRequest)
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
}

func handleCompliance(w http.ResponseWriter, r *http.Request, svc *compliance.Service) {
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) < 3 {
		http.NotFound(w, r)
		return
	}

	switch parts[2] {
	case "frameworks":
		switch r.Method {
		case http.MethodGet:
			workspaceID := r.URL.Query().Get("workspace_id")
			if workspaceID == "" {
				workspaceID = "default"
			}
			writeJSON(w, http.StatusOK, map[string]any{
				"frameworks": svc.ListFrameworks(workspaceID),
			})
			return
		case http.MethodPost:
			var payload compliance.Framework
			if err := decodeJSON(r, &payload); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			framework := svc.UpsertFramework(payload)
			svc.AddEvidence(compliance.Evidence{
				WorkspaceID: framework.WorkspaceID,
				FrameworkID: framework.ID,
				EventType:   "BREVIO.compliance.framework.created.v1",
				ArtifactURI: "s3://breviosboms/framework.json",
			})
			writeJSON(w, http.StatusCreated, framework)
			return
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

	case "evidence":
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		workspaceID := r.URL.Query().Get("workspace_id")
		if workspaceID == "" {
			workspaceID = "default"
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"evidence": svc.ListEvidence(workspaceID),
		})
		return

	case "dsr":
		switch {
		case len(parts) == 3 && r.Method == http.MethodGet:
			workspaceID := r.URL.Query().Get("workspace_id")
			if workspaceID == "" {
				workspaceID = "default"
			}
			writeJSON(w, http.StatusOK, map[string]any{
				"dsr_requests": svc.ListDSR(workspaceID),
				"sla_at_risk":  svc.ListDSRAtRisk(workspaceID),
			})
			return
		case len(parts) == 3 && r.Method == http.MethodPost:
			var payload compliance.DSRRequest
			if err := decodeJSON(r, &payload); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			writeJSON(w, http.StatusCreated, svc.CreateDSR(payload))
			return
		case len(parts) == 4 && r.Method == http.MethodGet:
			request, ok := svc.GetDSR(parts[3])
			if !ok {
				writeJSON(w, http.StatusOK, map[string]any{
					"id":     parts[3],
					"status": "not_found",
				})
				return
			}
			writeJSON(w, http.StatusOK, request)
			return
		case len(parts) == 4 && r.Method == http.MethodPut:
			var payload compliance.DSRRequest
			if err := decodeJSON(r, &payload); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			request, ok := svc.UpdateDSR(parts[3], payload)
			if !ok {
				writeJSON(w, http.StatusOK, map[string]any{
					"id":     parts[3],
					"status": "not_found",
				})
				return
			}
			writeJSON(w, http.StatusOK, request)
			return
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
	default:
		http.NotFound(w, r)
		return
	}
}

func handleAdmin(w http.ResponseWriter, r *http.Request, svc *admin.Service) {
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) < 3 {
		http.NotFound(w, r)
		return
	}

	switch parts[2] {
	case "trust-scores":
		if len(parts) == 4 && parts[3] == "recalculate" && r.Method == http.MethodPost {
			writeJSON(w, http.StatusAccepted, svc.RecalculateTrustScores())
			return
		}
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return

	case "learning":
		if len(parts) == 5 && parts[3] == "lessons" && parts[4] == "bulk-retire" && r.Method == http.MethodPost {
			writeJSON(w, http.StatusAccepted, svc.BulkRetireLessons())
			return
		}
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return

	case "users":
		switch {
		case len(parts) == 3 && r.Method == http.MethodGet:
			writeJSON(w, http.StatusOK, map[string]any{"users": svc.ListUsers()})
			return
		case len(parts) == 4 && r.Method == http.MethodGet:
			user, ok := svc.GetUser(parts[3])
			if !ok {
				writeJSON(w, http.StatusOK, map[string]any{"id": parts[3], "status": "not_found"})
				return
			}
			writeJSON(w, http.StatusOK, user)
			return
		case len(parts) == 4 && r.Method == http.MethodPut:
			var payload admin.User
			if err := decodeJSON(r, &payload); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			payload.ID = parts[3]
			writeJSON(w, http.StatusOK, svc.UpsertUser(payload))
			return
		case len(parts) == 5 && parts[4] == "sessions" && r.Method == http.MethodGet:
			writeJSON(w, http.StatusOK, map[string]any{"sessions": svc.ListUserSessions(parts[3])})
			return
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

	case "operations":
		if len(parts) != 4 || r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		switch parts[3] {
		case "dashboard":
			writeJSON(w, http.StatusOK, svc.Dashboard())
			return
		case "workflows":
			writeJSON(w, http.StatusOK, map[string]any{"workflows": svc.Workflows()})
			return
		case "queues":
			writeJSON(w, http.StatusOK, map[string]any{"queues": svc.Queues()})
			return
		default:
			http.NotFound(w, r)
			return
		}

	case "costs":
		if len(parts) != 4 {
			http.NotFound(w, r)
			return
		}
		switch parts[3] {
		case "summary":
			if r.Method != http.MethodGet {
				http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
				return
			}
			writeJSON(w, http.StatusOK, svc.CostSummary())
			return
		case "anomalies":
			if r.Method != http.MethodGet {
				http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"anomalies": svc.CostAnomalies()})
			return
		case "budgets":
			if r.Method == http.MethodGet {
				writeJSON(w, http.StatusOK, svc.GetBudget())
				return
			}
			if r.Method == http.MethodPut {
				var payload admin.CostBudget
				if err := decodeJSON(r, &payload); err != nil {
					http.Error(w, err.Error(), http.StatusBadRequest)
					return
				}
				writeJSON(w, http.StatusOK, svc.SetBudget(payload))
				return
			}
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		default:
			http.NotFound(w, r)
			return
		}

	case "alerts":
		if len(parts) < 4 {
			http.NotFound(w, r)
			return
		}
		switch parts[3] {
		case "rules":
			if len(parts) == 4 {
				switch r.Method {
				case http.MethodGet:
					writeJSON(w, http.StatusOK, map[string]any{"rules": svc.ListAlertRules()})
					return
				case http.MethodPost:
					var payload admin.AlertRule
					if err := decodeJSON(r, &payload); err != nil {
						http.Error(w, err.Error(), http.StatusBadRequest)
						return
					}
					writeJSON(w, http.StatusCreated, svc.UpsertAlertRule(payload))
					return
				default:
					http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
					return
				}
			}
			if len(parts) == 5 {
				switch r.Method {
				case http.MethodPut:
					var payload admin.AlertRule
					if err := decodeJSON(r, &payload); err != nil {
						http.Error(w, err.Error(), http.StatusBadRequest)
						return
					}
					payload.ID = parts[4]
					writeJSON(w, http.StatusOK, svc.UpsertAlertRule(payload))
					return
				case http.MethodDelete:
					writeJSON(w, http.StatusOK, map[string]any{"deleted": svc.DeleteAlertRule(parts[4])})
					return
				default:
					http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
					return
				}
			}
			http.NotFound(w, r)
			return

		case "channels":
			if len(parts) != 4 {
				http.NotFound(w, r)
				return
			}
			switch r.Method {
			case http.MethodGet:
				writeJSON(w, http.StatusOK, map[string]any{"channels": svc.ListAlertChannels()})
				return
			case http.MethodPost:
				var payload admin.AlertChannel
				if err := decodeJSON(r, &payload); err != nil {
					http.Error(w, err.Error(), http.StatusBadRequest)
					return
				}
				writeJSON(w, http.StatusCreated, svc.UpsertAlertChannel(payload))
				return
			default:
				http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
				return
			}

		default:
			http.NotFound(w, r)
			return
		}

	case "kpi":
		if len(parts) == 4 && parts[3] == "report" && r.Method == http.MethodGet {
			writeJSON(w, http.StatusOK, svc.KPIReport())
			return
		}
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
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
