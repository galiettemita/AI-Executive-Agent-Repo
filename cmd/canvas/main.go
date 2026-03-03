package main

import (
	"log"

	"github.com/brevio/brevio/internal/canvas"
	runtimeserver "github.com/brevio/brevio/internal/runtime"
)

func main() {
	injector := &canvas.InMemoryInjector{}
	service := canvas.NewService(injector)
	mux := canvas.NewMux(service)

	addr := ":18793"
	log.Printf("BREVIO canvas listening on %s", addr)
	if err := runtimeserver.ServeWithGracefulShutdown("canvas", addr, mux); err != nil {
		log.Fatalf("canvas server failed: %v", err)
	}
}
