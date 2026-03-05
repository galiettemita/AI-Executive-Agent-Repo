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
