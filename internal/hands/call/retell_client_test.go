package call

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRetellCreateCall(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v2/create-phone-call" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Fatal("expected authorization header")
		}

		resp := map[string]any{
			"call_id":         "retell-call-123",
			"call_status":     "initiated",
			"to_number":       "+15551234567",
			"start_timestamp": 1700000000000,
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewRetellClient("test-key", "phone-id", "https://webhook.example.com", "secret")
	client.baseURL = server.URL

	resp, err := client.CreateCall(context.Background(), CreateCallRequest{
		PhoneNumber:     "+15551234567",
		AssistantPrompt: "Hello",
		FirstMessage:    "Hi",
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if resp.CallID != "retell-call-123" {
		t.Fatalf("expected call ID retell-call-123, got %s", resp.CallID)
	}
}

func TestRetellCreateCallError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"server error"}`))
	}))
	defer server.Close()

	client := NewRetellClient("test-key", "phone-id", "", "")
	client.baseURL = server.URL

	_, err := client.CreateCall(context.Background(), CreateCallRequest{PhoneNumber: "+1"})
	if err == nil {
		t.Fatal("expected error for server error response")
	}
}

func TestRetellGetCall(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"call_id":           "retell-call-123",
			"call_status":      "completed",
			"duration_ms":       60000,
			"disconnect_reason": "agent_hangup",
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewRetellClient("test-key", "phone-id", "", "")
	client.baseURL = server.URL

	status, err := client.GetCall(context.Background(), "retell-call-123")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if status.Status != "completed" {
		t.Fatalf("expected completed, got %s", status.Status)
	}
	if status.Duration != 60 {
		t.Fatalf("expected duration 60s, got %d", status.Duration)
	}
}

func TestRetellCancelCall(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewRetellClient("test-key", "phone-id", "", "")
	client.baseURL = server.URL

	err := client.CancelCall(context.Background(), "call-123")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestRetellName(t *testing.T) {
	t.Parallel()

	client := NewRetellClient("key", "phone", "", "")
	if client.Name() != "retell" {
		t.Fatalf("expected 'retell', got %s", client.Name())
	}
}
