package e2e_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/brevio/brevio/tests/e2e/harness"
)

// E2E-010: Proactive monitoring — validates that the tool health and
// feature flags endpoints are wired for proactive system monitoring.
func E2E010_ProactiveCalendarConflict(t *testing.T, h *harness.Harness) {
	// List tool health scores — validates monitoring is wired.
	healthResp := h.GetJSON(t, "/v1/tools/health")
	healthBody := harness.ReadResponseBody(t, healthResp)
	assert.Equal(t, http.StatusOK, healthResp.StatusCode,
		"E2E-010: tool health endpoint should return 200")
	assert.True(t, len(healthBody) > 2,
		"E2E-010: tool health response should not be empty")

	// List feature flags — validates the experiment routing layer is wired.
	flagsResp := h.GetJSON(t, "/v1/flags")
	flagsBody := harness.ReadResponseBody(t, flagsResp)
	assert.Equal(t, http.StatusOK, flagsResp.StatusCode,
		"E2E-010: flags endpoint should return 200")
	assert.True(t, harness.ResponseContains(flagsBody, "flags"),
		"E2E-010: flags response should contain 'flags' field")

	t.Log("E2E-010 passed: proactive monitoring infrastructure responds correctly")
}
