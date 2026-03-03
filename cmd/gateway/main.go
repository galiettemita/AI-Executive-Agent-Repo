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

	log.Printf("BREVIO gateway listening on %s env=%s version=%s", cfg.ListenAddr, cfg.Environment, cfg.ServiceVersion)
	if err := runtimeserver.ServeWithGracefulShutdown("gateway", cfg.ListenAddr, mux); err != nil {
		log.Fatalf("gateway server failed: %v", err)
	}
}
