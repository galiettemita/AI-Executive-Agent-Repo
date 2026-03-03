package main

import (
	"log"
	"os"

	"github.com/brevio/brevio/internal/canvas"
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

	log.Printf("BREVIO canvas listening on %s env=%s version=%s", cfg.ListenAddr, cfg.Environment, cfg.ServiceVersion)
	if err := runtimeserver.ServeWithGracefulShutdown("canvas", cfg.ListenAddr, mux); err != nil {
		log.Fatalf("canvas server failed: %v", err)
	}
}
