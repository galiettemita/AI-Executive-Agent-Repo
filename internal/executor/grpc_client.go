package executor

import (
	"context"
	"fmt"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"

	executorv1 "github.com/brevio/brevio/proto/executor/v1"
	"github.com/brevio/brevio/internal/security"
)

// GRPCClient is the Brain-side client for the Executor gRPC service.
// All methods are safe for concurrent use.
type GRPCClient struct {
	conn   *grpc.ClientConn
	client executorv1.ExecutorServiceClient
}

// NewGRPCClient dials the Executor gRPC service. certConfig nil = insecure (dev only).
func NewGRPCClient(addr string, certConfig *security.MTLSConfig) (*GRPCClient, error) {
	var dialOpts []grpc.DialOption

	if certConfig != nil {
		tlsCfg, err := certConfig.ClientTLSConfig()
		if err != nil {
			return nil, fmt.Errorf("executor grpc client: tls config: %w", err)
		}
		dialOpts = append(dialOpts, grpc.WithTransportCredentials(credentials.NewTLS(tlsCfg)))
	} else {
		dialOpts = append(dialOpts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}

	dialOpts = append(dialOpts,
		grpc.WithDefaultCallOptions(
			grpc.MaxCallRecvMsgSize(4*1024*1024),
			grpc.MaxCallSendMsgSize(4*1024*1024),
		),
	)

	conn, err := grpc.NewClient(addr, dialOpts...)
	if err != nil {
		return nil, fmt.Errorf("executor grpc client: dial %s: %w", addr, err)
	}
	return &GRPCClient{conn: conn, client: executorv1.NewExecutorServiceClient(conn)}, nil
}

// DispatchTool calls the Executor service synchronously.
func (c *GRPCClient) DispatchTool(ctx context.Context, req DispatchRequest) (*DispatchResult, error) {
	resp, err := c.client.DispatchTool(ctx, &executorv1.DispatchToolRequest{
		MessageId:      req.MessageID,
		WorkspaceId:    req.WorkspaceID,
		ToolKey:        req.ToolKey,
		ReceiptId:      req.ReceiptID,
		IdempotencyKey: req.IdempotencyKey,
		InputJson:      req.InputJSON,
		OauthToken:     req.OAuthToken,
		TimeoutSeconds: int32(req.TimeoutSeconds),
	})
	if err != nil {
		return nil, fmt.Errorf("executor grpc dispatch: %w", err)
	}
	result := &DispatchResult{
		ExecutionID:  resp.GetExecutionId(),
		ToolKey:      resp.GetToolKey(),
		Phase:        resp.GetPhase(),
		Success:      resp.GetSuccess(),
		PayloadHash:  resp.GetPayloadHash(),
		OutputJSON:   resp.GetOutputJson(),
		ErrorMessage: resp.GetErrorMessage(),
		LatencyMs:    resp.GetLatencyMs(),
	}
	if t := resp.GetExecutedAt(); t != nil {
		result.ExecutedAt = t.AsTime()
	}
	return result, nil
}

// Ping checks the service health endpoint.
func (c *GRPCClient) Ping(ctx context.Context) error {
	_, err := c.client.HealthCheck(ctx, &executorv1.HealthCheckRequest{})
	return err
}

// Close releases the underlying connection.
func (c *GRPCClient) Close() error {
	return c.conn.Close()
}
