package main

import (
	"log"
	"os"

	"github.com/brevio/brevio/internal/gateway"
	runtimeserver "github.com/brevio/brevio/internal/runtime"
)

func main() {
	secret := os.Getenv("GATEWAY_WEBHOOK_SECRET")
	if secret == "" {
		secret = "dev-secret"
	}

	service := gateway.NewService(secret)
	mux := gateway.NewMux(service)

	addr := ":18080"
	log.Printf("BREVIO gateway listening on %s", addr)
	if err := runtimeserver.ServeWithGracefulShutdown("gateway", addr, mux); err != nil {
		log.Fatalf("gateway server failed: %v", err)
	}
}
