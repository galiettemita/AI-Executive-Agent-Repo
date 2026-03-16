package hands

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	handsv1 "github.com/brevio/brevio/proto/hands/v1"
	"github.com/brevio/brevio/internal/security"
)

// HandsGRPCServer implements handsv1.HandsServiceServer. It proxies skill
// execution calls to the TypeScript hands-runtime HTTP server.
type HandsGRPCServer struct {
	handsv1.UnimplementedHandsServiceServer
	runtimeURL string
	httpClient *http.Client
	certConfig *security.MTLSConfig
	version    string
}

// NewHandsGRPCServer creates the Hands gRPC server. runtimeURL is the address
// of the TypeScript hands-runtime HTTP process.
func NewHandsGRPCServer(runtimeURL string, certConfig *security.MTLSConfig, version string) *HandsGRPCServer {
	return &HandsGRPCServer{
		runtimeURL: strings.TrimRight(runtimeURL, "/"),
		httpClient: &http.Client{Timeout: 60 * time.Second},
		certConfig: certConfig,
		version:    version,
	}
}

// ListSkills queries the hands-runtime for all registered skill descriptors.
func (s *HandsGRPCServer) ListSkills(ctx context.Context, req *handsv1.ListSkillsRequest) (*handsv1.ListSkillsResponse, error) {
	url := s.runtimeURL + "/v1/skills"
	if req.GetWorkspaceId() != "" {
		url += "?workspace_id=" + req.GetWorkspaceId()
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "build list skills request: %v", err)
	}
	resp, err := s.httpClient.Do(httpReq)
	if err != nil {
		return nil, status.Errorf(codes.Unavailable, "hands runtime unreachable: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, status.Errorf(codes.Internal, "hands runtime returned status %d", resp.StatusCode)
	}

	var skills []struct {
		ID             string   `json:"id"`
		Name           string   `json:"name"`
		Description    string   `json:"description"`
		Category       string   `json:"category"`
		RequiredScopes []string `json:"required_scopes"`
		IsEnabled      bool     `json:"is_enabled"`
		Plane          string   `json:"plane"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&skills); err != nil {
		return nil, status.Errorf(codes.Internal, "decode skills response: %v", err)
	}

	descriptors := make([]*handsv1.SkillDescriptor, 0, len(skills))
	for _, sk := range skills {
		descriptors = append(descriptors, &handsv1.SkillDescriptor{
			SkillId:        sk.ID,
			Name:           sk.Name,
			Description:    sk.Description,
			Category:       sk.Category,
			RequiredScopes: sk.RequiredScopes,
			IsEnabled:      sk.IsEnabled,
			Plane:          sk.Plane,
		})
	}
	return &handsv1.ListSkillsResponse{Skills: descriptors}, nil
}

// ExecuteSkill calls the hands-runtime HTTP /v1/execute endpoint.
func (s *HandsGRPCServer) ExecuteSkill(ctx context.Context, req *handsv1.ExecuteSkillRequest) (*handsv1.ExecuteSkillResponse, error) {
	if req.GetWorkspaceId() == "" {
		return nil, status.Error(codes.InvalidArgument, "workspace_id is required")
	}
	if req.GetSkillId() == "" {
		return nil, status.Error(codes.InvalidArgument, "skill_id is required")
	}

	timeout := time.Duration(req.GetTimeoutSeconds()) * time.Second
	if timeout <= 0 || timeout > 5*time.Minute {
		timeout = 30 * time.Second
	}
	callCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	body, err := json.Marshal(map[string]any{
		"skill_id":        req.GetSkillId(),
		"workspace_id":    req.GetWorkspaceId(),
		"input":           json.RawMessage(req.GetInputJson()),
		"oauth_token":     req.GetOauthToken(),
		"idempotency_key": req.GetIdempotencyKey(),
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "marshal execute request: %v", err)
	}

	start := time.Now()
	httpReq, err := http.NewRequestWithContext(callCtx, http.MethodPost,
		s.runtimeURL+"/v1/execute", strings.NewReader(string(body)))
	if err != nil {
		return nil, status.Errorf(codes.Internal, "build execute request: %v", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	httpResp, err := s.httpClient.Do(httpReq)
	if err != nil {
		return nil, status.Errorf(codes.Unavailable, "hands runtime execute failed: %v", err)
	}
	defer httpResp.Body.Close()
	latencyMs := time.Since(start).Milliseconds()

	var result struct {
		Status string          `json:"status"`
		Data   json.RawMessage `json:"data"`
		Error  string          `json:"error"`
	}
	if err := json.NewDecoder(httpResp.Body).Decode(&result); err != nil {
		return nil, status.Errorf(codes.Internal, "decode execute response: %v", err)
	}

	success := result.Status == "SUCCESS" || result.Status == "success"
	outputJSON := string(result.Data)
	if outputJSON == "" || outputJSON == "null" {
		outputJSON = "{}"
	}

	return &handsv1.ExecuteSkillResponse{
		Success:      success,
		OutputJson:   outputJSON,
		ErrorMessage: result.Error,
		LatencyMs:    latencyMs,
		ExecutedAt:   timestamppb.Now(),
	}, nil
}

// CheckSkillHealth calls the hands-runtime health endpoint for a specific skill.
func (s *HandsGRPCServer) CheckSkillHealth(ctx context.Context, req *handsv1.CheckSkillHealthRequest) (*handsv1.CheckSkillHealthResponse, error) {
	if req.GetSkillId() == "" {
		return nil, status.Error(codes.InvalidArgument, "skill_id is required")
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet,
		s.runtimeURL+"/v1/skills/"+req.GetSkillId()+"/health", nil)
	if err != nil {
		return &handsv1.CheckSkillHealthResponse{SkillId: req.GetSkillId(), Status: "unavailable"}, nil
	}
	resp, err := s.httpClient.Do(httpReq)
	if err != nil || resp.StatusCode != http.StatusOK {
		return &handsv1.CheckSkillHealthResponse{SkillId: req.GetSkillId(), Status: "unavailable",
			Message: fmt.Sprintf("runtime unreachable: %v", err)}, nil
	}
	defer resp.Body.Close()
	return &handsv1.CheckSkillHealthResponse{SkillId: req.GetSkillId(), Status: "healthy"}, nil
}

// BuildGRPCServer constructs the underlying grpc.Server.
func (s *HandsGRPCServer) BuildGRPCServer() (*grpc.Server, error) {
	opts := []grpc.ServerOption{
		grpc.ChainUnaryInterceptor(handsRecoveryInterceptor),
		grpc.MaxRecvMsgSize(4 * 1024 * 1024),
		grpc.MaxSendMsgSize(4 * 1024 * 1024),
	}
	if s.certConfig != nil {
		tlsCfg, err := s.certConfig.ServerTLSConfig()
		if err != nil {
			return nil, fmt.Errorf("hands grpc: server tls: %w", err)
		}
		opts = append(opts, grpc.Creds(credentials.NewTLS(tlsCfg)))
	}
	srv := grpc.NewServer(opts...)
	handsv1.RegisterHandsServiceServer(srv, s)
	return srv, nil
}

// ListenAndServe starts the hands gRPC server on addr.
func (s *HandsGRPCServer) ListenAndServe(addr string) error {
	srv, err := s.BuildGRPCServer()
	if err != nil {
		return err
	}
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("hands grpc listen %s: %w", addr, err)
	}
	return srv.Serve(lis)
}

func handsRecoveryInterceptor(ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = status.Errorf(codes.Internal, "recovered: %v", r)
		}
	}()
	return handler(ctx, req)
}
