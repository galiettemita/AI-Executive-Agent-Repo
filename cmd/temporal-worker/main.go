package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/brevio/brevio/internal/compliance"
	"github.com/brevio/brevio/internal/outbox"
	runtimeserver "github.com/brevio/brevio/internal/runtime"
	breviotemporal "github.com/brevio/brevio/internal/temporal"
	"github.com/jackc/pgx/v5/pgxpool"
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

	logger := runtimeserver.NewJSONLogger("temporal-worker", cfg.Environment)
	logger.SetOutput(os.Stdout)

	// Start Temporal worker
	temporalClient, err := breviotemporal.NewClient()
	if err != nil {
		logger.Info("temporal_client_failed", map[string]any{
			"error": err.Error(),
		})
		log.Fatalf("failed to create temporal client: %v", err)
	}
	defer temporalClient.Close()

	// Build activity dependencies based on runtime environment.
	// When DATABASE_URL is set, activities use pgx-backed repositories and
	// the transactional outbox service. Otherwise, degraded/test mode.
	deps := breviotemporal.ActivityDeps{}
	dbURL := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	if dbURL != "" {
		ctx := context.Background()
		pool, poolErr := pgxpool.New(ctx, dbURL)
		if poolErr != nil {
			log.Fatalf("failed to create pgx pool: %v", poolErr)
		}
		defer pool.Close()

		deps.Pool = pool
		deps.OutboxSvc = outbox.NewService(pool)

		logger.Info("temporal_worker_production_deps", map[string]any{
			"database": "pgxpool",
			"outbox":   "db-backed",
		})
	}

	w := breviotemporal.NewWorkerWithDeps(temporalClient, breviotemporal.TaskQueueCore, deps)

	go func() {
		if runErr := w.Run(nil); runErr != nil {
			logger.Info("temporal_worker_stopped", map[string]any{
				"error": runErr.Error(),
			})
			log.Fatalf("temporal worker failed: %v", runErr)
		}
	}()

	logger.Info("temporal_worker_started", map[string]any{
		"task_queue": breviotemporal.TaskQueueCore,
		"production": dbURL != "",
	})

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
				"process":  "ok",
				"temporal": "polling",
			},
		})
	})
	mux.HandleFunc("GET /health/deep", func(w http.ResponseWriter, _ *http.Request) {
		checks := map[string]string{
			"process":  "ok",
			"temporal": "polling",
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
