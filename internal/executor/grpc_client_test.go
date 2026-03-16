package executor_test

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/brevio/brevio/internal/executor"
)

func TestGRPCClient_RoundTrip(t *testing.T) {
	t.Parallel()
	mock := &mockDispatcher{result: &executor.DispatchResult{
		ExecutionID: "exec-roundtrip",
		Success:     true,
		ExecutedAt:  time.Now(),
	}}
	srv := executor.NewGRPCServer(mock, nil, "test")
	grpcSrv, err := srv.BuildGRPCServer()
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	go grpcSrv.Serve(lis)
	defer grpcSrv.Stop()

	client, err := executor.NewGRPCClient(lis.Addr().String(), nil)
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	defer client.Close()

	result, err := client.DispatchTool(context.Background(), executor.DispatchRequest{
		WorkspaceID: "ws-1", ToolKey: "test.tool", ReceiptID: "r-1",
	})
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	if result.ExecutionID != "exec-roundtrip" {
		t.Errorf("expected exec-roundtrip, got %q", result.ExecutionID)
	}
}

func TestGRPCClient_Ping(t *testing.T) {
	t.Parallel()
	srv := executor.NewGRPCServer(&mockDispatcher{}, nil, "test")
	grpcSrv, _ := srv.BuildGRPCServer()
	lis, _ := net.Listen("tcp", "127.0.0.1:0")
	go grpcSrv.Serve(lis)
	defer grpcSrv.Stop()

	client, _ := executor.NewGRPCClient(lis.Addr().String(), nil)
	defer client.Close()
	if err := client.Ping(context.Background()); err != nil {
		t.Errorf("ping failed: %v", err)
	}
}
