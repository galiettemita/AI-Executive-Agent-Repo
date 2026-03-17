package hands_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brevio/brevio/internal/disclosure"
)

func TestDisclosureTransportEndToEnd(t *testing.T) {
	var capturedHeader string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedHeader = r.Header.Get(disclosure.AgentHeaderKey)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer server.Close()

	client := disclosure.NewDisclosureHTTPClient(nil)
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, server.URL+"/v1/skill/execute", nil)
	require.NoError(t, err)

	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, disclosure.AgentHeaderValue, capturedHeader,
		"X-Brevio-Agent must reach the external MCP server")
}

func TestDisclosureEmailSkillInjection_EndToEnd(t *testing.T) {
	originalArgs := map[string]any{
		"to": "bob@example.com", "subject": "Invoice", "body": "Please find attached.",
	}
	disclosedArgs := make(map[string]any, len(originalArgs))
	for k, v := range originalArgs {
		disclosedArgs[k] = v
	}
	if disclosure.IsEmailSkill("email.send") {
		disclosedArgs = disclosure.InjectEmailDisclosure(disclosedArgs)
	}
	body, _ := disclosedArgs["body"].(string)
	assert.Contains(t, body, "Please find attached.")
	assert.Contains(t, body, "Sent via Brevio AI")
	assert.Contains(t, body, disclosure.C2PATag)
	assert.Equal(t, "Please find attached.", originalArgs["body"].(string))
}

func TestDisclosureCalendarInjection_EndToEnd(t *testing.T) {
	originalArgs := map[string]any{
		"title": "Investor Call", "description": "Q1 update.", "start_time": "2026-04-15T14:00:00Z",
	}
	disclosedArgs := make(map[string]any, len(originalArgs))
	for k, v := range originalArgs {
		disclosedArgs[k] = v
	}
	if disclosure.IsCalendarWriteSkill("calendar.create") {
		disclosedArgs = disclosure.InjectCalendarDisclosure(disclosedArgs)
	}
	desc, _ := disclosedArgs["description"].(string)
	assert.Contains(t, desc, "Q1 update.")
	assert.Contains(t, desc, "Scheduled by Brevio AI")
}
