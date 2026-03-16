package hands

import (
	"context"
	"fmt"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"

	handsv1 "github.com/brevio/brevio/proto/hands/v1"
	"github.com/brevio/brevio/internal/security"
)

// HandsGRPCClient is the Brain/Executor-side client for the Hands gRPC service.
type HandsGRPCClient struct {
	conn   *grpc.ClientConn
	client handsv1.HandsServiceClient
}

// NewHandsGRPCClient dials the Hands gRPC service. certConfig nil = insecure.
func NewHandsGRPCClient(addr string, certConfig *security.MTLSConfig) (*HandsGRPCClient, error) {
	var dialOpts []grpc.DialOption
	if certConfig != nil {
		tlsCfg, err := certConfig.ClientTLSConfig()
		if err != nil {
			return nil, fmt.Errorf("hands grpc client: tls: %w", err)
		}
		dialOpts = append(dialOpts, grpc.WithTransportCredentials(credentials.NewTLS(tlsCfg)))
	} else {
		dialOpts = append(dialOpts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}
	conn, err := grpc.NewClient(addr, dialOpts...)
	if err != nil {
		return nil, fmt.Errorf("hands grpc client: dial %s: %w", addr, err)
	}
	return &HandsGRPCClient{conn: conn, client: handsv1.NewHandsServiceClient(conn)}, nil
}

// ExecuteSkill calls the hands skill execution endpoint.
func (c *HandsGRPCClient) ExecuteSkill(ctx context.Context, workspaceID, skillID, inputJSON, oauthToken, idempotencyKey string) (*handsv1.ExecuteSkillResponse, error) {
	return c.client.ExecuteSkill(ctx, &handsv1.ExecuteSkillRequest{
		WorkspaceId:    workspaceID,
		SkillId:        skillID,
		InputJson:      inputJSON,
		OauthToken:     oauthToken,
		IdempotencyKey: idempotencyKey,
		TimeoutSeconds: 30,
	})
}

// Close releases the connection.
func (c *HandsGRPCClient) Close() error { return c.conn.Close() }
