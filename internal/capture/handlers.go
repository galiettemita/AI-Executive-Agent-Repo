package capture

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

// RegisterRoutes registers capture HTTP handlers on the provided mux.
func RegisterRoutes(mux *http.ServeMux, svc *Service) {
	mux.HandleFunc("GET /v1/captures/daily", handleGetDailyCapture(svc))
	mux.HandleFunc("GET /v1/captures/{date}", handleListCaptures(svc))
}

func handleGetDailyCapture(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		workspaceID := workspaceFromRequest(r)
		date := r.URL.Query().Get("date")
		if date == "" {
			date = time.Now().UTC().Format("2006-01-02")
		}

		capture, ok := svc.Get(workspaceID, date)
		if !ok {
			// Return a default/empty capture for today if none exists.
			capture = DailyCapture{
				WorkspaceID: workspaceID,
				CaptureDate: date,
				Summary:     "",
				Wins:        []string{},
				Blockers:    []string{},
				NextActions: []string{},
				Status:      "pending",
			}
		}
		writeJSON(w, http.StatusOK, capture)
	}
}

func handleListCaptures(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		workspaceID := workspaceFromRequest(r)
		date := r.PathValue("date")
		if date != "" {
			// Specific date requested.
			capture, ok := svc.Get(workspaceID, date)
			if !ok {
				writeError(w, http.StatusNotFound, "capture not found for date")
				return
			}
			writeJSON(w, http.StatusOK, capture)
			return
		}

		captures := svc.List(workspaceID)
		writeJSON(w, http.StatusOK, captures)
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
