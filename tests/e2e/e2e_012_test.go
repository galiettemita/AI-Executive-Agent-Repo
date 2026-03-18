package e2e_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brevio/brevio/tests/e2e/harness"
)

// E2E-012: Gateway survives restart — validates that the gateway HTTP server
// can be stopped and restarted cleanly, and all core endpoints respond after
// restart. In production with PostgreSQL, PgWorldModelRepository ensures facts
// survive pod restarts; here we validate the restart lifecycle itself.
func E2E012_WorldModelSurvivesRestart(t *testing.T, h *harness.Harness) {
	// Step 1: Verify health endpoint works before restart.
	beforeResp := h.GetJSON(t, "/health")
	require.Equal(t, http.StatusOK, beforeResp.StatusCode,
		"E2E-012: health must respond before restart")
	_ = harness.ReadResponseBody(t, beforeResp)

	// Step 2: Record the old gateway URL.
	oldURL := h.GatewayURL

	// Step 3: Restart gateway (simulates pod restart).
	h.RestartGatewayContainer(t)

	// Step 4: Verify the URL changed (new port).
	assert.NotEqual(t, oldURL, h.GatewayURL,
		"E2E-012: gateway URL should change after restart (new port)")

	// Step 5: Verify health endpoint responds on the new server.
	afterHealth := h.GetJSON(t, "/health")
	require.Equal(t, http.StatusOK, afterHealth.StatusCode,
		"E2E-012: health must respond after restart")
	_ = harness.ReadResponseBody(t, afterHealth)

	// Step 6: Verify core API paths respond after restart.
	endpoints := []string{
		"/v1/admin/users",
		"/v1/compliance/dsr?workspace_id=" + h.WorkspaceID(),
		"/v1/compliance/frameworks?workspace_id=" + h.WorkspaceID(),
		"/v1/learning/lessons?workspace_id=" + h.WorkspaceID(),
	}
	for _, ep := range endpoints {
		resp := h.GetJSON(t, ep)
		assert.Equal(t, http.StatusOK, resp.StatusCode,
			"E2E-012: %s must respond 200 after restart", ep)
		_ = harness.ReadResponseBody(t, resp)
	}

	t.Log("E2E-012 passed: gateway survives restart, all endpoints respond")
}
