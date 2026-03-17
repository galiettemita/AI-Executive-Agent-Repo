package admin

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/brevio/brevio/internal/benchmark"
)

// ExtendWithBenchmark adds a benchmarkRepo to the Service.
func (s *Service) ExtendWithBenchmark(repo *benchmark.Repository) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.benchmarkRepo = repo
}

// RegisterBenchmarkRoutes adds benchmark admin endpoints.
func RegisterBenchmarkRoutes(mux *http.ServeMux, svc *Service) {
	mux.HandleFunc("GET /v1/admin/benchmark/latest", adminOnly(handleBenchmarkLatest(svc)))
	mux.HandleFunc("GET /v1/admin/benchmark/history", adminOnly(handleBenchmarkHistory(svc)))
}

func handleBenchmarkLatest(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if svc.benchmarkRepo == nil {
			http.Error(w, `{"error":"benchmark not configured"}`, http.StatusServiceUnavailable)
			return
		}
		run, err := svc.benchmarkRepo.LatestRun(r.Context())
		if err != nil {
			http.Error(w, `{"error":"no benchmark runs found"}`, http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(run)
	}
}

func handleBenchmarkHistory(svc *Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if svc.benchmarkRepo == nil {
			http.Error(w, `{"error":"benchmark not configured"}`, http.StatusServiceUnavailable)
			return
		}
		limit := 10
		if l := r.URL.Query().Get("limit"); l != "" {
			if n, err := strconv.Atoi(l); err == nil && n > 0 && n <= 100 {
				limit = n
			}
		}
		runs, err := svc.benchmarkRepo.RunHistory(r.Context(), limit)
		if err != nil {
			http.Error(w, `{"error":"failed to fetch history"}`, http.StatusInternalServerError)
			return
		}
		if runs == nil {
			runs = []benchmark.BenchmarkRun{}
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"runs": runs, "count": len(runs)})
	}
}
