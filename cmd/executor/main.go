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

	"github.com/brevio/brevio/internal/control"
	"github.com/brevio/brevio/internal/disclosure"
	"github.com/brevio/brevio/internal/executor"
	callpkg "github.com/brevio/brevio/internal/hands/call"
	"github.com/brevio/brevio/internal/metrics"
	runtimeserver "github.com/brevio/brevio/internal/runtime"
	"github.com/brevio/brevio/internal/security"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	cfg, err := runtimeserver.LoadServiceEnvConfig(os.Getenv, runtimeserver.ServiceEnvOptions{
		ServiceName:         "executor",
		DefaultListenAddr:   ":18083",
		RequiredNonLocalEnv: []string{"DATABASE_URL", "REDIS_URL", "TEMPORAL_HOST", "HMAC_KEY"},
	})
	if err != nil {
		log.Fatalf("executor config validation failed: %v", err)
	}

	logger := runtimeserver.NewJSONLogger("executor", cfg.Environment)
	logger.SetOutput(os.Stdout)

	// Inject AI disclosure transport globally — all outbound HTTP carries X-Brevio-Agent: true.
	http.DefaultTransport = disclosure.NewBrevioAgentTransport(http.DefaultTransport)

	// Build production executor when DATABASE_URL is available.
	dbURL := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	var prodSvc *executor.ProdService
	var callRepo callpkg.CallRepository

	if dbURL != "" {
		ctx := context.Background()
		pool, poolErr := pgxpool.New(ctx, dbURL)
		if poolErr != nil {
			log.Fatalf("failed to create pgx pool: %v", poolErr)
		}
		defer pool.Close()

		repo := executor.NewPgToolExecutionRepository(pool)
		receiptRepo := control.NewPgReceiptRepository(pool)

		hmacKey := []byte(os.Getenv("HMAC_KEY"))
		if len(hmacKey) == 0 {
			// REPAIR resolved: HMAC_KEY is now in RequiredNonLocalEnv — startup fails above
			// if unset in non-local environments. This fallback is only reachable in local/test.
			hmacKey = []byte("executor-local-dev-key-not-for-production")
		} else if len(hmacKey) < 32 {
			if cfg.Environment != "local" && cfg.Environment != "test" {
				log.Fatalf(
					"HMAC_KEY is too short in %s environment: got %d bytes, minimum is 32 bytes (256 bits). "+
						"Generate a new key with: openssl rand -hex 32",
					cfg.Environment, len(hmacKey))
			}
			logger.Info("executor_hmac_key_warning", map[string]any{
				"warning": "HMAC_KEY is shorter than 32 bytes — acceptable in local/test only",
				"length":  len(hmacKey),
			})
		}
		receiptSvc := control.NewReceiptService(hmacKey)
		durableReceipts := control.NewDurableReceiptService(receiptSvc, receiptRepo)

		prodSvc = executor.NewProdService(repo, durableReceipts)
		callRepo = callpkg.NewPgCallRepository(pool)

		logger.Info("executor_production_deps", map[string]any{
			"database":  "pgxpool",
			"receipts":  "durable",
			"executor":  "persistent",
			"call_repo": "pgx",
		})
	} else {
		logger.Info("executor_devtest_mode", map[string]any{
			"executor": "in-memory",
		})
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
				"database": boolToStatus(dbURL != ""),
			},
		})
	})
	mux.HandleFunc("GET /health/deep", func(w http.ResponseWriter, _ *http.Request) {
		checks := map[string]string{
			"process":  "ok",
			"database": boolToStatus(dbURL != ""),
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
	mux.Handle("GET /metrics", metrics.Handler())

	// T11.2: Tool execution endpoint — simulate or commit tool execution via ProdService.
	mux.HandleFunc("POST /v1/executor/tool/execute", func(w http.ResponseWriter, r *http.Request) {
		if prodSvc == nil {
			writeJSONError(w, http.StatusServiceUnavailable, "executor not configured (no DATABASE_URL)")
			return
		}

		var req struct {
			WorkspaceID       string `json:"workspace_id"`
			ToolKey           string `json:"tool_key"`
			Action            string `json:"action"`
			Provider          string `json:"provider,omitempty"`
			TargetURL         string `json:"target_url,omitempty"`
			IsMCP             bool   `json:"is_mcp,omitempty"`
			MCPServerID       string `json:"mcp_server_id,omitempty"`
			ContentProvenance string `json:"content_provenance,omitempty"`
			PIIContent        bool   `json:"pii_content,omitempty"`
			Phase             string `json:"phase"`
			ReceiptID         string `json:"receipt_id,omitempty"`
		}
		if err := readJSON(r, &req); err != nil {
			writeJSONError(w, http.StatusBadRequest, fmt.Sprintf("invalid request: %v", err))
			return
		}

		execReq := executor.ExecutionRequest{
			WorkspaceID:       req.WorkspaceID,
			ToolKey:           req.ToolKey,
			Action:            req.Action,
			Provider:          req.Provider,
			TargetURL:         req.TargetURL,
			IsMCP:             req.IsMCP,
			MCPServerID:       req.MCPServerID,
			ContentProvenance: req.ContentProvenance,
			PIIContent:        req.PIIContent,
		}

		switch req.Phase {
		case "simulate":
			exec, err := prodSvc.Simulate(r.Context(), execReq)
			if err != nil {
				writeJSONError(w, http.StatusUnprocessableEntity, err.Error())
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"execution_id":      exec.ID.String(),
				"phase":             string(exec.Phase),
				"idempotency_key":   exec.IdempotencyKey,
				"content_provenance": exec.ContentProvenance,
			})

		case "commit":
			if req.ReceiptID == "" {
				writeJSONError(w, http.StatusBadRequest, "receipt_id is required for commit phase")
				return
			}
			exec, receipt, err := prodSvc.Commit(r.Context(), execReq, req.ReceiptID)
			if err != nil {
				writeJSONError(w, http.StatusUnprocessableEntity, err.Error())
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"execution_id":      exec.ID.String(),
				"phase":             string(exec.Phase),
				"trust_receipt_id":  receipt.ID.String(),
				"idempotency_key":   exec.IdempotencyKey,
				"content_provenance": exec.ContentProvenance,
			})

		default:
			writeJSONError(w, http.StatusBadRequest, "phase must be 'simulate' or 'commit'")
		}
	})

	// T11.2: Call approval request endpoint.
	mux.HandleFunc("POST /v1/executor/call/approve", func(w http.ResponseWriter, r *http.Request) {
		if callRepo == nil {
			writeJSONError(w, http.StatusServiceUnavailable, "call subsystem not configured")
			return
		}

		var req struct {
			ApprovalID string `json:"approval_id"`
			Decision   string `json:"decision"` // approve, deny
			DecidedBy  string `json:"decided_by"`
			Reason     string `json:"reason,omitempty"`
		}
		if err := readJSON(r, &req); err != nil {
			writeJSONError(w, http.StatusBadRequest, fmt.Sprintf("invalid request: %v", err))
			return
		}

		approvalSvc := callpkg.NewApprovalService(callRepo)
		switch req.Decision {
		case "approve":
			if err := approvalSvc.Approve(r.Context(), req.ApprovalID, req.DecidedBy, req.Reason); err != nil {
				writeJSONError(w, http.StatusUnprocessableEntity, err.Error())
				return
			}
		case "deny":
			if err := approvalSvc.Deny(r.Context(), req.ApprovalID, req.DecidedBy, req.Reason); err != nil {
				writeJSONError(w, http.StatusUnprocessableEntity, err.Error())
				return
			}
		default:
			writeJSONError(w, http.StatusBadRequest, "decision must be 'approve' or 'deny'")
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"approval_id": req.ApprovalID,
			"decision":    req.Decision,
			"status":      "processed",
		})
	})

	// T11.2: Get call by ID.
	mux.HandleFunc("GET /v1/executor/call/{id}", func(w http.ResponseWriter, r *http.Request) {
		if callRepo == nil {
			writeJSONError(w, http.StatusServiceUnavailable, "call subsystem not configured")
			return
		}

		callID := r.PathValue("id")
		if callID == "" {
			writeJSONError(w, http.StatusBadRequest, "call id required")
			return
		}

		callRow, err := callRepo.GetCall(r.Context(), callID)
		if err != nil {
			writeJSONError(w, http.StatusNotFound, fmt.Sprintf("call not found: %v", err))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":                  callRow.ID,
			"workspace_id":       callRow.WorkspaceID,
			"status":             callRow.Status,
			"direction":          callRow.Direction,
			"provider_call_id":   callRow.ProviderCallID,
			"duration_seconds":   callRow.DurationSeconds,
			"failover_count":     callRow.FailoverCount,
		})
	})

	// T11.2: List calls for a workspace.
	mux.HandleFunc("GET /v1/executor/calls", func(w http.ResponseWriter, r *http.Request) {
		if callRepo == nil {
			writeJSONError(w, http.StatusServiceUnavailable, "call subsystem not configured")
			return
		}

		workspaceID := r.URL.Query().Get("workspace_id")
		if workspaceID == "" {
			writeJSONError(w, http.StatusBadRequest, "workspace_id query parameter required")
			return
		}

		calls, err := callRepo.ListCalls(r.Context(), workspaceID, 50)
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, fmt.Sprintf("list calls: %v", err))
			return
		}

		items := make([]map[string]any, 0, len(calls))
		for _, c := range calls {
			items = append(items, map[string]any{
				"id":              c.ID,
				"status":          c.Status,
				"direction":       c.Direction,
				"duration_seconds": c.DurationSeconds,
			})
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{"calls": items})
	})

	// T11.2: Get call transcript segments.
	mux.HandleFunc("GET /v1/executor/call/{id}/transcript", func(w http.ResponseWriter, r *http.Request) {
		if callRepo == nil {
			writeJSONError(w, http.StatusServiceUnavailable, "call subsystem not configured")
			return
		}

		callID := r.PathValue("id")
		if callID == "" {
			writeJSONError(w, http.StatusBadRequest, "call id required")
			return
		}

		segments, err := callRepo.GetTranscriptSegments(r.Context(), callID)
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, fmt.Sprintf("get transcript: %v", err))
			return
		}

		items := make([]map[string]any, 0, len(segments))
		for _, s := range segments {
			items = append(items, map[string]any{
				"segment_index": s.SegmentIndex,
				"segment_type":  s.SegmentType,
				"speaker":       s.Speaker,
				"content":       s.Content,
				"started_at_ms": s.StartedAtMs,
				"duration_ms":   s.DurationMs,
				"confidence":    s.Confidence,
				"language":      s.Language,
			})
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{"segments": items})
	})

	// Executor gRPC server — runs alongside HTTP.
	grpcAddr := os.Getenv("EXECUTOR_GRPC_ADDR")
	if grpcAddr == "" {
		grpcAddr = ":50051"
	}

	var mtlsCfg *security.MTLSConfig
	certFile := os.Getenv("MTLS_CERT_FILE")
	keyFile := os.Getenv("MTLS_KEY_FILE")
	caFile := os.Getenv("MTLS_CA_FILE")
	if certFile != "" && keyFile != "" && caFile != "" {
		mtlsCfg = &security.MTLSConfig{
			CertFile: certFile,
			KeyFile:  keyFile,
			CAFile:   caFile,
		}
	}

	// Create a dispatcher adapter for the gRPC server.
	grpcDispatcher := &grpcDispatchAdapter{prodSvc: prodSvc}
	grpcSrv := executor.NewGRPCServer(grpcDispatcher, mtlsCfg, cfg.ServiceVersion)
	go func() {
		logger.Info("executor_grpc_start", map[string]any{"addr": grpcAddr})
		if grpcErr := grpcSrv.ListenAndServe(grpcAddr); grpcErr != nil {
			logger.Info("executor_grpc_stopped", map[string]any{"error": grpcErr.Error()})
		}
	}()

	handler := logger.Middleware(mux)

	logger.Info("service_start", map[string]any{
		"listen_addr":  cfg.ListenAddr,
		"grpc_addr":    grpcAddr,
		"version":      cfg.ServiceVersion,
		"production":   dbURL != "",
	})
	if err := runtimeserver.ServeWithGracefulShutdown("executor", cfg.ListenAddr, handler); err != nil {
		log.Fatalf("executor server failed: %v", err)
	}
}

func readJSON(r *http.Request, v any) error {
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		return err
	}
	return json.Unmarshal(body, v)
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

// grpcDispatchAdapter wraps ProdService to implement executor.DispatcherIface.
type grpcDispatchAdapter struct {
	prodSvc *executor.ProdService
}

func (a *grpcDispatchAdapter) Dispatch(ctx context.Context, req executor.DispatchRequest) (*executor.DispatchResult, error) {
	if a.prodSvc == nil {
		return &executor.DispatchResult{
			ToolKey:      req.ToolKey,
			Phase:        "commit",
			Success:      false,
			ErrorMessage: "executor not configured (no DATABASE_URL)",
		}, nil
	}

	execReq := executor.ExecutionRequest{
		WorkspaceID: req.WorkspaceID,
		ToolKey:     req.ToolKey,
		Action:      "execute",
	}

	exec, _, err := a.prodSvc.Commit(ctx, execReq, req.ReceiptID)
	if err != nil {
		return &executor.DispatchResult{
			ToolKey:      req.ToolKey,
			Phase:        "commit",
			Success:      false,
			ErrorMessage: err.Error(),
		}, nil
	}

	return &executor.DispatchResult{
		ExecutionID: exec.ID.String(),
		ToolKey:     req.ToolKey,
		Phase:       string(exec.Phase),
		Success:     true,
		ExecutedAt:  time.Now(),
	}, nil
}
