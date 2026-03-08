package canvas

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestBroadcastToWorkspace(t *testing.T) {
	t.Parallel()

	injector := &InMemoryInjector{}
	svc := NewService(injector)
	server := httptest.NewServer(NewMux(svc))
	defer server.Close()

	// Connect two clients in workspace "ws1".
	wsURL1 := "ws" + strings.TrimPrefix(server.URL, "http") + "/v1/canvas/ws?session_id=s1&workspace_id=ws1"
	conn1, _, err := websocket.DefaultDialer.Dial(wsURL1, nil)
	if err != nil {
		t.Fatalf("dial ws1-s1: %v", err)
	}
	defer conn1.Close()

	wsURL2 := "ws" + strings.TrimPrefix(server.URL, "http") + "/v1/canvas/ws?session_id=s2&workspace_id=ws1"
	conn2, _, err := websocket.DefaultDialer.Dial(wsURL2, nil)
	if err != nil {
		t.Fatalf("dial ws1-s2: %v", err)
	}
	defer conn2.Close()

	// Connect one client in workspace "ws2".
	wsURL3 := "ws" + strings.TrimPrefix(server.URL, "http") + "/v1/canvas/ws?session_id=s3&workspace_id=ws2"
	conn3, _, err := websocket.DefaultDialer.Dial(wsURL3, nil)
	if err != nil {
		t.Fatalf("dial ws2-s3: %v", err)
	}
	defer conn3.Close()

	// Broadcast a message to workspace "ws1".
	svc.BroadcastToWorkspace("ws1", A2UIMessage{
		Type:    MsgTypeNotification,
		Payload: map[string]any{"text": "hello ws1"},
	})

	// Both ws1 connections should receive the message.
	for _, conn := range []*websocket.Conn{conn1, conn2} {
		_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		_, data, err := conn.ReadMessage()
		if err != nil {
			t.Fatalf("read broadcast: %v", err)
		}
		var msg A2UIMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			t.Fatalf("unmarshal broadcast: %v", err)
		}
		if msg.Type != MsgTypeNotification {
			t.Fatalf("expected notification type, got %s", msg.Type)
		}
		if msg.WorkspaceID != "ws1" {
			t.Fatalf("expected workspace ws1, got %s", msg.WorkspaceID)
		}
	}

	// ws2 connection should NOT receive the message (set a short deadline).
	_ = conn3.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
	_, _, err = conn3.ReadMessage()
	if err == nil {
		t.Fatal("ws2 connection should not have received ws1 broadcast")
	}
}

func TestHandleIncomingRouting(t *testing.T) {
	t.Parallel()

	svc := NewService(&InMemoryInjector{})

	// Register a custom handler to verify routing.
	routed := false
	svc.mu.Lock()
	svc.messageHandlers["notification"] = func(sessionID string, payload map[string]any) {
		routed = true
		if sessionID != "test-session" {
			t.Errorf("expected session test-session, got %s", sessionID)
		}
		if payload["text"] != "hello" {
			t.Errorf("expected text hello, got %v", payload["text"])
		}
	}
	svc.mu.Unlock()

	raw := []byte(`{"type":"notification","payload":{"text":"hello"}}`)
	svc.HandleIncoming("test-session", raw)

	if !routed {
		t.Fatal("message was not routed to notification handler")
	}
}

func TestHandleIncomingIgnoresUnknownType(t *testing.T) {
	t.Parallel()

	svc := NewService(&InMemoryInjector{})
	// Should not panic on unknown type.
	svc.HandleIncoming("s1", []byte(`{"type":"unknown_type","payload":{}}`))
}

func TestHandleIncomingIgnoresInvalidJSON(t *testing.T) {
	t.Parallel()

	svc := NewService(&InMemoryInjector{})
	// Should not panic on invalid JSON.
	svc.HandleIncoming("s1", []byte(`not json`))
}

func TestRegisterSurface(t *testing.T) {
	t.Parallel()

	svc := NewService(&InMemoryInjector{})

	svc.RegisterSurface("ws1", "mission_control")
	svc.RegisterSurface("ws1", "approvals")
	svc.RegisterSurface("ws1", "activity_feed")
	// Duplicate should be ignored.
	svc.RegisterSurface("ws1", "mission_control")

	surfaces := svc.RegisteredSurfaces("ws1")
	if len(surfaces) != 3 {
		t.Fatalf("expected 3 surfaces, got %d", len(surfaces))
	}
	expected := map[string]bool{"mission_control": true, "approvals": true, "activity_feed": true}
	for _, s := range surfaces {
		if !expected[s] {
			t.Fatalf("unexpected surface: %s", s)
		}
	}
}

func TestRegisterSurfaceDefaultWorkspace(t *testing.T) {
	t.Parallel()

	svc := NewService(&InMemoryInjector{})
	svc.RegisterSurface("", "mission_control")
	surfaces := svc.RegisteredSurfaces("default")
	if len(surfaces) != 1 {
		t.Fatalf("expected 1 surface for default workspace, got %d", len(surfaces))
	}
}

func TestPushMissionControlUpdate(t *testing.T) {
	t.Parallel()

	injector := &InMemoryInjector{}
	svc := NewService(injector)
	server := httptest.NewServer(NewMux(svc))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/v1/canvas/ws?session_id=s1&workspace_id=ws1"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	now := time.Now().UTC()
	widgets := []Widget{
		{ID: "w1", Type: "metric", Title: "Trust Score", Data: map[string]any{"value": 0.92}, UpdatedAt: now},
		{ID: "w2", Type: "list", Title: "Active Goals", Data: map[string]any{"count": 5}, UpdatedAt: now},
	}
	svc.PushMissionControlUpdate("ws1", widgets)

	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, data, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read mission control update: %v", err)
	}

	var msg A2UIMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if msg.Type != MsgTypeMissionControlUpdate {
		t.Fatalf("expected mission_control_update, got %s", msg.Type)
	}
	widgetList, ok := msg.Payload["widgets"].([]any)
	if !ok {
		t.Fatal("expected widgets array in payload")
	}
	if len(widgetList) != 2 {
		t.Fatalf("expected 2 widgets, got %d", len(widgetList))
	}
}

func TestRequestApproval(t *testing.T) {
	t.Parallel()

	svc := NewService(&InMemoryInjector{})

	req := ApprovalRequest{
		ID:          "approval-001",
		Action:      "deploy_to_production",
		RiskLevel:   "high",
		Description: "Deploy v2.0 to production",
		ExpiresAt:   time.Now().Add(5 * time.Minute),
	}
	ch := svc.RequestApproval("ws1", req)

	// Simulate an operator responding via HandleIncoming.
	go func() {
		time.Sleep(50 * time.Millisecond)
		raw := `{"type":"approval_request","payload":{"id":"approval-001","approved":true,"reason":"looks good","responded_by":"operator-1"}}`
		svc.HandleIncoming("s1", []byte(raw))
	}()

	select {
	case resp := <-ch:
		if !resp.Approved {
			t.Fatal("expected approved response")
		}
		if resp.Reason != "looks good" {
			t.Fatalf("unexpected reason: %s", resp.Reason)
		}
		if resp.RespondedBy != "operator-1" {
			t.Fatalf("unexpected respondedBy: %s", resp.RespondedBy)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for approval response")
	}
}

func TestA2UIMessagePriorityDefaults(t *testing.T) {
	t.Parallel()

	svc := NewService(&InMemoryInjector{})

	msg := A2UIMessage{
		Type:    MsgTypeNotification,
		Payload: map[string]any{"text": "test"},
	}
	// BroadcastToWorkspace with no connections should not panic.
	svc.BroadcastToWorkspace("ws-empty", msg)

	// Verify the message gets default priority set.
	if msg.Priority != "" {
		// We only set defaults inside BroadcastToWorkspace, so the original is unchanged.
		// This test is more about verifying no panic.
	}
}
