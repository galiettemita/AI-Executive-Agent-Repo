package executor

import (
	"context"
	"fmt"
	"net"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	executorv1 "github.com/brevio/brevio/proto/executor/v1"
	"github.com/brevio/brevio/internal/security"
)

// DispatcherIface is the minimal dispatch interface used by the gRPC server.
type DispatcherIface interface {
	Dispatch(ctx context.Context, req DispatchRequest) (*DispatchResult, error)
}

// DispatchRequest maps to DispatchToolRequest over the wire.
type DispatchRequest struct {
	MessageID      string
	WorkspaceID    string
	ToolKey        string
	ReceiptID      string
	IdempotencyKey string
	InputJSON      string
	OAuthToken     string
	TimeoutSeconds int
}

// DispatchResult maps to DispatchToolResponse over the wire.
type DispatchResult struct {
	ExecutionID  string
	ToolKey      string
	Phase        string
	Success      bool
	PayloadHash  string
	OutputJSON   string
	ErrorMessage string
	ExecutedAt   time.Time
	LatencyMs    int64
}

// GRPCServer implements executorv1.ExecutorServiceServer over mTLS.
type GRPCServer struct {
	executorv1.UnimplementedExecutorServiceServer
	dispatcher DispatcherIface
	version    string
	certConfig *security.MTLSConfig
}

// NewGRPCServer creates the executor gRPC server. certConfig may be nil
// in development (no TLS). In production certConfig must be non-nil.
func NewGRPCServer(dispatcher DispatcherIface, certConfig *security.MTLSConfig, version string) *GRPCServer {
	return &GRPCServer{
		dispatcher: dispatcher,
		certConfig: certConfig,
		version:    version,
	}
}

// DispatchTool implements the synchronous tool dispatch RPC.
func (s *GRPCServer) DispatchTool(ctx context.Context, req *executorv1.DispatchToolRequest) (*executorv1.DispatchToolResponse, error) {
	if req.GetReceiptId() == "" {
		return nil, status.Error(codes.InvalidArgument, "AUTHORIZATION_REQUIRED: receipt_id is required")
	}
	if req.GetWorkspaceId() == "" {
		return nil, status.Error(codes.InvalidArgument, "workspace_id is required")
	}
	if req.GetToolKey() == "" {
		return nil, status.Error(codes.InvalidArgument, "tool_key is required")
	}

	timeout := time.Duration(req.GetTimeoutSeconds()) * time.Second
	if timeout <= 0 || timeout > 5*time.Minute {
		timeout = 30 * time.Second
	}
	callCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	start := time.Now()
	result, err := s.dispatcher.Dispatch(callCtx, DispatchRequest{
		MessageID:      req.GetMessageId(),
		WorkspaceID:    req.GetWorkspaceId(),
		ToolKey:        req.GetToolKey(),
		ReceiptID:      req.GetReceiptId(),
		IdempotencyKey: req.GetIdempotencyKey(),
		InputJSON:      req.GetInputJson(),
		OAuthToken:     req.GetOauthToken(),
		TimeoutSeconds: int(req.GetTimeoutSeconds()),
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "dispatch failed: %v", err)
	}
	result.LatencyMs = time.Since(start).Milliseconds()

	resp := &executorv1.DispatchToolResponse{
		ExecutionId:  result.ExecutionID,
		ToolKey:      result.ToolKey,
		Phase:        result.Phase,
		Success:      result.Success,
		PayloadHash:  result.PayloadHash,
		OutputJson:   result.OutputJSON,
		ErrorMessage: result.ErrorMessage,
		LatencyMs:    result.LatencyMs,
	}
	if !result.ExecutedAt.IsZero() {
		resp.ExecutedAt = timestamppb.New(result.ExecutedAt)
	}
	return resp, nil
}

// DispatchToolStream dispatches synchronously then emits a single completion event.
func (s *GRPCServer) DispatchToolStream(req *executorv1.DispatchToolRequest, stream executorv1.ExecutorService_DispatchToolStreamServer) error {
	resp, err := s.DispatchTool(stream.Context(), req)
	if err != nil {
		return err
	}
	eventType := "completed"
	if !resp.Success {
		eventType = "failed"
	}
	return stream.Send(&executorv1.ExecutionEvent{
		ExecutionId: resp.ExecutionId,
		EventType:   eventType,
		Message:     resp.ErrorMessage,
		Timestamp:   resp.ExecutedAt,
	})
}

// GetExecutionStatus returns "not_found" stub.
func (s *GRPCServer) GetExecutionStatus(_ context.Context, req *executorv1.GetExecutionStatusRequest) (*executorv1.GetExecutionStatusResponse, error) {
	return &executorv1.GetExecutionStatusResponse{
		ExecutionId: req.GetExecutionId(),
		Status:      "not_found",
	}, nil
}

// HealthCheck returns the service health.
func (s *GRPCServer) HealthCheck(_ context.Context, _ *executorv1.HealthCheckRequest) (*executorv1.HealthCheckResponse, error) {
	return &executorv1.HealthCheckResponse{
		Status:  "healthy",
		Version: s.version,
	}, nil
}

// BuildGRPCServer creates a configured grpc.Server with interceptors.
func (s *GRPCServer) BuildGRPCServer() (*grpc.Server, error) {
	opts := []grpc.ServerOption{
		grpc.ChainUnaryInterceptor(
			grpcRecoveryInterceptor,
		),
		grpc.MaxRecvMsgSize(4 * 1024 * 1024),
		grpc.MaxSendMsgSize(4 * 1024 * 1024),
	}

	if s.certConfig != nil {
		tlsCfg, err := s.certConfig.ServerTLSConfig()
		if err != nil {
			return nil, fmt.Errorf("executor grpc: tls config: %w", err)
		}
		opts = append(opts, grpc.Creds(credentials.NewTLS(tlsCfg)))
	}

	srv := grpc.NewServer(opts...)
	executorv1.RegisterExecutorServiceServer(srv, s)
	return srv, nil
}

// ListenAndServe starts the gRPC server on addr.
func (s *GRPCServer) ListenAndServe(addr string) error {
	srv, err := s.BuildGRPCServer()
	if err != nil {
		return err
	}
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("executor grpc: listen %s: %w", addr, err)
	}
	return srv.Serve(lis)
}

func grpcRecoveryInterceptor(ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = status.Errorf(codes.Internal, "recovered from panic: %v", r)
		}
	}()
	return handler(ctx, req)
}
