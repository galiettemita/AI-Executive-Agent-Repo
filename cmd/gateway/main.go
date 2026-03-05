package main

import (
	"log"
	"os"

	"github.com/brevio/brevio/internal/gateway"
	runtimeserver "github.com/brevio/brevio/internal/runtime"
)

func main() {
	cfg, err := gateway.LoadEnvConfig(os.Getenv)
	if err != nil {
		log.Fatalf("gateway config validation failed: %v", err)
	}

	service := gateway.NewServiceWithOptions(cfg.WebhookSecret, gateway.ServiceOptions{
		IMessageWebhookAPIKey: cfg.IMessageWebhookAPIKey,
	})
	mux := gateway.NewMux(service)
	logger := runtimeserver.NewJSONLogger("gateway", cfg.Environment)
	logger.SetOutput(os.Stdout)
	handler := logger.Middleware(mux)

	logger.Info("service_start", map[string]any{
		"listen_addr": cfg.ListenAddr,
		"version":     cfg.ServiceVersion,
	})
	if err := runtimeserver.ServeWithGracefulShutdown("gateway", cfg.ListenAddr, handler); err != nil {
		log.Fatalf("gateway server failed: %v", err)
	}
}
