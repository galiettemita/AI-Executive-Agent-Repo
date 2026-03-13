package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/brevio/brevio/internal/connectors"
	"github.com/brevio/brevio/internal/hands"
	runtimeserver "github.com/brevio/brevio/internal/runtime"
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

	// REPAIR: Load connector registry with real key material in non-local environments.
	env := strings.TrimSpace(os.Getenv("BREVIO_ENV"))
	keyB64 := strings.TrimSpace(os.Getenv("CONNECTORS_MASTER_KEY_B64"))
	var keyMaterial []byte
	if keyB64 != "" {
		keyMaterial = []byte(keyB64)
	} else if env == "" || env == "local" || env == "test" {
		keyMaterial = make([]byte, 32) // zero key acceptable in local/test only
	} else {
		log.Fatalf("CONNECTORS_MASTER_KEY_B64 is required in %s environment", env)
	}
	kp := connectors.NewInMemoryKeyProvider("v0", keyMaterial)
	connSvc := connectors.NewService(kp)
	if err := connSvc.LoadSeedFile(seedPath); err != nil {
		log.Fatalf("Failed to load seed file: %v", err)
	}

	// REPAIR: Reject placeholder MCP URLs in non-local environments.
	if env != "" && env != "local" && env != "test" {
		for _, c := range connSvc.ListConnectors() {
			if strings.Contains(c.MCPServerURL, "unconfigured.local") {
				log.Fatalf("Placeholder MCP URL detected for connector %s in %s environment — configure real MCP servers via MCP_BASE_URL", c.Key, env)
			}
		}
	}

	// Create MCP client.
	mcpClient := hands.NewHTTPMCPClient(30 * time.Second)

	// Create hands service.
	svc := hands.NewService(connSvc, mcpClient)

	mux := http.NewServeMux()
	svc.RegisterRoutes(mux)

	logger := runtimeserver.NewJSONLogger("hands", env)
	logger.SetOutput(os.Stdout)
	handler := logger.Middleware(mux)

	addr := fmt.Sprintf(":%s", port)
	log.Printf("brevio-hands starting on %s (%d skills loaded)", addr, len(svc.ListSkills()))
	if err := http.ListenAndServe(addr, handler); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
