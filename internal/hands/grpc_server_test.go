package hands_test

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/brevio/brevio/internal/hands"
	handsv1 "github.com/brevio/brevio/proto/hands/v1"
)

func startHandsTestServer(t *testing.T, runtimeURL string) handsv1.HandsServiceClient {
	t.Helper()
	srv := hands.NewHandsGRPCServer(runtimeURL, nil, "test")
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

	conn, err := grpc.NewClient(lis.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { conn.Close() })
	return handsv1.NewHandsServiceClient(conn)
}

func TestHandsGRPCServer_ListSkills_Success(t *testing.T) {
	t.Parallel()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]map[string]any{
			{"id": "google-calendar", "name": "Google Calendar", "is_enabled": true, "plane": "hands"},
		})
	}))
	defer ts.Close()

	client := startHandsTestServer(t, ts.URL)
	resp, err := client.ListSkills(context.Background(), &handsv1.ListSkillsRequest{})
	if err != nil {
		t.Fatalf("ListSkills: %v", err)
	}
	if len(resp.Skills) != 1 {
		t.Fatalf("expected 1 skill, got %d", len(resp.Skills))
	}
	if resp.Skills[0].SkillId != "google-calendar" {
		t.Errorf("expected google-calendar, got %q", resp.Skills[0].SkillId)
	}
}

func TestHandsGRPCServer_ExecuteSkill_Success(t *testing.T) {
	t.Parallel()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"status": "SUCCESS",
			"data":   map[string]string{"event_id": "evt-1"},
		})
	}))
	defer ts.Close()

	client := startHandsTestServer(t, ts.URL)
	resp, err := client.ExecuteSkill(context.Background(), &handsv1.ExecuteSkillRequest{
		WorkspaceId: "ws-1", SkillId: "google-calendar", InputJson: `{}`,
	})
	if err != nil {
		t.Fatalf("ExecuteSkill: %v", err)
	}
	if !resp.Success {
		t.Error("expected success=true")
	}
}

func TestHandsGRPCServer_ExecuteSkill_RuntimeUnavailable(t *testing.T) {
	t.Parallel()
	client := startHandsTestServer(t, "http://127.0.0.1:1") // nothing listening
	_, err := client.ExecuteSkill(context.Background(), &handsv1.ExecuteSkillRequest{
		WorkspaceId: "ws-1", SkillId: "test", InputJson: `{}`,
	})
	if err == nil {
		t.Fatal("expected error when runtime is unavailable")
	}
}

func TestHandsGRPCServer_CheckSkillHealth_Healthy(t *testing.T) {
	t.Parallel()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	client := startHandsTestServer(t, ts.URL)
	resp, err := client.CheckSkillHealth(context.Background(), &handsv1.CheckSkillHealthRequest{
		SkillId: "google-calendar",
	})
	if err != nil {
		t.Fatalf("CheckSkillHealth: %v", err)
	}
	if resp.Status != "healthy" {
		t.Errorf("expected healthy, got %q", resp.Status)
	}
}

func TestHandsGRPCServer_CheckSkillHealth_Unavailable(t *testing.T) {
	t.Parallel()
	client := startHandsTestServer(t, "http://127.0.0.1:1")
	resp, err := client.CheckSkillHealth(context.Background(), &handsv1.CheckSkillHealthRequest{
		SkillId: "test-skill",
	})
	if err != nil {
		t.Fatalf("CheckSkillHealth: %v", err)
	}
	if resp.Status != "unavailable" {
		t.Errorf("expected unavailable, got %q", resp.Status)
	}
}
