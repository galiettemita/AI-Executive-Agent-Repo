package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/brevio/brevio/internal/canvas"
	"github.com/brevio/brevio/internal/metrics"
	runtimeserver "github.com/brevio/brevio/internal/runtime"
)

func main() {
	cfg, err := runtimeserver.LoadServiceEnvConfig(os.Getenv, runtimeserver.ServiceEnvOptions{
		ServiceName:       "canvas",
		DefaultListenAddr: ":18793",
	})
	if err != nil {
		log.Fatalf("canvas config validation failed: %v", err)
	}
	injector := &canvas.InMemoryInjector{}
	service := canvas.NewService(injector)
	mux := canvas.NewMux(service)
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
			"service":   "canvas",
			"checks":    checks,
			"uptime_ms": time.Since(startedAt).Milliseconds(),
		})
	})
	mux.Handle("GET /metrics", metrics.Handler())
	logger := runtimeserver.NewJSONLogger("canvas", cfg.Environment)
	logger.SetOutput(os.Stdout)
	handler := logger.Middleware(mux)

	logger.Info("service_start", map[string]any{
		"listen_addr": cfg.ListenAddr,
		"version":     cfg.ServiceVersion,
	})
	if err := runtimeserver.ServeWithGracefulShutdown("canvas", cfg.ListenAddr, handler); err != nil {
		log.Fatalf("canvas server failed: %v", err)
	}
}
