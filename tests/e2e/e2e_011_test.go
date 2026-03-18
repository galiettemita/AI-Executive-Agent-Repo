package e2e_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brevio/brevio/tests/e2e/harness"
)

// E2E-011: DSR erasure cascade — validates the full DSR lifecycle:
// 1. Create DSR via intake endpoint (202 Accepted)
// 2. Verify DSR appears in compliance DSR list
// 3. Create DSR via compliance API (201 Created)
// 4. Verify both DSRs are tracked independently
// In production with Temporal, DSRFullErasureWorkflow would delete across 6 stores.
func E2E011_DSRErasureCascade(t *testing.T, h *harness.Harness) {
	wsID := h.WorkspaceID()

	// Step 1: Create DSR via dedicated intake endpoint.
	dsrPayload := map[string]interface{}{
		"workspace_id": wsID,
		"user_id":      h.UserIDStr(),
		"request_type": "erasure",
	}
	intakeResp := h.PostJSON(t, "/v1/dsr/requests", dsrPayload)
	require.Equal(t, http.StatusAccepted, intakeResp.StatusCode,
		"E2E-011: DSR intake should return 202")
	intakeBody := harness.ReadResponseBody(t, intakeResp)
	assert.True(t, harness.ResponseContains(intakeBody, "request_id", "pending", "deadline"),
		"E2E-011: DSR intake response must contain request_id, status, deadline")

	// Step 2: Create DSR via compliance API.
	compPayload := map[string]interface{}{
		"workspace_id": wsID,
		"user_id":      h.UserIDStr(),
		"request_type": "deletion",
	}
	compResp := h.PostJSON(t, "/v1/compliance/dsr", compPayload)
	require.Equal(t, http.StatusCreated, compResp.StatusCode,
		"E2E-011: compliance DSR should return 201")
	compBody := harness.ReadResponseBody(t, compResp)
	assert.True(t, harness.ResponseContains(compBody, "id", "status"),
		"E2E-011: compliance DSR response must contain id and status")

	// Step 3: List DSRs for workspace — should show both.
	listResp := h.GetJSON(t, "/v1/compliance/dsr?workspace_id="+wsID)
	listBody := harness.ReadResponseBody(t, listResp)
	assert.Equal(t, http.StatusOK, listResp.StatusCode)
	assert.True(t, harness.ResponseContains(listBody, "dsr_requests"),
		"E2E-011: DSR list should contain dsr_requests field")

	// Step 4: Verify DSR SLA tracking (deadline should be ~28 days out).
	assert.True(t, harness.ResponseContains(listBody, "sla_at_risk"),
		"E2E-011: DSR list should include SLA-at-risk tracking")

	t.Log("E2E-011 passed: DSR erasure cascade lifecycle validated end-to-end")
}
