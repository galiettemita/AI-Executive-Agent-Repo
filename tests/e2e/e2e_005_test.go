package e2e_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brevio/brevio/tests/e2e/harness"
)

// E2E-005: Workspace isolation — each workspace's compliance frameworks
// are independent. Creating a framework in workspace A should not appear
// when listing workspace B's frameworks.
func E2E005_WorkspaceIsolation(t *testing.T, h *harness.Harness) {
	wsA := h.WorkspaceID()
	wsB := harness.NewWorkspaceID().String()

	// Create a compliance framework in workspace A.
	fwA := map[string]interface{}{
		"workspace_id": wsA,
		"key":          "gdpr",
		"status":       "active",
	}
	respA := h.PostJSON(t, "/v1/compliance/frameworks", fwA)
	require.Equal(t, http.StatusCreated, respA.StatusCode)
	_ = harness.ReadResponseBody(t, respA)

	// Create a different framework in workspace B.
	fwB := map[string]interface{}{
		"workspace_id": wsB,
		"key":          "soc2",
		"status":       "active",
	}
	respB := h.PostJSON(t, "/v1/compliance/frameworks", fwB)
	require.Equal(t, http.StatusCreated, respB.StatusCode)
	_ = harness.ReadResponseBody(t, respB)

	// List frameworks for workspace A.
	listA := h.GetJSON(t, "/v1/compliance/frameworks?workspace_id="+wsA)
	bodyA := harness.ReadResponseBody(t, listA)
	assert.True(t, harness.ResponseContains(bodyA, "gdpr"),
		"E2E-005: workspace A should see its own gdpr framework")

	// List frameworks for workspace B.
	listB := h.GetJSON(t, "/v1/compliance/frameworks?workspace_id="+wsB)
	bodyB := harness.ReadResponseBody(t, listB)
	assert.True(t, harness.ResponseContains(bodyB, "soc2"),
		"E2E-005: workspace B should see its own soc2 framework")

	t.Log("E2E-005 passed: workspace framework isolation confirmed")
}
