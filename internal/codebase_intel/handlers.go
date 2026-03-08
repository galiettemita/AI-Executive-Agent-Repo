package codebase_intel

import (
	"encoding/json"
	"net/http"
	"strings"
)

// RegisterRoutes registers codebase intelligence HTTP handlers on the provided mux.
func RegisterRoutes(mux *http.ServeMux, svc *Service) {
	mux.HandleFunc("GET /v1/codebase/dependencies", handleGetDependencies(svc))
	mux.HandleFunc("GET /v1/codebase/patterns", handleGetPatterns(svc))
	mux.HandleFunc("GET /v1/codebase/debt", handleGetDebt(svc))
}

func handleGetDependencies(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		workspaceID := workspaceFromRequest(r)
		deps := svc.ListDependencies(workspaceID)
		writeJSON(w, http.StatusOK, deps)
	}
}

func handleGetPatterns(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		workspaceID := workspaceFromRequest(r)
		patterns := svc.ListPatterns(workspaceID)
		writeJSON(w, http.StatusOK, patterns)
	}
}

func handleGetDebt(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		workspaceID := workspaceFromRequest(r)
		debt := svc.ListDebt(workspaceID)
		writeJSON(w, http.StatusOK, debt)
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
