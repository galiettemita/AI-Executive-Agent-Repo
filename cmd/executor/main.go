package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/brevio/brevio/internal/control"
	"github.com/brevio/brevio/internal/executor"
	runtimeserver "github.com/brevio/brevio/internal/runtime"
	"github.com/jackc/pgx/v5/pgxpool"
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

	logger := runtimeserver.NewJSONLogger("executor", cfg.Environment)
	logger.SetOutput(os.Stdout)

	// Build production executor when DATABASE_URL is available.
	dbURL := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	var prodSvc *executor.ProdService

	if dbURL != "" {
		ctx := context.Background()
		pool, poolErr := pgxpool.New(ctx, dbURL)
		if poolErr != nil {
			log.Fatalf("failed to create pgx pool: %v", poolErr)
		}
		defer pool.Close()

		repo := executor.NewPgToolExecutionRepository(pool)
		receiptRepo := control.NewPgReceiptRepository(pool)

		hmacKey := []byte(os.Getenv("HMAC_KEY"))
		if len(hmacKey) == 0 {
			hmacKey = []byte("executor-default-hmac-key")
		}
		receiptSvc := control.NewReceiptService(hmacKey)
		durableReceipts := control.NewDurableReceiptService(receiptSvc, receiptRepo)

		prodSvc = executor.NewProdService(repo, durableReceipts)

		logger.Info("executor_production_deps", map[string]any{
			"database":  "pgxpool",
			"receipts":  "durable",
			"executor":  "persistent",
		})
	} else {
		logger.Info("executor_devtest_mode", map[string]any{
			"executor": "in-memory",
		})
	}

	_ = prodSvc // used by future route handlers

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
				"database": boolToStatus(dbURL != ""),
			},
		})
	})
	mux.HandleFunc("GET /health/deep", func(w http.ResponseWriter, _ *http.Request) {
		checks := map[string]string{
			"process":  "ok",
			"database": boolToStatus(dbURL != ""),
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
		"production":  dbURL != "",
	})
	if err := runtimeserver.ServeWithGracefulShutdown("executor", cfg.ListenAddr, handler); err != nil {
		log.Fatalf("executor server failed: %v", err)
	}
}

func boolToStatus(b bool) string {
	if b {
		return "connected"
	}
	return "unavailable"
}
