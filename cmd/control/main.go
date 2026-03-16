package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/brevio/brevio/internal/audit"
	"github.com/brevio/brevio/internal/control"
	"github.com/brevio/brevio/internal/exploration"
	"github.com/brevio/brevio/internal/metrics"
	runtimeserver "github.com/brevio/brevio/internal/runtime"
	selfmod "github.com/brevio/brevio/internal/self_modification"
)

func main() {
	cfg, err := runtimeserver.LoadServiceEnvConfig(os.Getenv, runtimeserver.ServiceEnvOptions{
		ServiceName:       "control",
		DefaultListenAddr: ":18082",
	})
	if err != nil {
		log.Fatalf("control config validation failed: %v", err)
	}
	logger := runtimeserver.NewJSONLogger("control", cfg.Environment)
	logger.SetOutput(os.Stdout)
	secret, err := runtimeserver.ResolveSecretWithLocalDefault(os.Getenv, "CONTROL_APP_SECRET", cfg.Environment, "dev-secret")
	if err != nil {
		log.Fatalf("control secret validation failed: %v", err)
	}

	auditSvc := audit.NewService()
	databaseDSN := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	if databaseDSN != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		sink, sinkErr := audit.NewPGSink(ctx, databaseDSN)
		cancel()
		if sinkErr != nil {
			logger.Info("audit_sink_disabled", map[string]any{
				"reason": sinkErr.Error(),
			})
		} else {
			auditSvc = audit.NewService(audit.WithSink(sink))
			defer func() {
				if closeErr := auditSvc.Close(); closeErr != nil {
					logger.Info("audit_sink_close_failed", map[string]any{
						"error": closeErr.Error(),
					})
				}
			}()
			logger.Info("audit_sink_enabled", map[string]any{
				"type": "postgres",
			})
		}
	}

	// Initialize OPA evaluator with optional sidecar client.
	svc := control.NewService(secret)
	evaluator := control.NewOPAEvaluator(svc)

	opaURL := strings.TrimSpace(os.Getenv("OPA_URL"))
	if opaURL != "" {
		opaCfg := control.DefaultOPAClientConfig()
		opaCfg.BaseURL = opaURL
		if v := os.Getenv("OPA_TIMEOUT_MS"); v != "" {
			if ms, err := strconv.Atoi(v); err == nil && ms > 0 {
				opaCfg.Timeout = time.Duration(ms) * time.Millisecond
			}
		}
		opaClient := control.NewOPAClient(opaCfg)
		evaluator.SetOPAClient(opaClient)

		logger.Info("opa_client_enabled", map[string]any{
			"url":     opaURL,
			"timeout": opaCfg.Timeout.String(),
		})
	} else {
		logger.Info("opa_client_disabled", map[string]any{
			"reason": "OPA_URL not set, using embedded gate logic",
		})
	}

	// Load Rego policies (for audit/debugging — actual evaluation via OPA sidecar).
	policiesDir := strings.TrimSpace(os.Getenv("OPA_POLICIES_DIR"))
	if policiesDir == "" {
		policiesDir = "policies"
	}
	if err := evaluator.LoadPolicies(policiesDir); err != nil {
		logger.Info("opa_policies_load_skipped", map[string]any{
			"reason": err.Error(),
		})
	} else {
		logger.Info("opa_policies_loaded", map[string]any{
			"count": evaluator.PolicyCount(),
		})
	}

	mux := control.NewMuxWithDependencies(svc, control.MuxDependencies{
		AuditService: auditSvc,
		OPAEvaluator: evaluator,
	})
	startedAt := time.Now().UTC()
	mux.HandleFunc("GET /health/deep", func(w http.ResponseWriter, _ *http.Request) {
		checks := runtimeserver.DeepDependencyChecks(os.Getenv)
		overall := runtimeserver.OverallStatus(checks)
		httpStatus := http.StatusOK
		if overall != "healthy" {
			httpStatus = http.StatusServiceUnavailable
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(httpStatus)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status":    overall,
			"version":   cfg.ServiceVersion,
			"service":   "control",
			"checks":    checks,
			"uptime_ms": time.Since(startedAt).Milliseconds(),
		})
	})
	mux.Handle("GET /metrics", metrics.Handler())

	// Mount capability exploration routes.
	explorationSvc := exploration.NewService()
	exploration.RegisterRoutes(mux, explorationSvc)
	logger.Info("exploration_routes_mounted", map[string]any{"status": "active"})

	// Mount self-modification policy management routes.
	selfModSvc := selfmod.NewService()
	selfmod.RegisterRoutes(mux, selfModSvc)
	logger.Info("self_modification_routes_mounted", map[string]any{"status": "active"})

	handler := logger.Middleware(mux)

	logger.Info("service_start", map[string]any{
		"listen_addr": cfg.ListenAddr,
		"version":     cfg.ServiceVersion,
		"opa_enabled": evaluator.HasOPAClient(),
	})
	if err := runtimeserver.ServeWithGracefulShutdown("control", cfg.ListenAddr, handler); err != nil {
		log.Fatalf("control server failed: %v", err)
	}
}
