package e2e_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brevio/brevio/tests/e2e/harness"
)

// E2E-001: Health endpoint returns 200 with status=healthy.
// This validates the basic gateway lifecycle: server starts, routes register,
// health check responds. Foundation for all subsequent E2E tests.
func E2E001_HealthEndpoint(t *testing.T, h *harness.Harness) {
	start := time.Now()

	resp := h.GetJSON(t, "/health")
	elapsed := time.Since(start)

	require.Equal(t, http.StatusOK, resp.StatusCode,
		"E2E-001: /health should return 200")

	body := harness.ReadResponseBody(t, resp)
	assert.True(t, harness.ResponseContains(body, "healthy", "ok", "status"),
		"E2E-001: health response should contain status indicator, got: %s", body)

	assert.Less(t, elapsed, 2*time.Second,
		"E2E-001: health check latency should be < 2s, got %s", elapsed)

	t.Logf("E2E-001 passed: status=200 latency=%s", elapsed)
}
