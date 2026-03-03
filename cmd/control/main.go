package main

import (
	"log"

	"github.com/brevio/brevio/internal/control"
	runtimeserver "github.com/brevio/brevio/internal/runtime"
)

func main() {
	mux := control.NewMux(control.NewService("dev-secret"))

	addr := ":18082"
	log.Printf("BREVIO control listening on %s", addr)
	if err := runtimeserver.ServeWithGracefulShutdown("control", addr, mux); err != nil {
		log.Fatalf("control server failed: %v", err)
	}
}
