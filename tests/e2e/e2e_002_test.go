package e2e_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brevio/brevio/tests/e2e/harness"
)

// E2E-002: POST /v1/compliance/dsr creates a DSR request and returns it.
// Validates the compliance DSR intake pipeline end-to-end.
func E2E002_ComplianceDSRCreate(t *testing.T, h *harness.Harness) {
	// Create a DSR erasure request via the compliance API.
	payload := map[string]interface{}{
		"workspace_id": h.WorkspaceID(),
		"user_id":      h.UserIDStr(),
		"request_type": "deletion",
	}
	resp := h.PostJSON(t, "/v1/compliance/dsr", payload)
	require.Equal(t, http.StatusCreated, resp.StatusCode,
		"E2E-002: POST /v1/compliance/dsr should return 201 Created")

	body := harness.ReadResponseBody(t, resp)
	assert.True(t, harness.ResponseContains(body, "id", "status"),
		"E2E-002: response should contain id and status, got: %s", body)
	assert.True(t, harness.ResponseContains(body, "pending", "deletion"),
		"E2E-002: DSR should be in pending status with deletion type, got: %s", body)

	t.Logf("E2E-002 passed: DSR created via compliance API")
}
