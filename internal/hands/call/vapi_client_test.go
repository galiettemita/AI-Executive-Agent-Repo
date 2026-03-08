package call

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestVAPICreateCall(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/call" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Fatal("expected authorization header")
		}

		resp := CallResponse{
			CallID:      "vapi-call-456",
			Status:      "queued",
			PhoneNumber: "+15551234567",
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewVAPIClient("test-key", "phone-id", "https://webhook.example.com", "secret")
	client.baseURL = server.URL

	resp, err := client.CreateCall(context.Background(), CreateCallRequest{
		PhoneNumber:     "+15551234567",
		AssistantPrompt: "Hello",
		FirstMessage:    "Hi",
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if resp.CallID != "vapi-call-456" {
		t.Fatalf("expected call ID vapi-call-456, got %s", resp.CallID)
	}
}

func TestVAPICreateCallError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error":"bad request"}`))
	}))
	defer server.Close()

	client := NewVAPIClient("test-key", "phone-id", "", "")
	client.baseURL = server.URL

	_, err := client.CreateCall(context.Background(), CreateCallRequest{PhoneNumber: "+1"})
	if err == nil {
		t.Fatal("expected error for bad request")
	}
}

func TestVAPIGetCall(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		resp := CallStatus{
			CallID:    "vapi-call-456",
			Status:    "in_progress",
			Duration:  30,
			EndReason: "",
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewVAPIClient("test-key", "phone-id", "", "")
	client.baseURL = server.URL

	status, err := client.GetCall(context.Background(), "vapi-call-456")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if status.Status != "in_progress" {
		t.Fatalf("expected in_progress, got %s", status.Status)
	}
}

func TestVAPICancelCall(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Fatalf("expected DELETE, got %s", r.Method)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewVAPIClient("test-key", "phone-id", "", "")
	client.baseURL = server.URL

	err := client.CancelCall(context.Background(), "call-123")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestVAPIName(t *testing.T) {
	t.Parallel()

	client := NewVAPIClient("key", "phone", "", "")
	if client.Name() != "vapi" {
		t.Fatalf("expected 'vapi', got %s", client.Name())
	}
}
