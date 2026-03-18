package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	adminpkg "github.com/brevio/brevio/internal/admin"
	brainpkg "github.com/brevio/brevio/internal/brain"
	cachingpkg "github.com/brevio/brevio/internal/caching"
	euaiactpkg "github.com/brevio/brevio/internal/compliance/eu_ai_act"
	"github.com/brevio/brevio/internal/connectors"
	handspkg "github.com/brevio/brevio/internal/hands"
	callpkg "github.com/brevio/brevio/internal/hands/call"
	cognitionpkg "github.com/brevio/brevio/internal/cognition"
	"github.com/brevio/brevio/internal/compliance"
	contextpkg "github.com/brevio/brevio/internal/context"
	eqpkg "github.com/brevio/brevio/internal/eq"
	executorpkg "github.com/brevio/brevio/internal/executor"
	experimentpkg "github.com/brevio/brevio/internal/experiment"
	a2apkg "github.com/brevio/brevio/internal/a2a"
	benchmarkpkg "github.com/brevio/brevio/internal/benchmark"
	browserpkg "github.com/brevio/brevio/internal/browser"
	delegationpkg "github.com/brevio/brevio/internal/delegation"
	kgpkg "github.com/brevio/brevio/internal/memory/kg"
	consentpkg "github.com/brevio/brevio/internal/compliance/consent"
	soc2pkg "github.com/brevio/brevio/internal/compliance/soc2"
	learningpkg "github.com/brevio/brevio/internal/learning"
	redteampkg "github.com/brevio/brevio/internal/security/redteam"
	sandboxpkg "github.com/brevio/brevio/internal/security/sandbox"
	watermarkpkg "github.com/brevio/brevio/internal/watermark"
	billingpkg "github.com/brevio/brevio/internal/billing"
	simulationpkg "github.com/brevio/brevio/internal/simulation"
	walletpkg "github.com/brevio/brevio/internal/wallet"
	dpopkg "github.com/brevio/brevio/internal/dpo"
	featureflagspkg "github.com/brevio/brevio/internal/feature_flags"
	ragevalpkg "github.com/brevio/brevio/internal/rag/eval"
	guardrailspkg "github.com/brevio/brevio/internal/guardrails"
	proactivepkg "github.com/brevio/brevio/internal/proactive"
	preferencepkg "github.com/brevio/brevio/internal/preference"
	llmpkg "github.com/brevio/brevio/internal/llm"
	policypkg "github.com/brevio/brevio/internal/policy"
	memorypkg "github.com/brevio/brevio/internal/memory"
	onboardingpkg "github.com/brevio/brevio/internal/onboarding"
	"github.com/brevio/brevio/internal/outbox"
	ragpkg "github.com/brevio/brevio/internal/rag"
	runtimeserver "github.com/brevio/brevio/internal/runtime"
	breviotemporal "github.com/brevio/brevio/internal/temporal"
	trustpkg "github.com/brevio/brevio/internal/trust"
	workingmemorypkg "github.com/brevio/brevio/internal/workingmemory"
	"github.com/jackc/pgx/v5/pgxpool"
	goredis "github.com/redis/go-redis/v9"
	temporalclient "go.temporal.io/sdk/client"
)

// llmCompleteAdapter wraps an llm.Client to satisfy the brain.CAILLMClient
// and rag.CitationLLM interfaces (Complete(ctx, system, user) (string, error)).
type llmCompleteAdapter struct {
	client llmpkg.Client
}

func (a *llmCompleteAdapter) Complete(ctx context.Context, system, user string) (string, error) {
	resp, _, err := a.client.Generate(ctx, llmpkg.GenerateRequest{
		Model:     "claude-haiku-4-5-20251001",
		MaxTokens: 1024,
		Messages: []llmpkg.ChatMsg{
			{Role: "system", Content: system},
			{Role: "user", Content: user},
		},
	})
	if err != nil {
		return "", err
	}
	return resp.Content, nil
}

// mainLoggerAdapter adapts the runtime JSON logger to the brain.CAILogger / rag.Logger interfaces.
type mainLoggerAdapter struct {
	logger *runtimeserver.JSONLogger
}

func (a *mainLoggerAdapter) Info(msg string, args ...any)  { a.logger.Info(msg, map[string]any{"detail": fmt.Sprint(args...)}) }
func (a *mainLoggerAdapter) Warn(msg string, args ...any)  { a.logger.Info("WARN_"+msg, map[string]any{"detail": fmt.Sprint(args...)}) }
func (a *mainLoggerAdapter) Error(msg string, args ...any) { a.logger.Info("ERROR_"+msg, map[string]any{"detail": fmt.Sprint(args...)}) }

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
		deps.EQService = eqpkg.NewEQService()
		if eqABTracker, eqABErr := eqpkg.NewEQABTracker(pool); eqABErr == nil {
			deps.EQABTracker = eqABTracker
		}
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
		if err := ragpkg.ValidateEmbeddingDimensions(cachedEmbedder); err != nil {
			log.Fatalf("failed to validate embedding dimensions: %v", err)
		}
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

	// Wire working memory tier (Redis-backed).
	if redisURLForWM := strings.TrimSpace(os.Getenv("REDIS_URL")); redisURLForWM != "" {
		wmOpts, wmErr := goredis.ParseURL(redisURLForWM)
		if wmErr != nil {
			logger.Info("working_memory_redis_parse_error", map[string]any{"error": wmErr.Error()})
		} else {
			wmRDB := goredis.NewClient(wmOpts)
			wmAdapter := workingmemorypkg.NewGoRedisAdapter(wmRDB)
			wmRepo, wmRepoErr := workingmemorypkg.NewRepository(wmAdapter)
			if wmRepoErr != nil {
				log.Fatalf("failed to create working memory repository: %v", wmRepoErr)
			}
			wmLogger := &wmLogAdapter{logger: logger}
			wmSvc := workingmemorypkg.NewService(wmRepo, wmLogger)
			deps.WorkingMemory = wmSvc
			logger.Info("working_memory_initialized", map[string]any{"backend": "redis"})
		}
	}

	// Wire PAHF preference learning loop.
	pahfMemorySvc := memorypkg.NewService()
	deps.MemorySvc = pahfMemorySvc
	deps.PreferenceRetriever = preferencepkg.NewRetriever(pahfMemorySvc)
	logger.Info("preference_learning_initialized", map[string]any{"status": "pahf_active"})

	// Wire IPI inference guard for post-tool-call taint tracking.
	deps.InferenceGuard = guardrailspkg.NewInferenceGuard()
	logger.Info("inference_guard_initialized", map[string]any{"ipi_rules": "active"})

	// ── Semantic LLM Cache (pgvector-backed, 94% similarity, 4h TTL) ──────────
	if llmSvc.Intelligence() != nil {
		rawEmbed := ragpkg.NewOpenAIEmbeddingProvider("", os.Getenv("OPENAI_API_KEY"))
		semCache := llmpkg.NewSemanticCache(
			nil, // nil store → in-memory fallback; swap for pgvector store when ready
			rawEmbed,
			llmpkg.DefaultSemanticCacheConfig(),
		)
		deps.SemanticCache = semCache
		logger.Info("semantic_cache_initialized", map[string]any{
			"similarity_threshold": llmpkg.DefaultSemanticCacheConfig().SimilarityThreshold,
			"ttl":                  llmpkg.DefaultSemanticCacheConfig().TTL.String(),
		})

		// Wire LLM cache bridge (L1→L2→L3→semantic).
		cacheLayers := cachingpkg.NewService()
		llmCacheBridge, bridgeErr := cachingpkg.NewLLMCacheBridge(semCache, cacheLayers)
		if bridgeErr != nil {
			log.Fatalf("failed to construct LLMCacheBridge: %v", bridgeErr)
		}
		deps.LLMCacheBridge = llmCacheBridge
		logger.Info("llm_cache_bridge_initialized", map[string]any{"status": "active"})
	}

	// ── Constitutional AI + Citation Attribution (Plan 06) ─────────────────────
	if anthropicKey := os.Getenv("ANTHROPIC_API_KEY"); anthropicKey != "" {
		caiClient, caiErr := llmpkg.NewAnthropicClient(llmpkg.AnthropicConfig{
			APIKey:  anthropicKey,
			Timeout: 30 * time.Second,
		})
		if caiErr == nil {
			caiAdapter := &llmCompleteAdapter{client: caiClient}
			deps.ConstitutionalAICritiquer = brainpkg.NewConstitutionalAICritiquer(
				caiAdapter, brainpkg.DefaultCAIConfig(), &mainLoggerAdapter{logger: logger},
			)
			deps.CitationExtractor = ragpkg.NewCitationExtractor(
				caiAdapter, &mainLoggerAdapter{logger: logger},
			)
			logger.Info("cai_and_citations_initialized", map[string]any{"status": "active"})
		}
	}

	// Wire A/B experiment routing.
	if deps.Pool != nil {
		deps.ExperimentRouter = experimentpkg.NewExperimentRouter(deps.Pool)
		deps.VariantScoreStore = experimentpkg.NewVariantScoreStore(deps.Pool)
		logger.Info("experiment_router_initialized", map[string]any{"status": "active"})
	}

	// Wire proactive monitor.
	proactiveSnooze := proactivepkg.NewSnoozeStore(deps.Pool)
	deps.ProactiveMonitor = proactivepkg.NewProactiveMonitor(deps.Pool, proactiveSnooze, nil, nil)
	deps.OfferBuilder = proactivepkg.NewOfferBuilder()
	logger.Info("proactive_monitor_initialized", map[string]any{"status": "active"})

	// Wire A2A client and external agent registry.
	a2aRegistry := a2apkg.NewExternalAgentRegistry(deps.Pool)
	a2aTaskStore := a2apkg.NewTaskStore(deps.Pool)
	deps.ExternalAgentRegistry = a2aRegistry
	deps.A2AClient = a2apkg.NewA2AClient(a2aRegistry, a2aTaskStore)
	logger.Info("a2a_client_initialized", map[string]any{"status": "active"})

	// Wire DPO pipeline.
	dpoRepo := dpopkg.NewRepository(deps.Pool)
	ftc, ftcErr := dpopkg.NewFineTuneClient()
	if ftcErr != nil {
		logger.Info("dpo_fine_tune_client_unavailable", map[string]any{"reason": ftcErr.Error()})
	}
	if ftcErr == nil {
		deps.DPOService = dpopkg.NewService(dpoRepo, ftc)
	}
	deps.ScoreStore = ragevalpkg.NewScoreStore("")
	deps.FeatureFlagService = featureflagspkg.NewService()
	logger.Info("dpo_pipeline_initialized", map[string]any{"enabled": ftcErr == nil})

	// Wire plan simulator (world model).
	walletSvc := walletpkg.NewWalletService()
	billingSvc := billingpkg.NewBillingService()
	financeProvider := simulationpkg.NewWalletFinancialProvider(walletSvc, billingSvc)
	calendarProvider := &simulationpkg.NoOpCalendarProvider{}
	deps.Simulator = simulationpkg.NewSimulator(calendarProvider, financeProvider)
	logger.Info("plan_simulator_initialized", map[string]any{"status": "active"})

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

	// Schedule memory decay sweep cron (every 6 hours).
	{
		cronOpts := temporalclient.StartWorkflowOptions{
			ID:           "brevio-memory-decay-sweep-global",
			TaskQueue:    breviotemporal.TaskQueueCore,
			CronSchedule: "0 */6 * * *",
		}
		_, cronErr := temporalClient.ExecuteWorkflow(context.Background(), cronOpts,
			memorypkg.DecaySweepWorkflow, memorypkg.DecaySweepWorkflowInput{WorkspaceID: ""})
		if cronErr != nil {
			logger.Info("decay_cron_schedule_result", map[string]any{"note": cronErr.Error()})
		} else {
			logger.Info("decay_cron_scheduled", map[string]any{"schedule": "0 */6 * * *"})
		}
	}

	// Schedule RAPTOR consolidation cron (nightly at 02:00 UTC).
	{
		cronOpts := temporalclient.StartWorkflowOptions{
			ID:           "brevio-raptor-consolidation-nightly",
			TaskQueue:    breviotemporal.TaskQueueCore,
			CronSchedule: "0 2 * * *",
		}
		_, cronErr := temporalClient.ExecuteWorkflow(context.Background(), cronOpts,
			memorypkg.RaptorConsolidationWorkflow, memorypkg.RaptorConsolidationWorkflowInput{WorkspaceID: ""})
		if cronErr != nil {
			logger.Info("raptor_cron_schedule_result", map[string]any{"note": cronErr.Error()})
		} else {
			logger.Info("raptor_cron_scheduled", map[string]any{"schedule": "0 2 * * *"})
		}
	}

	// Schedule production eval sampler cron (every hour).
	{
		cronOpts := temporalclient.StartWorkflowOptions{
			ID:           "brevio-production-eval-sampler-cron",
			TaskQueue:    breviotemporal.TaskQueueCore,
			CronSchedule: "0 * * * *",
		}
		_, cronErr := temporalClient.ExecuteWorkflow(context.Background(), cronOpts,
			breviotemporal.ProductionEvalSamplerWorkflow)
		if cronErr != nil {
			logger.Info("production_eval_cron_schedule_result", map[string]any{"note": cronErr.Error()})
		} else {
			logger.Info("production_eval_cron_scheduled", map[string]any{"schedule": "0 * * * *"})
		}
	}

	// Schedule proactive monitor cron (every 15 minutes).
	{
		cronOpts := temporalclient.StartWorkflowOptions{
			ID:           "brevio-proactive-monitor-global-cron",
			TaskQueue:    breviotemporal.TaskQueueCore,
			CronSchedule: "*/15 * * * *",
		}
		_, cronErr := temporalClient.ExecuteWorkflow(context.Background(), cronOpts,
			breviotemporal.ProactiveMonitorWorkflow, "global")
		if cronErr != nil {
			logger.Info("proactive_monitor_cron_schedule_result", map[string]any{"note": cronErr.Error()})
		} else {
			logger.Info("proactive_monitor_cron_scheduled", map[string]any{"schedule": "*/15 * * * *"})
		}
	}

	// Schedule DPO nightly cron (03:00 UTC).
	{
		cronOpts := temporalclient.StartWorkflowOptions{
			ID:           "dpo-nightly-cron",
			TaskQueue:    breviotemporal.TaskQueueCore,
			CronSchedule: "0 3 * * *",
		}
		_, cronErr := temporalClient.ExecuteWorkflow(context.Background(), cronOpts,
			breviotemporal.DPORoundWorkflow, dpopkg.DPORoundInput{MinPairCount: dpopkg.MinPairsForDPO})
		if cronErr != nil {
			logger.Info("dpo_cron_schedule_result", map[string]any{"note": cronErr.Error()})
		} else {
			logger.Info("dpo_cron_scheduled", map[string]any{"schedule": "0 3 * * *"})
		}
	}

	// Wire GAIA benchmark.
	deps.BenchmarkRepo = benchmarkpkg.NewRepository(deps.Pool)
	logger.Info("gaia_benchmark_initialized", map[string]any{"status": "active"})

	// Wire browser automation client.
	browserClient := browserpkg.NewClient()
	deps.BrowserClient = browserClient
	deps.BrowserSandboxSvc = sandboxpkg.NewMCPSandboxService()
	logger.Info("browser_client_initialized", map[string]any{"url": os.Getenv("BROWSER_MCP_URL")})

	// Wire delegation service (Phase 4).
	deps.DelegationSvc = delegationpkg.NewService()
	logger.Info("delegation_service_initialized", map[string]any{"status": "active"})

	// Wire trust service for SubAgent autonomy gate.
	deps.TrustSvc = trustpkg.NewService()
	logger.Info("trust_service_initialized", map[string]any{"status": "active"})

	// Wire compliance evidence pool (P3-05), HIPAA pool (P3-06), and eval pool (P3-12).
	if deps.Pool != nil {
		adminpkg.CompliancePool = deps.Pool
		adminpkg.HIPAAPool = deps.Pool
		adminpkg.EvalPool = deps.Pool
		logger.Info("compliance_pool_initialized", map[string]any{"status": "active"})
	}

	// Wire consent middleware (P3-04).
	if deps.Pool != nil {
		consentLogger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))
		consentRegistry := consentpkg.NewConsentRegistry(deps.Pool, consentLogger)
		deps.ConsentMiddleware = consentpkg.NewPurposeLimitationMiddleware(consentRegistry, consentLogger)
		logger.Info("consent_middleware_initialized", map[string]any{"status": "active"})
	}

	// Wire DP accountant (P3-03).
	if deps.Pool != nil {
		dpLogger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))
		deps.DPAccountant = learningpkg.NewRDPAccountant(deps.Pool, dpLogger)
		adminpkg.DPBudgetPool = deps.Pool
		logger.Info("dp_accountant_initialized", map[string]any{"status": "active"})
	}

	// Wire watermarking services (P3-02).
	if wmKeyHex := os.Getenv("WATERMARK_HMAC_KEY"); wmKeyHex != "" {
		wmLogger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))
		c2pa, c2paErr := watermarkpkg.NewC2PAContentWatermarker(wmLogger)
		if c2paErr != nil {
			logger.Info("watermark_c2pa_unavailable", map[string]any{"reason": c2paErr.Error()})
		} else {
			deps.C2PAWatermarker = c2pa
			logger.Info("watermark_c2pa_initialized", map[string]any{"status": "active"})

			if os.Getenv("SEMANTIC_WATERMARK_ENABLED") != "false" {
				semWM, semErr := watermarkpkg.NewSemanticWatermarker(c2pa.HMACKey(), wmLogger)
				if semErr != nil {
					logger.Info("watermark_semantic_unavailable", map[string]any{"reason": semErr.Error()})
				} else {
					deps.SemanticWatermark = semWM
					logger.Info("watermark_semantic_initialized", map[string]any{"status": "active"})
				}
			}

			if deps.Pool != nil {
				deps.ProvenanceStore = watermarkpkg.NewProvenanceStore(deps.Pool, wmLogger)
				adminpkg.ProvenancePool = deps.Pool
				logger.Info("provenance_store_initialized", map[string]any{"status": "active"})
			}
		}
	} else if cfg.Environment != "" && cfg.Environment != "local" && cfg.Environment != "test" {
		logger.Info("watermark_disabled", map[string]any{"reason": "WATERMARK_HMAC_KEY not set"})
	}

	// Wire red-team runner (P3-01).
	if deps.Pool != nil {
		rtLogger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))
		rtGuard := guardrailspkg.NewInferenceGuard()
		rtGuardSvc := guardrailspkg.NewService()
		rtAttackGen, rtErr := redteampkg.NewAttackGenerator(rtGuard, rtGuardSvc, rtLogger)
		if rtErr != nil {
			logger.Info("redteam_attack_generator_unavailable", map[string]any{"reason": rtErr.Error()})
		} else {
			rtHarmBench := redteampkg.NewHarmBenchEvaluator(deps.Pool, rtLogger)
			deps.RedTeamRunner = redteampkg.NewRedTeamRunner(deps.Pool, temporalClient, rtAttackGen, rtHarmBench, rtLogger)
			adminpkg.HarmBenchPool = deps.Pool
			logger.Info("redteam_runner_initialized", map[string]any{"status": "active"})
		}
	}

	// Wire world model repository.
	if deps.Pool != nil {
		wmRepo, wmRepoErr := brainpkg.NewPgWorldModelRepository(deps.Pool)
		if wmRepoErr != nil {
			log.Fatalf("failed to construct WorldModelRepository: %v", wmRepoErr)
		}
		deps.WorldModelRepo = wmRepo
		logger.Info("world_model_repo_initialized", map[string]any{"status": "active"})
	}

	// Wire EU AI Act compliance services (Art. 9, 10, 73).
	if deps.Pool != nil {
		euRR, _ := euaiactpkg.NewRiskRegister(deps.Pool)
		euDG, _ := euaiactpkg.NewDataGovernanceLog(deps.Pool)
		euIL, _ := euaiactpkg.NewIncidentLog(deps.Pool)
		deps.EUAIActRiskRegister = euRR
		deps.EUAIActDataGov = euDG
		deps.EUAIActIncidentLog = euIL
		if euRR != nil && euDG != nil && euIL != nil {
			euCAG, _ := euaiactpkg.NewConformityAssessmentGenerator(deps.Pool, euRR, euIL, euDG)
			deps.EUAIActConformityGenerator = euCAG
		}
		logger.Info("eu_ai_act_services_initialized", map[string]any{"status": "active"})
	}

	// Wire Outcome Reward Model (ORM).
	if anthropicKey := os.Getenv("ANTHROPIC_API_KEY"); anthropicKey != "" {
		ormClient, ormErr := llmpkg.NewAnthropicClient(llmpkg.AnthropicConfig{
			APIKey:  anthropicKey,
			Timeout: 30 * time.Second,
		})
		if ormErr == nil {
			var criticRepo brainpkg.CriticTraceRepository
			deps.ORM = brainpkg.NewOutcomeRewardModel(ormClient, criticRepo)
			logger.Info("orm_initialized", map[string]any{"status": "active", "model": "claude-haiku-4-5"})
		}
	}

	// Wire KG service and retriever (Phase 5).
	if deps.Pool != nil {
		kgLogger := breviotemporal.NewKGLogger()
		kgDB := kgpkg.NewPgxPoolAdapter(deps.Pool)
		kgRepo := kgpkg.NewRepository(kgDB, kgLogger)
		deps.KGRetriever = kgpkg.NewRetriever(kgRepo, nil, kgLogger)
		deps.KGService = kgpkg.NewService(nil, deps.KGRetriever, kgRepo, nil, kgLogger)
		logger.Info("kg_service_initialized", map[string]any{"status": "active"})
	}

	// Schedule GAIA weekly cron (Sunday 23:00 UTC).
	{
		cronOpts := temporalclient.StartWorkflowOptions{
			ID:           "gaia-weekly-cron",
			TaskQueue:    breviotemporal.TaskQueueCore,
			CronSchedule: "0 23 * * 0",
		}
		_, cronErr := temporalClient.ExecuteWorkflow(context.Background(), cronOpts,
			breviotemporal.GAIARunnerWorkflow, benchmarkpkg.GAIARunnerInput{
				DatasetPath: "evals/gaia/brevio_gaia_dataset.json",
				TriggeredBy: "cron",
				ModelVersion: "claude-sonnet-4-6",
			})
		if cronErr != nil {
			logger.Info("gaia_cron_schedule_result", map[string]any{"note": cronErr.Error()})
		} else {
			logger.Info("gaia_cron_scheduled", map[string]any{"schedule": "0 23 * * 0"})
		}
	}

	// Schedule world model expiry sweep cron (every hour).
	{
		cronOpts := temporalclient.StartWorkflowOptions{
			ID:           "brevio-world-model-expiry-cron",
			TaskQueue:    breviotemporal.TaskQueueCore,
			CronSchedule: "0 * * * *",
		}
		_, cronErr := temporalClient.ExecuteWorkflow(context.Background(), cronOpts,
			breviotemporal.WorldModelExpiryCronWorkflow)
		if cronErr != nil {
			logger.Info("world_model_cron_schedule_result", map[string]any{"note": cronErr.Error()})
		} else {
			logger.Info("world_model_cron_scheduled", map[string]any{"schedule": "0 * * * *"})
		}
	}

	// Schedule EU AI Act compliance cron (daily at 02:00 UTC).
	{
		cronOpts := temporalclient.StartWorkflowOptions{
			ID:           breviotemporal.EUAIActComplianceWorkflowID(),
			TaskQueue:    breviotemporal.TaskQueueCore,
			CronSchedule: breviotemporal.EUAIActCronSchedule,
		}
		_, cronErr := temporalClient.ExecuteWorkflow(context.Background(), cronOpts,
			breviotemporal.EUAIActComplianceWorkflow)
		if cronErr != nil {
			logger.Info("eu_ai_act_cron_schedule_result", map[string]any{"note": cronErr.Error()})
		} else {
			logger.Info("eu_ai_act_cron_scheduled", map[string]any{"schedule": breviotemporal.EUAIActCronSchedule})
		}
	}

	// Schedule red-team weekly cron (Sunday 02:00 UTC) — P3-01.
	if deps.RedTeamRunner != nil {
		if cronErr := redteampkg.ScheduleRedTeamCron(temporalClient, breviotemporal.TaskQueueCore); cronErr != nil {
			logger.Info("redteam_cron_schedule_result", map[string]any{"note": cronErr.Error()})
		} else {
			logger.Info("redteam_cron_scheduled", map[string]any{"schedule": redteampkg.RedTeamCronSchedule})
		}
	}

	// Schedule compliance evidence daily cron (03:00 UTC) — P3-05.
	{
		if cronErr := soc2pkg.ScheduleComplianceCron(temporalClient, breviotemporal.TaskQueueCore); cronErr != nil {
			logger.Info("compliance_cron_schedule_result", map[string]any{"note": cronErr.Error()})
		} else {
			logger.Info("compliance_cron_scheduled", map[string]any{"schedule": soc2pkg.ComplianceCronSchedule})
		}
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

// wmLogAdapter adapts runtimeserver.JSONLogger to workingmemory.Logger.
type wmLogAdapter struct {
	logger *runtimeserver.JSONLogger
}

func (a *wmLogAdapter) Info(msg string, args ...any) {
	a.logger.Info(msg, map[string]any{"args": fmt.Sprint(args...)})
}
func (a *wmLogAdapter) Warn(msg string, args ...any) {
	a.logger.Info("WARN: "+msg, map[string]any{"args": fmt.Sprint(args...)})
}
func (a *wmLogAdapter) Error(msg string, args ...any) {
	a.logger.Info("ERROR: "+msg, map[string]any{"args": fmt.Sprint(args...)})
}
