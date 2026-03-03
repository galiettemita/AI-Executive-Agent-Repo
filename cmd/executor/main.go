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
		ServiceName:         "executor",
		DefaultListenAddr:   ":18083",
		RequiredNonLocalEnv: []string{"DATABASE_URL", "REDIS_URL", "TEMPORAL_HOST"},
	})
	if err != nil {
		log.Fatalf("executor config validation failed: %v", err)
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
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status":    "healthy",
			"version":   cfg.ServiceVersion,
			"uptime_ms": time.Since(startedAt).Milliseconds(),
			"checks": map[string]string{
				"process":  "ok",
				"db":       runtimeserver.EnvStatus("DATABASE_URL"),
				"redis":    runtimeserver.EnvStatus("REDIS_URL"),
				"temporal": runtimeserver.EnvStatus("TEMPORAL_HOST"),
			},
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

	log.Printf("BREVIO executor listening on %s env=%s version=%s", cfg.ListenAddr, cfg.Environment, cfg.ServiceVersion)
	if err := runtimeserver.ServeWithGracefulShutdown("executor", cfg.ListenAddr, mux); err != nil {
		log.Fatalf("executor server failed: %v", err)
	}
}
