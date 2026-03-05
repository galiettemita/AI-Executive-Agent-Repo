package contracts

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGatewayServiceRuntimeClosure(t *testing.T) {
	t.Parallel()

	root := repositoryRoot(t)
	gatewaySource := filepath.Join(root, "services", "brevio-gateway", "src", "index.ts")
	normalizeSource := filepath.Join(root, "services", "brevio-gateway", "src", "normalize.ts")
	gatewayReadme := filepath.Join(root, "services", "brevio-gateway", "README.md")

	assertFileContainsTokens(t, gatewaySource, []string{
		"/webhooks/whatsapp",
		"/webhooks/imessage",
		"/webhooks/temporal",
		"/api/v1/gateway/format",
		"x-hub-signature-256",
		"x-api-key",
		"idempotent_replay",
		"rate_limited",
		"normalizeWebhook",
		"sessionIdleMs",
		"gateway.webhook.accepted",
	})

	assertFileContainsTokens(t, normalizeSource, []string{
		"MessageEnvelope",
		"user_profile_hash",
		"channel_message_id",
		"session_id",
	})

	assertFileContainsTokens(t, gatewayReadme, []string{
		"POST /webhooks/whatsapp",
		"POST /webhooks/imessage",
		"POST /webhooks/temporal",
		"Deduplicates webhook events",
		"30 messages/hour",
		"120 messages/hour",
	})

	body, err := os.ReadFile(gatewayReadme)
	if err != nil {
		t.Fatalf("read gateway readme: %v", err)
	}
	if strings.Contains(strings.ToLower(string(body)), "scaffold directory") {
		t.Fatalf("gateway README still contains scaffold marker")
	}
}
