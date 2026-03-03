package main

import (
	"log"
	"os"

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
	secret, err := runtimeserver.ResolveSecretWithLocalDefault(os.Getenv, "CONTROL_APP_SECRET", cfg.Environment, "dev-secret")
	if err != nil {
		log.Fatalf("control secret validation failed: %v", err)
	}
	mux := control.NewMux(control.NewService(secret))
	logger := runtimeserver.NewJSONLogger("control", cfg.Environment)
	logger.SetOutput(os.Stdout)
	handler := logger.Middleware(mux)

	logger.Info("service_start", map[string]any{
		"listen_addr": cfg.ListenAddr,
		"version":     cfg.ServiceVersion,
	})
	if err := runtimeserver.ServeWithGracefulShutdown("control", cfg.ListenAddr, handler); err != nil {
		log.Fatalf("control server failed: %v", err)
	}
}
