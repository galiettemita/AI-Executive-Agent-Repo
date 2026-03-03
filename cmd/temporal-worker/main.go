package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"

	runtimeserver "github.com/brevio/brevio/internal/runtime"
)

func main() {
	cfg, err := runtimeserver.LoadServiceEnvConfig(os.Getenv, runtimeserver.ServiceEnvOptions{
		ServiceName:         "temporal-worker",
		DefaultListenAddr:   ":18084",
		RequiredNonLocalEnv: []string{"DATABASE_URL", "REDIS_URL", "TEMPORAL_HOST", "TEMPORAL_NAMESPACE"},
	})
	if err != nil {
		log.Fatalf("temporal-worker config validation failed: %v", err)
	}
	mux := http.NewServeMux()
	startedAt := time.Now().UTC()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status":    "healthy",
			"version":   cfg.ServiceVersion,
			"uptime_ms": time.Since(startedAt).Milliseconds(),
			"checks": map[string]string{
				"process": "ok",
			},
		})
	})
	mux.HandleFunc("GET /health/deep", func(w http.ResponseWriter, _ *http.Request) {
		checks := map[string]string{
			"process": "ok",
		}
		for key, status := range runtimeserver.DeepDependencyChecks(os.Getenv) {
			checks[key] = status
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status":    "healthy",
			"version":   cfg.ServiceVersion,
			"uptime_ms": time.Since(startedAt).Milliseconds(),
			"checks":    checks,
		})
	})
	mux.HandleFunc("GET /healthz/ready", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("GET /healthz/live", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	logger := runtimeserver.NewJSONLogger("temporal-worker", cfg.Environment)
	logger.SetOutput(os.Stdout)
	handler := logger.Middleware(mux)

	logger.Info("service_start", map[string]any{
		"listen_addr": cfg.ListenAddr,
		"version":     cfg.ServiceVersion,
	})
	if err := runtimeserver.ServeWithGracefulShutdown("temporal-worker", cfg.ListenAddr, handler); err != nil {
		log.Fatalf("temporal worker failed: %v", err)
	}
}
