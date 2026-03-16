package self_modification

import (
	"encoding/json"
	"net/http"
)

// RegisterRoutes mounts self-modification policy management endpoints on the mux.
func RegisterRoutes(mux *http.ServeMux, svc *Service) {
	mux.HandleFunc("GET /v1/self-modification/policy/{workspaceID}", handleGetPolicy(svc))
	mux.HandleFunc("PUT /v1/self-modification/policy/{workspaceID}", handleUpsertPolicy(svc))
	mux.HandleFunc("POST /v1/self-modification/evaluate/{workspaceID}", handleEvaluateAction(svc))
	mux.HandleFunc("GET /v1/self-modification/decisions/{workspaceID}", handleListDecisions(svc))
}

func handleGetPolicy(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		workspaceID := r.PathValue("workspaceID")
		policy, found := svc.GetPolicy(workspaceID)
		if !found {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "policy not found"})
			return
		}
		writeJSON(w, http.StatusOK, policy)
	}
}

func handleUpsertPolicy(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		workspaceID := r.PathValue("workspaceID")
		var policy Policy
		if err := json.NewDecoder(r.Body).Decode(&policy); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
			return
		}
		policy.WorkspaceID = workspaceID
		updated, err := svc.UpsertPolicyStrict(workspaceID, policy)
		if err != nil {
			writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, updated)
	}
}

func handleEvaluateAction(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		workspaceID := r.PathValue("workspaceID")
		var req ActionRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
			return
		}
		req.WorkspaceID = workspaceID
		decision := svc.EvaluateAction(workspaceID, req)
		writeJSON(w, http.StatusOK, decision)
	}
}

func handleListDecisions(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		workspaceID := r.PathValue("workspaceID")
		decisions := svc.Decisions(workspaceID)
		writeJSON(w, http.StatusOK, map[string]any{
			"workspace_id": workspaceID,
			"decisions":    decisions,
			"count":        len(decisions),
		})
	}
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
