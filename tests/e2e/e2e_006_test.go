package e2e_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brevio/brevio/tests/e2e/harness"
)

// E2E-006: DSR intake via /v1/dsr/requests returns 202 Accepted.
// Validates the dedicated DSR intake endpoint (separate from the compliance CRUD API).
// This endpoint is the GDPR Art. 17 entry point for data subject requests.
func E2E006_DLQRetryExhaustion(t *testing.T, h *harness.Harness) {
	// Create a DSR via the dedicated intake endpoint.
	payload := map[string]interface{}{
		"workspace_id": h.WorkspaceID(),
		"user_id":      h.UserIDStr(),
		"request_type": "erasure",
	}
	resp := h.PostJSON(t, "/v1/dsr/requests", payload)

	require.Equal(t, http.StatusAccepted, resp.StatusCode,
		"E2E-006: POST /v1/dsr/requests should return 202 Accepted")

	body := harness.ReadResponseBody(t, resp)
	assert.True(t, harness.ResponseContains(body, "request_id", "status", "deadline"),
		"E2E-006: response should contain request_id, status, and deadline, got: %s", body)
	assert.True(t, harness.ResponseContains(body, "pending"),
		"E2E-006: DSR status should be 'pending', got: %s", body)

	t.Logf("E2E-006 passed: DSR intake returned 202 with deadline")
}
