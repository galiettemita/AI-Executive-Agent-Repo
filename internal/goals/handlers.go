package goals

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

// RegisterRoutes registers goal HTTP handlers on the provided mux.
func RegisterRoutes(mux *http.ServeMux, svc *Service) {
	mux.HandleFunc("POST /v1/goals", handleCreateGoal(svc))
	mux.HandleFunc("GET /v1/goals", handleListGoals(svc))
	mux.HandleFunc("GET /v1/goals/{id}", handleGetGoal(svc))
	mux.HandleFunc("PATCH /v1/goals/{id}", handleUpdateGoal(svc))
	mux.HandleFunc("DELETE /v1/goals/{id}", handleDeleteGoal(svc))
}

func handleCreateGoal(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		workspaceID := workspaceFromRequest(r)

		var input struct {
			Title    string `json:"title"`
			Status   string `json:"status"`
			Priority string `json:"priority"`
		}
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		if strings.TrimSpace(input.Title) == "" {
			writeError(w, http.StatusBadRequest, "title is required")
			return
		}

		goal, err := svc.CreateGoal(Goal{
			WorkspaceID: workspaceID,
			Title:       input.Title,
			Status:      input.Status,
			Priority:    input.Priority,
		}, time.Now().UTC())
		if err != nil {
			if strings.Contains(err.Error(), "rate limit") {
				writeError(w, http.StatusTooManyRequests, err.Error())
				return
			}
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}

		writeJSON(w, http.StatusCreated, goal)
	}
}

func handleListGoals(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		workspaceID := workspaceFromRequest(r)
		goals := svc.ListGoals(workspaceID)
		writeJSON(w, http.StatusOK, goals)
	}
}

func handleGetGoal(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if id == "" {
			writeError(w, http.StatusBadRequest, "goal id is required")
			return
		}

		goal, ok := svc.GetGoal(id)
		if !ok {
			writeError(w, http.StatusNotFound, "goal not found")
			return
		}
		writeJSON(w, http.StatusOK, goal)
	}
}

func handleUpdateGoal(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if id == "" {
			writeError(w, http.StatusBadRequest, "goal id is required")
			return
		}

		existing, ok := svc.GetGoal(id)
		if !ok {
			writeError(w, http.StatusNotFound, "goal not found")
			return
		}

		var input struct {
			Title    *string `json:"title"`
			Status   *string `json:"status"`
			Priority *string `json:"priority"`
		}
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		if input.Title != nil {
			existing.Title = *input.Title
		}
		if input.Status != nil {
			existing.Status = *input.Status
		}
		if input.Priority != nil {
			existing.Priority = *input.Priority
		}

		updated := svc.UpsertGoal(existing)
		writeJSON(w, http.StatusOK, updated)
	}
}

func handleDeleteGoal(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if id == "" {
			writeError(w, http.StatusBadRequest, "goal id is required")
			return
		}

		if !svc.DeleteGoal(id) {
			writeError(w, http.StatusNotFound, "goal not found")
			return
		}
		w.WriteHeader(http.StatusNoContent)
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
