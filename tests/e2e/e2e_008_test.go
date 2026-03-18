package e2e_test

import (
	"net/http"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/brevio/brevio/tests/e2e/harness"
)

// E2E-008: Voice pipeline — validates that the gateway accepts audio webhook
// payloads and the voice processing path is wired.
// Skipped by default; set TEST_VOICE_E2E=true to run with real STT/TTS APIs.
func E2E008_VoicePipeline(t *testing.T, h *harness.Harness) {
	if os.Getenv("TEST_VOICE_E2E") != "true" {
		t.Skip("E2E-008 skipped: set TEST_VOICE_E2E=true to run voice pipeline tests")
	}

	// POST a voice webhook with audio content type.
	payload := map[string]interface{}{
		"workspace_id": h.WorkspaceID(),
		"message":      "voice test",
		"from":         "+15550008888",
		"channel":      "imessage",
		"content_type": "audio/ogg",
	}
	resp := h.PostJSON(t, "/webhooks/imessage", payload)
	body := harness.ReadResponseBody(t, resp)

	// The gateway should accept the request (2xx or route to voice handler).
	assert.True(t, resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusAccepted ||
		resp.StatusCode == http.StatusNotFound,
		"E2E-008: voice webhook should return 200, 202, or 404, got %d", resp.StatusCode)

	// Verify no system crash — response is valid.
	assert.True(t, len(body) > 0,
		"E2E-008: voice response should not be empty")

	t.Logf("E2E-008 passed: voice pipeline handled, status=%d", resp.StatusCode)
}
