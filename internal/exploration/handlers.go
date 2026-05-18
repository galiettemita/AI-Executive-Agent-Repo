package exploration

import (
	"encoding/json"
	"net/http"
	"strings"
)

// RegisterRoutes registers exploration/capability HTTP handlers on the provided mux.
func RegisterRoutes(mux *http.ServeMux, svc *Service) {
	mux.HandleFunc("GET /v1/capabilities/recommendations", handleListRecommendations(svc))
	mux.HandleFunc("POST /v1/capabilities/{action}", handleAdoptRecommendation(svc))
}

func handleListRecommendations(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		workspaceID := workspaceFromRequest(r)
		recommendations := svc.ListRecommendations(workspaceID)
		writeJSON(w, http.StatusOK, recommendations)
	}
}

func handleAdoptRecommendation(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		action := r.PathValue("action")
		if action == "" {
			writeError(w, http.StatusBadRequest, "action is required")
			return
		}

		var input struct {
			RecommendationID string `json:"recommendation_id"`
			Decision         string `json:"decision"` // accept, reject, defer
		}
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		if input.RecommendationID == "" {
			writeError(w, http.StatusBadRequest, "recommendation_id is required")
			return
		}
		if input.Decision == "" {
			input.Decision = action
		}

		rec, found, err := svc.DecideRecommendation(input.RecommendationID, input.Decision)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		if !found {
			writeError(w, http.StatusNotFound, "recommendation not found")
			return
		}
		writeJSON(w, http.StatusOK, rec)
	}
}

func workspaceFromRequest(r *http.Request) string {
	ws := r.Header.Get("X-Workspace-ID")
	if strings.TrimSpace(ws) == "" {
		return "default"
	}
	return ws
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
}
