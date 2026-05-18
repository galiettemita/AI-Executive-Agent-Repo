package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/brevio/brevio/internal/connectors"
	"github.com/brevio/brevio/internal/hands"
)

func main() {
	port := os.Getenv("HANDS_PORT")
	if port == "" {
		port = "18090"
	}

	seedPath := os.Getenv("SEED_FILE")
	if seedPath == "" {
		seedPath = "internal/connectors/seeds/connectors.yaml"
	}

	// Load connector registry.
	kp := connectors.NewInMemoryKeyProvider("v0", make([]byte, 32))
	connSvc := connectors.NewService(kp)
	if err := connSvc.LoadSeedFile(seedPath); err != nil {
		log.Fatalf("Failed to load seed file: %v", err)
	}

	// Create MCP client.
	mcpClient := hands.NewHTTPMCPClient(30 * time.Second)

	// Create hands service.
	svc := hands.NewService(connSvc, mcpClient)

	mux := http.NewServeMux()
	svc.RegisterRoutes(mux)

	addr := fmt.Sprintf(":%s", port)
	log.Printf("brevio-hands starting on %s (%d skills loaded)", addr, len(svc.ListSkills()))
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
