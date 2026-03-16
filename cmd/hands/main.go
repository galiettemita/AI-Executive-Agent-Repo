package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/brevio/brevio/internal/connectors"
	"github.com/brevio/brevio/internal/hands"
	"github.com/brevio/brevio/internal/metrics"
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
	startedAt := time.Now().UTC()
	mux.HandleFunc("GET /health/deep", func(w http.ResponseWriter, _ *http.Request) {
		checks := runtimeserver.DeepDependencyChecks(os.Getenv)
		overall := runtimeserver.OverallStatus(checks)
		httpStatus := http.StatusOK
		if overall != "healthy" {
			httpStatus = http.StatusServiceUnavailable
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(httpStatus)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status":    overall,
			"version":   "v1.0.0",
			"service":   "hands",
			"checks":    checks,
			"uptime_ms": time.Since(startedAt).Milliseconds(),
		})
	})
	mux.Handle("GET /metrics", metrics.Handler())

	logger := runtimeserver.NewJSONLogger("hands", env)
	logger.SetOutput(os.Stdout)
	handler := logger.Middleware(mux)

	// Start Hands gRPC server alongside HTTP.
	handsGRPCAddr := os.Getenv("HANDS_GRPC_ADDR")
	if handsGRPCAddr == "" {
		handsGRPCAddr = ":50052"
	}
	handsRuntimeURL := os.Getenv("HANDS_RUNTIME_URL")
	if handsRuntimeURL == "" {
		handsRuntimeURL = fmt.Sprintf("http://localhost:%s", port)
	}
	handsSrv := hands.NewHandsGRPCServer(handsRuntimeURL, nil, "v1.0.0")
	go func() {
		logger.Info("hands_grpc_start", map[string]any{"addr": handsGRPCAddr})
		if grpcErr := handsSrv.ListenAndServe(handsGRPCAddr); grpcErr != nil {
			logger.Info("hands_grpc_stopped", map[string]any{"error": grpcErr.Error()})
		}
	}()

	addr := fmt.Sprintf(":%s", port)
	log.Printf("brevio-hands starting on %s (gRPC on %s, %d skills loaded)", addr, handsGRPCAddr, len(svc.ListSkills()))
	if err := http.ListenAndServe(addr, handler); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
