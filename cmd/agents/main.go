package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/brevio/brevio/internal/agents"
	runtimeserver "github.com/brevio/brevio/internal/runtime"
)

func main() {
	cfg, err := runtimeserver.LoadServiceEnvConfig(os.Getenv, runtimeserver.ServiceEnvOptions{
		ServiceName:         "agents",
		DefaultListenAddr:   ":18092",
		RequiredNonLocalEnv: []string{"DATABASE_URL", "REDIS_URL", "TEMPORAL_HOST"},
	})
	if err != nil {
		log.Fatalf("agents config validation failed: %v", err)
	}

	logger := runtimeserver.NewJSONLogger("agents", cfg.Environment)
	logger.SetOutput(os.Stdout)

	svc := agents.NewService(agents.Config{
		DatabaseURL:  os.Getenv("DATABASE_URL"),
		RedisURL:     os.Getenv("REDIS_URL"),
		TemporalHost: os.Getenv("TEMPORAL_HOST"),
	}, logger)

	mux := http.NewServeMux()
	startedAt := time.Now().UTC()

	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status":    "healthy",
			"version":   cfg.ServiceVersion,
			"uptime_ms": time.Since(startedAt).Milliseconds(),
			"checks":    map[string]string{"process": "ok"},
		})
	})
	mux.HandleFunc("GET /health/deep", func(w http.ResponseWriter, _ *http.Request) {
		checks := map[string]string{"process": "ok"}
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

	svc.RegisterRoutes(mux)

	handler := logger.Middleware(mux)
	logger.Info("service_start", map[string]any{
		"listen_addr": cfg.ListenAddr,
		"version":     cfg.ServiceVersion,
	})
	if err := runtimeserver.ServeWithGracefulShutdown("agents", cfg.ListenAddr, handler); err != nil {
		log.Fatalf("agents server failed: %v", err)
	}
}
