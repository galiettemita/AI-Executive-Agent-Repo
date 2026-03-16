package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/brevio/brevio/internal/gateway"
	"github.com/brevio/brevio/internal/metrics"
	"github.com/brevio/brevio/internal/outbox"
	runtimeserver "github.com/brevio/brevio/internal/runtime"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	cfg, err := gateway.LoadEnvConfig(os.Getenv)
	if err != nil {
		log.Fatalf("gateway config validation failed: %v", err)
	}

	logger := runtimeserver.NewJSONLogger("gateway", cfg.Environment)
	logger.SetOutput(os.Stdout)
	startedAt := time.Now().UTC()

	dbURL := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	if dbURL != "" {
		// Production path: use pgx-backed repositories.
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		pool, err := pgxpool.New(ctx, dbURL)
		if err != nil {
			log.Fatalf("gateway: connect to database: %v", err)
		}
		defer pool.Close()

		outboxSvc := outbox.NewService(pool)

		prodService, err := gateway.NewServiceProd(gateway.ProdDeps{
			DB:            pool,
			Pool:          pool,
			WebhookSecret: cfg.WebhookSecret,
			ServiceOptions: gateway.ServiceOptions{
				IMessageWebhookAPIKey: cfg.IMessageWebhookAPIKey,
			},
			OutboxService: outboxSvc,
		})
		if err != nil {
			log.Fatalf("gateway: create production service: %v", err)
		}

		mux := gateway.NewProdMux(prodService)
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
				"service":   "gateway",
				"checks":    checks,
				"uptime_ms": time.Since(startedAt).Milliseconds(),
			})
		})
		mux.Handle("GET /metrics", metrics.Handler())
		handler := logger.Middleware(mux)

		logger.Info("service_start", map[string]any{
			"listen_addr": cfg.ListenAddr,
			"version":     cfg.ServiceVersion,
			"mode":        "production",
			"database":    "connected",
		})
		if err := runtimeserver.ServeWithGracefulShutdown("gateway", cfg.ListenAddr, handler); err != nil {
			log.Fatalf("gateway server failed: %v", err)
		}
		return
	}

	// Dev/test path: in-memory stores.
	service := gateway.NewServiceWithOptions(cfg.WebhookSecret, gateway.ServiceOptions{
		IMessageWebhookAPIKey: cfg.IMessageWebhookAPIKey,
	})
	mux := gateway.NewMux(service)
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
			"service":   "gateway",
			"checks":    checks,
			"uptime_ms": time.Since(startedAt).Milliseconds(),
		})
	})
	mux.Handle("GET /metrics", metrics.Handler())
	handler := logger.Middleware(mux)

	logger.Info("service_start", map[string]any{
		"listen_addr": cfg.ListenAddr,
		"version":     cfg.ServiceVersion,
		"mode":        "devtest",
	})
	if err := runtimeserver.ServeWithGracefulShutdown("gateway", cfg.ListenAddr, handler); err != nil {
		log.Fatalf("gateway server failed: %v", err)
	}
}
