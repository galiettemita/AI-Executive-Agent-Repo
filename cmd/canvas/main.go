package main

import (
	"log"
	"net/http"

	"github.com/brevio/brevio/internal/canvas"
)

func main() {
	injector := &canvas.InMemoryInjector{}
	service := canvas.NewService(injector)
	mux := canvas.NewMux(service)

	addr := ":18793"
	log.Printf("BREVIO canvas listening on %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("canvas server failed: %v", err)
	}
}
