package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/brevio/brevio/internal/compliance"
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

	scrubStore, err := compliance.NewPGExecutionLogPIIScrubStore(context.Background(), os.Getenv("DATABASE_URL"))
	if err != nil {
		logger.Info("execution_log_scrubber_disabled", map[string]any{
			"reason": err.Error(),
		})
	} else {
		defer scrubStore.Close()
		scheduler := compliance.NewExecutionLogPIIScrubScheduler(
			scrubStore,
			compliance.DefaultExecutionLogPIIScrubBatchSize,
			func(format string, args ...any) {
				logger.Info("execution_log_scrubber_log", map[string]any{
					"message": fmt.Sprintf(format, args...),
				})
			},
		)
		go func() {
			if runErr := scheduler.Run(context.Background()); runErr != nil {
				logger.Info("execution_log_scrubber_stopped", map[string]any{
					"error": runErr.Error(),
				})
			}
		}()
		logger.Info("execution_log_scrubber_started", map[string]any{
			"next_run_at": compliance.NextExecutionLogPIIScrubRun(time.Now().UTC()).Format(time.RFC3339),
		})
	}

	if err := runtimeserver.ServeWithGracefulShutdown("temporal-worker", cfg.ListenAddr, handler); err != nil {
		log.Fatalf("temporal worker failed: %v", err)
	}
}
