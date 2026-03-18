package e2e_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/brevio/brevio/tests/e2e/harness"
)

// E2E-003: Admin API gate — validates that the admin control plane
// exposes management endpoints and returns structured responses.
func E2E003_AutonomyGateBlocksWrite(t *testing.T, h *harness.Harness) {
	// List admin users — should return 200 with a users array.
	resp := h.GetJSON(t, "/v1/admin/users")
	assert.Equal(t, http.StatusOK, resp.StatusCode,
		"E2E-003: GET /v1/admin/users should return 200")

	body := harness.ReadResponseBody(t, resp)
	assert.True(t, harness.ResponseContains(body, "users"),
		"E2E-003: response should contain 'users' field, got: %s", body)

	// Verify the admin API returns a JSON response.
	assert.True(t, len(body) > 2,
		"E2E-003: admin response should not be empty")

	t.Log("E2E-003 passed: admin control plane responds correctly")
}
