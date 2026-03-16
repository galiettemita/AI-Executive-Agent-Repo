package executor_test

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/brevio/brevio/internal/executor"
	executorv1 "github.com/brevio/brevio/proto/executor/v1"
)

type mockDispatcher struct {
	result *executor.DispatchResult
	err    error
}

func (m *mockDispatcher) Dispatch(_ context.Context, _ executor.DispatchRequest) (*executor.DispatchResult, error) {
	return m.result, m.err
}

func startTestServer(t *testing.T, dispatcher executor.DispatcherIface) executorv1.ExecutorServiceClient {
	t.Helper()
	srv := executor.NewGRPCServer(dispatcher, nil, "test")
	grpcSrv, err := srv.BuildGRPCServer()
	if err != nil {
		t.Fatalf("build grpc server: %v", err)
	}
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	go grpcSrv.Serve(lis)
	t.Cleanup(func() { grpcSrv.Stop() })

	conn, err := grpc.NewClient(lis.Addr().String(),
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { conn.Close() })
	return executorv1.NewExecutorServiceClient(conn)
}

func TestGRPCServer_DispatchTool_MissingReceipt(t *testing.T) {
	t.Parallel()
	client := startTestServer(t, &mockDispatcher{})
	_, err := client.DispatchTool(context.Background(), &executorv1.DispatchToolRequest{
		WorkspaceId: "ws-1",
		ToolKey:     "google_calendar.read_events",
	})
	if err == nil {
		t.Fatal("expected error for missing receipt_id")
	}
}

func TestGRPCServer_DispatchTool_Success(t *testing.T) {
	t.Parallel()
	mock := &mockDispatcher{result: &executor.DispatchResult{
		ExecutionID: "exec-001",
		ToolKey:     "google_calendar.read_events",
		Success:     true,
		PayloadHash: "abc123",
		OutputJSON:  `{"events":[]}`,
		ExecutedAt:  time.Now(),
	}}
	client := startTestServer(t, mock)
	resp, err := client.DispatchTool(context.Background(), &executorv1.DispatchToolRequest{
		WorkspaceId:    "ws-1",
		ToolKey:        "google_calendar.read_events",
		ReceiptId:      "receipt-001",
		IdempotencyKey: "idem-001",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resp.Success {
		t.Error("expected success=true")
	}
	if resp.ExecutionId != "exec-001" {
		t.Errorf("expected exec-001, got %q", resp.ExecutionId)
	}
}

func TestGRPCServer_DispatchTool_DispatcherError(t *testing.T) {
	t.Parallel()
	mock := &mockDispatcher{err: fmt.Errorf("hands runtime unreachable")}
	client := startTestServer(t, mock)
	_, err := client.DispatchTool(context.Background(), &executorv1.DispatchToolRequest{
		WorkspaceId: "ws-1", ToolKey: "slack.post_message", ReceiptId: "r-1",
	})
	if err == nil {
		t.Fatal("expected error when dispatcher returns error")
	}
}

func TestGRPCServer_DispatchTool_PanicIsRecovered(t *testing.T) {
	t.Parallel()
	client := startTestServer(t, &panicDispatcher{})
	_, err := client.DispatchTool(context.Background(), &executorv1.DispatchToolRequest{
		WorkspaceId: "ws-1", ToolKey: "test.panic", ReceiptId: "r-1",
	})
	if err == nil {
		t.Fatal("expected error after panic recovery")
	}
}

func TestGRPCServer_HealthCheck(t *testing.T) {
	t.Parallel()
	client := startTestServer(t, &mockDispatcher{})
	resp, err := client.HealthCheck(context.Background(), &executorv1.HealthCheckRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Status != "healthy" {
		t.Errorf("expected healthy, got %q", resp.Status)
	}
}

type panicDispatcher struct{}

func (p *panicDispatcher) Dispatch(_ context.Context, _ executor.DispatchRequest) (*executor.DispatchResult, error) {
	panic("test panic")
}
