package harness

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
)

// bytesReader returns a ReadCloser from a byte slice.
func bytesReader(b []byte) io.ReadCloser {
	return io.NopCloser(bytes.NewReader(b))
}

// PostWebhook sends a webhook message to the gateway and returns the HTTP response.
func (h *Harness) PostWebhook(t *testing.T, channel, message, from string) *http.Response {
	t.Helper()

	payload := map[string]interface{}{
		"workspace_id": h.WorkspaceID(),
		"message":      message,
		"from":         from,
		"channel":      channel,
	}
	body, _ := json.Marshal(payload)
	return h.CallJSON(t, http.MethodPost, "/webhooks/"+channel, body)
}

// PostJSON sends a JSON POST to a path and returns the response.
func (h *Harness) PostJSON(t *testing.T, path string, payload interface{}) *http.Response {
	t.Helper()
	body, _ := json.Marshal(payload)
	return h.CallJSON(t, http.MethodPost, path, body)
}

// GetJSON sends a GET request and returns the response.
func (h *Harness) GetJSON(t *testing.T, path string) *http.Response {
	t.Helper()
	return h.CallJSON(t, http.MethodGet, path, nil)
}

// ReadResponseBody reads and returns the body as a string, closing the body.
func ReadResponseBody(t *testing.T, resp *http.Response) string {
	t.Helper()
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("ReadResponseBody: %v", err)
	}
	return string(b)
}

// DecodeResponseJSON reads the body into the target struct.
func DecodeResponseJSON(t *testing.T, resp *http.Response, target interface{}) {
	t.Helper()
	defer resp.Body.Close()
	if err := json.NewDecoder(resp.Body).Decode(target); err != nil {
		t.Fatalf("DecodeResponseJSON: %v", err)
	}
}

// CreateDSRRequest creates a DSR erasure request via the API.
func (h *Harness) CreateDSRRequest(t *testing.T) (string, *http.Response) {
	t.Helper()
	payload := map[string]interface{}{
		"workspace_id": h.WorkspaceID(),
		"user_id":      h.UserIDStr(),
		"request_type": "erasure",
	}
	resp := h.PostJSON(t, "/v1/dsr/requests", payload)

	var result map[string]interface{}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	_ = json.Unmarshal(body, &result)

	requestID, _ := result["request_id"].(string)
	// Re-wrap body for caller
	resp.Body = io.NopCloser(bytes.NewReader(body))
	return requestID, resp
}

// CreateComplianceDSR creates a compliance DSR via the compliance API.
func (h *Harness) CreateComplianceDSR(t *testing.T, requestType string) map[string]interface{} {
	t.Helper()
	payload := map[string]interface{}{
		"workspace_id": h.WorkspaceID(),
		"user_id":      h.UserIDStr(),
		"request_type": requestType,
	}
	resp := h.PostJSON(t, "/v1/compliance/dsr", payload)
	var result map[string]interface{}
	DecodeResponseJSON(t, resp, &result)
	return result
}

// GetHealth calls GET /health and returns the response body.
func (h *Harness) GetHealth(t *testing.T) map[string]interface{} {
	t.Helper()
	resp := h.GetJSON(t, "/health")
	var result map[string]interface{}
	DecodeResponseJSON(t, resp, &result)
	return result
}

// ResponseContains checks if any of the given substrings appear in the response body (case-insensitive).
func ResponseContains(body string, substrings ...string) bool {
	lower := strings.ToLower(body)
	for _, s := range substrings {
		if strings.Contains(lower, strings.ToLower(s)) {
			return true
		}
	}
	return false
}

// NewWorkspaceID returns a new random workspace UUID.
func NewWorkspaceID() uuid.UUID {
	return uuid.New()
}
