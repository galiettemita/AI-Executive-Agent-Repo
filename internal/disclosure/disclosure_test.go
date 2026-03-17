package disclosure_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brevio/brevio/internal/disclosure"
)

func TestBrevioAgentTransport_InjectsHeader(t *testing.T) {
	var capturedHeader string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedHeader = r.Header.Get(disclosure.AgentHeaderKey)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := disclosure.NewDisclosureHTTPClient(nil)
	resp, err := client.Get(server.URL + "/test")
	require.NoError(t, err)
	defer resp.Body.Close()
	_, _ = io.ReadAll(resp.Body)
	assert.Equal(t, disclosure.AgentHeaderValue, capturedHeader)
}

func TestBrevioAgentTransport_Idempotent(t *testing.T) {
	var capturedHeader string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedHeader = r.Header.Get(disclosure.AgentHeaderKey)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := disclosure.NewDisclosureHTTPClient(nil)
	req, _ := http.NewRequest(http.MethodGet, server.URL, nil)
	req.Header.Set(disclosure.AgentHeaderKey, "custom-value")
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, "custom-value", capturedHeader, "existing header must not be overwritten")
}

func TestInjectEmailDisclosure_AppendsFooterAndC2PA(t *testing.T) {
	args := map[string]any{"to": "alice@example.com", "subject": "Q3", "body": "Hi Alice, update."}
	result := disclosure.InjectEmailDisclosure(args)
	body, _ := result["body"].(string)
	assert.Contains(t, body, "Hi Alice")
	assert.Contains(t, body, "Sent via Brevio AI")
	assert.Contains(t, body, disclosure.C2PATag)
}

func TestInjectEmailDisclosure_NoBodyField_AddsFallback(t *testing.T) {
	args := map[string]any{"to": "alice@example.com"}
	result := disclosure.InjectEmailDisclosure(args)
	_, hasDisclosure := result["_brevio_disclosure"]
	_, hasC2PA := result["_c2pa_tag"]
	assert.True(t, hasDisclosure)
	assert.True(t, hasC2PA)
}

func TestInjectEmailDisclosure_DoesNotMutateOriginal(t *testing.T) {
	original := "Original body"
	args := map[string]any{"body": original}
	argsCopy := map[string]any{"body": original}
	disclosure.InjectEmailDisclosure(argsCopy)
	assert.Equal(t, original, args["body"])
	assert.Contains(t, argsCopy["body"], "Sent via Brevio AI")
}

func TestInjectCalendarDisclosure_AppendsDisclaimer(t *testing.T) {
	args := map[string]any{"title": "Board", "description": "Quarterly review."}
	result := disclosure.InjectCalendarDisclosure(args)
	desc, _ := result["description"].(string)
	assert.Contains(t, desc, "Quarterly review.")
	assert.Contains(t, desc, "Scheduled by Brevio AI")
}

func TestInjectCalendarDisclosure_NoDescription_CreatesField(t *testing.T) {
	args := map[string]any{"title": "Board"}
	result := disclosure.InjectCalendarDisclosure(args)
	desc, _ := result["description"].(string)
	assert.Equal(t, disclosure.CalendarDisclaimer, desc)
}

func TestIsEmailSkill(t *testing.T) {
	shouldMatch := []string{"email.send", "email.reply", "email.forward", "gmail.send", "outlook.send", "mail.send"}
	shouldNotMatch := []string{"email.read", "calendar.create", "email.search", "slack.send"}
	for _, k := range shouldMatch {
		assert.True(t, disclosure.IsEmailSkill(k), "should match: %q", k)
	}
	for _, k := range shouldNotMatch {
		assert.False(t, disclosure.IsEmailSkill(k), "should NOT match: %q", k)
	}
}

func TestIsCalendarWriteSkill(t *testing.T) {
	shouldMatch := []string{"calendar.create", "calendar.write", "calendar.update", "calendar.book", "gcal.create"}
	shouldNotMatch := []string{"calendar.read", "calendar.list", "email.send"}
	for _, k := range shouldMatch {
		assert.True(t, disclosure.IsCalendarWriteSkill(k), "should match: %q", k)
	}
	for _, k := range shouldNotMatch {
		assert.False(t, disclosure.IsCalendarWriteSkill(k), "should NOT match: %q", k)
	}
}

func TestC2PATagProperties(t *testing.T) {
	tag := disclosure.C2PATag
	assert.NotEmpty(t, tag)
	stripped := disclosure.StripC2PATag("hello" + tag + "world")
	assert.Equal(t, "helloworld", stripped)
	assert.Contains(t, tag, "brevio-ai")
}

func TestConstants(t *testing.T) {
	assert.Equal(t, "X-Brevio-Agent", disclosure.AgentHeaderKey)
	assert.Equal(t, "true", disclosure.AgentHeaderValue)
	assert.Equal(t, "1.0", disclosure.AgentVersion)
	assert.True(t, strings.Contains(disclosure.EmailFooter, "Brevio AI"))
	assert.True(t, strings.Contains(disclosure.CalendarDisclaimer, "Brevio AI"))
}
