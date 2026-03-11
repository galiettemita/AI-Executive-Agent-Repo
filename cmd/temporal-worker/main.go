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

	adminpkg "github.com/brevio/brevio/internal/admin"
	brainpkg "github.com/brevio/brevio/internal/brain"
	callpkg "github.com/brevio/brevio/internal/hands/call"
	cognitionpkg "github.com/brevio/brevio/internal/cognition"
	"github.com/brevio/brevio/internal/compliance"
	contextpkg "github.com/brevio/brevio/internal/context"
	eqpkg "github.com/brevio/brevio/internal/eq"
	executorpkg "github.com/brevio/brevio/internal/executor"
	memorypkg "github.com/brevio/brevio/internal/memory"
	onboardingpkg "github.com/brevio/brevio/internal/onboarding"
	"github.com/brevio/brevio/internal/outbox"
	ragpkg "github.com/brevio/brevio/internal/rag"
	runtimeserver "github.com/brevio/brevio/internal/runtime"
	breviotemporal "github.com/brevio/brevio/internal/temporal"
	trustpkg "github.com/brevio/brevio/internal/trust"
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
		deps.MemoryRepo = memorypkg.NewPgItemRepository(pool)
		deps.OnboardingRepo = onboardingpkg.NewPgRepository(pool)
		deps.RAGRepo = ragpkg.NewPgRepository(pool)
		deps.KillSwitchCheck = adminpkg.NewPgKillSwitchRepository(pool)
		deps.SkillACLCheck = adminpkg.NewPgSkillACLRepository(pool)
		deps.EQRepo = eqpkg.NewPgEQStrategyRepository(pool)
		deps.DemotionRepo = trustpkg.NewPgDemotionRepository(pool)
		deps.IntelligenceRepo = brainpkg.NewPgIntelligenceRepository(pool)
		deps.DecayRepo = memorypkg.NewPgDecayRepository(pool)
		deps.ConflictRepo = memorypkg.NewPgConflictRepository(pool)
		deps.ChunkSpecRepo = ragpkg.NewPgChunkSpecRepository(pool)
		deps.CompressionRepo = contextpkg.NewPgCompressionRepository(pool)
		deps.ContextRepo = contextpkg.NewPgRepository(pool)
		deps.LatencyRepo = executorpkg.NewPgLatencyRepository(pool)
		deps.EmbeddingProvider = ragpkg.NewOpenAIEmbeddingProvider("", os.Getenv("OPENAI_API_KEY"))
		deps.CognitiveRepo = cognitionpkg.NewPgCognitiveRepository(pool)
		deps.CallRepo = callpkg.NewPgCallRepository(pool)
		deps.PhoneVerifier = callpkg.NewGooglePlacesClient(os.Getenv("GOOGLE_PLACES_API_KEY"))

		logger.Info("temporal_worker_production_deps", map[string]any{
			"database":        "pgxpool",
			"outbox":          "db-backed",
			"memory":          "pg-repository",
			"onboarding":      "pg-repository",
			"rag":             "pg-repository",
			"kill_switch":     "pg-repository",
			"skill_acl":       "pg-repository",
			"eq_strategy":     "pg-repository",
			"demotion":        "pg-repository",
			"intelligence":    "pg-repository",
			"decay":           "pg-repository",
			"conflict":        "pg-repository",
			"chunk_spec":      "pg-repository",
			"compression":     "pg-repository",
			"context_budget":  "pg-repository",
			"latency":         "pg-repository",
			"embedding":       "openai",
			"cognitive":       "pg-repository",
			"call":            "pg-repository",
			"phone_verifier":  "google-places",
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
