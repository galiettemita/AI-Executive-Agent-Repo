package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/brevio/brevio/internal/gateway"
	runtimeserver "github.com/brevio/brevio/internal/runtime"
	brtemporal "github.com/brevio/brevio/internal/temporal"
	"go.temporal.io/sdk/client"
)

func main() {
	cfg, err := runtimeserver.LoadServiceEnvConfig(os.Getenv, runtimeserver.ServiceEnvOptions{
		ServiceName:         "brain",
		DefaultListenAddr:   ":18081",
		RequiredNonLocalEnv: []string{"DATABASE_URL", "REDIS_URL", "TEMPORAL_HOST"},
	})
	if err != nil {
		log.Fatalf("brain config validation failed: %v", err)
	}

	logger := runtimeserver.NewJSONLogger("brain", cfg.Environment)
	logger.SetOutput(os.Stdout)

	// Connect to Temporal for workflow dispatch.
	var temporalClient client.Client
	temporalHost := strings.TrimSpace(os.Getenv("TEMPORAL_HOST"))
	if temporalHost != "" {
		tc, tcErr := client.Dial(client.Options{HostPort: temporalHost})
		if tcErr != nil {
			logger.Info("temporal_unavailable", map[string]any{"error": tcErr.Error()})
		} else {
			temporalClient = tc
			defer tc.Close()
		}
	}

	mux := http.NewServeMux()
	startedAt := time.Now().UTC()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status":    "healthy",
			"version":   cfg.ServiceVersion,
			"uptime_ms": time.Since(startedAt).Milliseconds(),
			"checks": map[string]string{
				"process":  "ok",
				"temporal": boolToStatus(temporalClient != nil),
			},
		})
	})
	mux.HandleFunc("GET /health/deep", func(w http.ResponseWriter, _ *http.Request) {
		checks := map[string]string{
			"process":  "ok",
			"temporal": boolToStatus(temporalClient != nil),
		}
		for key, status := range runtimeserver.DeepDependencyChecks(os.Getenv) {
			checks[key] = status
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status":    "healthy",
			"version":   cfg.ServiceVersion,
			"uptime_ms": time.Since(startedAt).Milliseconds(),
			"checks":    checks,
		})
	})
	mux.HandleFunc("GET /healthz/ready", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("GET /healthz/live", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	// T11.1: Brain ingress — accept inbound message envelope, start MessageProcessingWorkflow.
	// Idempotency: uses message ID as Temporal workflow ID to prevent duplicate processing.
	mux.HandleFunc("POST /v1/brain/ingest", func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20)) // 1 MiB max
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, "failed to read body")
			return
		}

		envelope, err := gateway.DecodeMessageEnvelope(body)
		if err != nil {
			writeJSONError(w, http.StatusUnprocessableEntity, fmt.Sprintf("invalid envelope: %v", err))
			return
		}

		workspaceID := r.URL.Query().Get("workspace_id")
		if workspaceID == "" {
			workspaceID = "default"
		}

		if temporalClient == nil {
			writeJSONError(w, http.StatusServiceUnavailable, "temporal not connected")
			return
		}

		wfInput := brtemporal.MessageProcessingWorkflowInput{
			MessageID:      envelope.ID,
			WorkspaceID:    workspaceID,
			ChannelType:    envelope.Channel,
			RawPayload:     string(body),
			IdempotencyKey: envelope.ID,
		}

		// Use message ID as workflow ID for idempotency.
		workflowID := fmt.Sprintf("msg-%s", envelope.ID)
		opts := client.StartWorkflowOptions{
			ID:        workflowID,
			TaskQueue: "brevio-main",
		}

		run, err := temporalClient.ExecuteWorkflow(context.Background(), opts, brtemporal.MessageProcessingWorkflow, wfInput)
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, fmt.Sprintf("workflow start failed: %v", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status":      "accepted",
			"message_id":  envelope.ID,
			"workflow_id": run.GetID(),
			"run_id":      run.GetRunID(),
		})
	})

	handler := logger.Middleware(mux)

	logger.Info("service_start", map[string]any{
		"listen_addr": cfg.ListenAddr,
		"version":     cfg.ServiceVersion,
		"temporal":    temporalClient != nil,
	})
	if err := runtimeserver.ServeWithGracefulShutdown("brain", cfg.ListenAddr, handler); err != nil {
		log.Fatalf("brain server failed: %v", err)
	}
}

func writeJSONError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
}

func boolToStatus(b bool) string {
	if b {
		return "connected"
	}
	return "unavailable"
}
