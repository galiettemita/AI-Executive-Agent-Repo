package main

import (
	"log"
	"net/http"

	"github.com/brevio/brevio/internal/control"
)

func main() {
	mux := control.NewMux(control.NewService("dev-secret"))

	addr := ":18082"
	log.Printf("BREVIO control listening on %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("control server failed: %v", err)
	}
}
