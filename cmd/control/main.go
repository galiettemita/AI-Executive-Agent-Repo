package main

import (
	"context"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/brevio/brevio/internal/audit"
	"github.com/brevio/brevio/internal/control"
	runtimeserver "github.com/brevio/brevio/internal/runtime"
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
	})
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
