package main

import (
	"context"
	"log"
	"os"
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

	mux := control.NewMuxWithDependencies(control.NewService(secret), control.MuxDependencies{
		AuditService: auditSvc,
	})
	handler := logger.Middleware(mux)

	logger.Info("service_start", map[string]any{
		"listen_addr": cfg.ListenAddr,
		"version":     cfg.ServiceVersion,
	})
	if err := runtimeserver.ServeWithGracefulShutdown("control", cfg.ListenAddr, handler); err != nil {
		log.Fatalf("control server failed: %v", err)
	}
}
