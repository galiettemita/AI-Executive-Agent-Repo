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
	"github.com/brevio/brevio/internal/connectors"
	handspkg "github.com/brevio/brevio/internal/hands"
	callpkg "github.com/brevio/brevio/internal/hands/call"
	cognitionpkg "github.com/brevio/brevio/internal/cognition"
	"github.com/brevio/brevio/internal/compliance"
	contextpkg "github.com/brevio/brevio/internal/context"
	eqpkg "github.com/brevio/brevio/internal/eq"
	executorpkg "github.com/brevio/brevio/internal/executor"
	llmpkg "github.com/brevio/brevio/internal/llm"
	policypkg "github.com/brevio/brevio/internal/policy"
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

	// Bootstrap LLM intelligence layer from environment variables.
	llmSvc := llmpkg.BootstrapService()

	// Bootstrap OPA policy evaluator.
	opaEval, opaErr := policypkg.NewEvaluator()
	if opaErr != nil {
		log.Fatalf("OPA evaluator init failed: %v", opaErr)
	}
	logger.Info("opa_evaluator_ready", map[string]any{"status": "policies_loaded"})

	// Build activity dependencies based on runtime environment.
	// When DATABASE_URL is set, activities use pgx-backed repositories and
	// the transactional outbox service. Otherwise, degraded/test mode.
	deps := breviotemporal.ActivityDeps{
		LLMService:   llmSvc,
		OPAEvaluator: opaEval,
	}
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
		rawEmbedder := ragpkg.NewOpenAIEmbeddingProvider("", os.Getenv("OPENAI_API_KEY"))
		cachedEmbedder := ragpkg.NewEmbeddingService(rawEmbedder)
		ragpkg.ValidateEmbeddingDimensions(cachedEmbedder)
		deps.EmbeddingProvider = cachedEmbedder
		deps.CognitiveRepo = cognitionpkg.NewPgCognitiveRepository(pool)
		deps.CallRepo = callpkg.NewPgCallRepository(pool)
		deps.PhoneVerifier = callpkg.NewGooglePlacesClient(os.Getenv("GOOGLE_PLACES_API_KEY"))

		llmStatus := "disabled"
		if llmSvc.Intelligence() != nil {
			llmStatus = "active"
		}
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
			"llm_intelligence": llmStatus,
		})
	}

	// Wire OAuth credential resolver for tool execution.
	if deps.Pool != nil {
		tokenRefresher := connectors.NewTokenRefresher(connectors.NewService(connectors.NewInMemoryKeyProvider("v0", make([]byte, 32))))
		credConnSvc := connectors.NewService(connectors.NewInMemoryKeyProvider("v0", make([]byte, 32)))
		deps.CredentialResolver = connectors.NewCredentialResolver(credConnSvc, tokenRefresher)
		logger.Info("credential_resolver_wired", map[string]any{"status": "oauth_tokens_active"})
	}

	// REPAIR: Wire HandsExecutor — connects data plane to control plane.
	// Uses in-process hands service with MCP client for tool execution.
	seedPath := strings.TrimSpace(os.Getenv("CONNECTORS_SEED_FILE"))
	if seedPath == "" {
		seedPath = "internal/connectors/seeds/connectors.yaml"
	}
	handsKeyMaterial := strings.TrimSpace(os.Getenv("CONNECTORS_MASTER_KEY_B64"))
	var handsKey []byte
	if handsKeyMaterial != "" {
		handsKey = []byte(handsKeyMaterial)
	} else if cfg.Environment == "local" || cfg.Environment == "test" {
		handsKey = make([]byte, 32) // zero key acceptable in local/test only
	}
	if handsKey != nil {
		kp := connectors.NewInMemoryKeyProvider("v0", handsKey)
		connSvc := connectors.NewService(kp)
		if loadErr := connSvc.LoadSeedFile(seedPath); loadErr != nil {
			logger.Info("hands_seed_load_warning", map[string]any{"error": loadErr.Error()})
		}
		mcpClient := handspkg.NewHTTPMCPClient(30 * time.Second)
		handsSvc := handspkg.NewService(connSvc, mcpClient)
		deps.HandsExecutor = handspkg.NewExecutorAdapter(handsSvc)
		logger.Info("hands_executor_wired", map[string]any{"status": "enabled", "skills": len(handsSvc.ListSkills())})
	} else {
		// Fail-fast: non-local/test environments must have HandsExecutor configured.
		if cfg.Environment != "" && cfg.Environment != "local" && cfg.Environment != "test" {
			log.Fatalf("CONNECTORS_MASTER_KEY_B64 is required in %s environment — HandsExecutor cannot be nil in production", cfg.Environment)
		}
		logger.Info("hands_executor_degraded_mode", map[string]any{
			"reason": "CONNECTORS_MASTER_KEY_B64 not set (acceptable in local/test only)",
		})
	}

	// REPAIR: Wire OutboxDispatcher — enables real outbound delivery.
	deps.OutboxDispatcher = breviotemporal.NewHTTPOutboxDispatcher(30 * time.Second)
	logger.Info("outbox_dispatcher_wired", map[string]any{"status": "enabled", "type": "http"})

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
