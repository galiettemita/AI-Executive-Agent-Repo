package call

import (
	"context"
	"fmt"
	"testing"
)

// mockProvider implements CallProvider for testing.
type mockProvider struct {
	name       string
	createResp *CallResponse
	createErr  error
	getResp    *CallStatus
	getErr     error
	cancelErr  error
}

func (m *mockProvider) Name() string { return m.name }
func (m *mockProvider) CreateCall(_ context.Context, _ CreateCallRequest) (*CallResponse, error) {
	return m.createResp, m.createErr
}
func (m *mockProvider) GetCall(_ context.Context, _ string) (*CallStatus, error) {
	return m.getResp, m.getErr
}
func (m *mockProvider) CancelCall(_ context.Context, _ string) error {
	return m.cancelErr
}

func TestInitiateCallSuccess(t *testing.T) {
	t.Parallel()

	primary := &mockProvider{
		name:       "test_primary",
		createResp: &CallResponse{CallID: "provider-123", Status: "queued"},
	}
	fallback := &mockProvider{name: "test_fallback"}

	svc := NewCallService(primary, fallback)

	call, err := svc.InitiateCall(context.Background(), "ws1", CallRequest{
		PhoneNumber: "+15551234567",
		CallType:    "reservation",
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if call.Provider != "test_primary" {
		t.Fatalf("expected test_primary, got %s", call.Provider)
	}
	if call.Status != "queued" {
		t.Fatalf("expected queued status, got %s", call.Status)
	}
	if call.PhoneNumber != "+15551234567" {
		t.Fatalf("expected phone number to be set, got %s", call.PhoneNumber)
	}
}

func TestInitiateCallFallback(t *testing.T) {
	t.Parallel()

	primary := &mockProvider{
		name:      "test_primary",
		createErr: fmt.Errorf("primary down"),
	}
	fallback := &mockProvider{
		name:       "test_fallback",
		createResp: &CallResponse{CallID: "fallback-456", Status: "queued"},
	}

	svc := NewCallService(primary, fallback)

	call, err := svc.InitiateCall(context.Background(), "ws1", CallRequest{
		PhoneNumber: "+15551234567",
		CallType:    "appointment",
	})
	if err != nil {
		t.Fatalf("expected fallback to succeed, got %v", err)
	}
	if call.Provider != "test_fallback" {
		t.Fatalf("expected test_fallback provider, got %s", call.Provider)
	}
}

func TestRequireApprovalAndApproveLifecycle(t *testing.T) {
	t.Parallel()

	primary := &mockProvider{name: "p"}
	svc := NewCallService(primary, nil)

	approval, err := svc.RequireApproval("ws1", CallRequest{PhoneNumber: "+15551234567", CallType: "custom"})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if approval.Status != "pending" {
		t.Fatalf("expected pending status, got %s", approval.Status)
	}

	err = svc.ApproveCall(approval.ID.String())
	if err != nil {
		t.Fatalf("expected no error on approve, got %v", err)
	}

	// Approving again should fail.
	err = svc.ApproveCall(approval.ID.String())
	if err == nil {
		t.Fatal("expected error when approving already approved call")
	}

	// Invalid UUID.
	err = svc.ApproveCall("not-a-uuid")
	if err == nil {
		t.Fatal("expected error for invalid UUID")
	}
}

func TestHandleCallCompleted(t *testing.T) {
	t.Parallel()

	primary := &mockProvider{
		name:       "p",
		createResp: &CallResponse{CallID: "prov-1", Status: "queued"},
	}
	svc := NewCallService(primary, nil)

	call, _ := svc.InitiateCall(context.Background(), "ws1", CallRequest{
		PhoneNumber: "+15551234567",
		CallType:    "reservation",
	})

	err := svc.HandleCallCompleted("prov-1", "Hello, your reservation is confirmed.")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	updated, _ := svc.GetCall(call.ID.String())
	if updated.Status != "completed" {
		t.Fatalf("expected completed status, got %s", updated.Status)
	}
	if updated.Transcript == "" {
		t.Fatal("expected transcript to be set")
	}

	// Non-existent provider call.
	err = svc.HandleCallCompleted("nonexistent", "transcript")
	if err == nil {
		t.Fatal("expected error for non-existent provider call")
	}
}

func TestListCalls(t *testing.T) {
	t.Parallel()

	primary := &mockProvider{
		name:       "p",
		createResp: &CallResponse{CallID: "prov-1", Status: "queued"},
	}
	svc := NewCallService(primary, nil)

	svc.InitiateCall(context.Background(), "ws1", CallRequest{PhoneNumber: "+1", CallType: "reservation"})
	primary.createResp = &CallResponse{CallID: "prov-2", Status: "queued"}
	svc.InitiateCall(context.Background(), "ws1", CallRequest{PhoneNumber: "+2", CallType: "appointment"})
	primary.createResp = &CallResponse{CallID: "prov-3", Status: "queued"}
	svc.InitiateCall(context.Background(), "ws2", CallRequest{PhoneNumber: "+3", CallType: "quote"})

	ws1Calls := svc.ListCalls("ws1")
	if len(ws1Calls) != 2 {
		t.Fatalf("expected 2 calls for ws1, got %d", len(ws1Calls))
	}

	ws2Calls := svc.ListCalls("ws2")
	if len(ws2Calls) != 1 {
		t.Fatalf("expected 1 call for ws2, got %d", len(ws2Calls))
	}
}
