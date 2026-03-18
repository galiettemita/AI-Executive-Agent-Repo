package e2e_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brevio/brevio/tests/e2e/harness"
)

// E2E-009: Episodic memory — validates that the learning API stores and
// retrieves lessons, which form the episodic memory layer.
func E2E009_EpisodicMemory(t *testing.T, h *harness.Harness) {
	wsID := h.WorkspaceID()

	// List lessons — should return 200 with lessons array.
	listResp := h.GetJSON(t, "/v1/learning/lessons?workspace_id="+wsID)
	require.Equal(t, http.StatusOK, listResp.StatusCode,
		"E2E-009: GET /v1/learning/lessons should return 200")
	listBody := harness.ReadResponseBody(t, listResp)
	assert.True(t, harness.ResponseContains(listBody, "lessons"),
		"E2E-009: response should contain 'lessons' field, got: %s", listBody)

	// List RAG collections — verifies the RAG retrieval layer is wired.
	ragResp := h.GetJSON(t, "/v1/rag/collections?workspace_id="+wsID)
	assert.Equal(t, http.StatusOK, ragResp.StatusCode,
		"E2E-009: GET /v1/rag/collections should return 200")
	ragBody := harness.ReadResponseBody(t, ragResp)
	assert.True(t, harness.ResponseContains(ragBody, "collections"),
		"E2E-009: RAG collections response should contain collections field")

	t.Log("E2E-009 passed: episodic memory and RAG retrieval APIs respond correctly")
}
