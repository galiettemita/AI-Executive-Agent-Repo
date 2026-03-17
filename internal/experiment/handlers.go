package experiment

import (
	"encoding/json"
	"net/http"
)

// RegisterRoutes mounts experiment management endpoints on the mux.
func RegisterRoutes(mux *http.ServeMux, router *ExperimentRouter) {
	mux.HandleFunc("GET /v1/experiments", handleListExperiments(router))
	mux.HandleFunc("POST /v1/experiments", handleCreateExperiment(router))
	mux.HandleFunc("PUT /v1/experiments/{id}/start", handleStartExperiment(router))
}

func handleListExperiments(router *ExperimentRouter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		exps, err := router.ListExperiments(r.Context())
		if err != nil {
			writeExpJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		if exps == nil {
			exps = []Experiment{}
		}
		writeExpJSON(w, http.StatusOK, map[string]any{"experiments": exps, "count": len(exps)})
	}
}

func handleCreateExperiment(router *ExperimentRouter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var exp Experiment
		if err := json.NewDecoder(r.Body).Decode(&exp); err != nil {
			writeExpJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
			return
		}
		if exp.Name == "" || exp.ControlPrompt == "" || exp.VariantPrompt == "" {
			writeExpJSON(w, http.StatusBadRequest, map[string]string{"error": "name, control_prompt, and variant_prompt are required"})
			return
		}
		id, err := router.CreateExperiment(r.Context(), exp)
		if err != nil {
			writeExpJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeExpJSON(w, http.StatusCreated, map[string]string{"id": id, "status": "draft"})
	}
}

func handleStartExperiment(router *ExperimentRouter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if id == "" {
			writeExpJSON(w, http.StatusBadRequest, map[string]string{"error": "experiment id required"})
			return
		}
		if err := router.StartExperiment(r.Context(), id); err != nil {
			writeExpJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeExpJSON(w, http.StatusOK, map[string]string{"id": id, "status": "running"})
	}
}

func writeExpJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
