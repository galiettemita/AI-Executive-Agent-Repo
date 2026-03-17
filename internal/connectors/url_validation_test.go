package connectors_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/brevio/brevio/internal/connectors"
)

func TestIsPlaceholderMCPURL_BlockedPatterns(t *testing.T) {
	blocked := []struct {
		url    string
		reason string
	}{
		{"", "empty URL"},
		{"   ", "whitespace-only URL"},
		{"http://unconfigured.local/mcp", "seed-file placeholder"},
		{"https://unconfigured.local/v1/skills", "seed-file placeholder HTTPS"},
		{"http://localhost:8080/v1", "localhost HTTP"},
		{"https://localhost/skills", "localhost HTTPS"},
		{"HTTPS://LOCALHOST/skills", "localhost uppercase"},
		{"http://127.0.0.1:3000/mcp", "loopback 127.0.0.1"},
		{"https://127.0.0.1/api", "loopback 127.0.0.1 HTTPS"},
		{"http://127.255.255.255/mcp", "loopback 127.x edge"},
		{"http://[::1]:8080/mcp", "IPv6 loopback"},
		{"http://0.0.0.0:9090/v1", "all-zeros binding"},
		{"https://0.0.0.0/mcp", "all-zeros binding HTTPS"},
	}
	for _, tc := range blocked {
		t.Run(tc.reason, func(t *testing.T) {
			assert.True(t, connectors.IsPlaceholderMCPURL(tc.url),
				"URL %q must be blocked: %s", tc.url, tc.reason)
		})
	}
}

func TestIsPlaceholderMCPURL_AllowedPatterns(t *testing.T) {
	allowed := []struct {
		url    string
		reason string
	}{
		{"https://mcp.acme-corp.com/v1", "legitimate external URL"},
		{"https://skills.brevio.ai/v1", "Brevio production endpoint"},
		{"https://brevio-hands.internal.svc.cluster.local/v1", "Kubernetes cluster DNS"},
		{"https://10.100.50.25/mcp", "internal cluster IP (not loopback)"},
		{"https://192.168.1.100/v1", "private range (non-loopback)"},
		{"https://mcp.staging.brevio.com/v1", "staging endpoint"},
	}
	for _, tc := range allowed {
		t.Run(tc.reason, func(t *testing.T) {
			assert.False(t, connectors.IsPlaceholderMCPURL(tc.url),
				"URL %q must be allowed: %s", tc.url, tc.reason)
		})
	}
}

func TestIsPlaceholderMCPURL_DoesNotBlockKubernetesClusterDNS(t *testing.T) {
	k8sURLs := []string{
		"https://brevio-skills.default.svc.cluster.local/v1",
		"https://mcp-service.brevio.svc.cluster.local:8080/skills",
		"http://hands-runtime.prod.svc.cluster.local/api",
	}
	for _, url := range k8sURLs {
		assert.False(t, connectors.IsPlaceholderMCPURL(url),
			"Kubernetes cluster DNS %q must not be blocked", url)
	}
}
