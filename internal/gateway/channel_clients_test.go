package gateway

import (
	"strings"
	"testing"
	"time"
)

func TestChannelClientPolicies(t *testing.T) {
	t.Parallel()

	retry := DefaultConnectorRetryPolicy()
	if retry.BaseDelay != time.Second || retry.MaxDelay != 60*time.Second || retry.MaxAttempts != 5 {
		t.Fatalf("unexpected retry policy: %+v", retry)
	}
	cb := DefaultConnectorCircuitBreakerPolicy()
	if cb.FailureThreshold != 5 || cb.Window != 60*time.Second || cb.HalfOpenAfter != 300*time.Second {
		t.Fatalf("unexpected circuit breaker policy: %+v", cb)
	}
}

func TestWhatsAppClientConfigAndErrorActions(t *testing.T) {
	t.Parallel()

	cfg := NewWhatsAppClientConfig("", "123456789")
	if cfg.APIVersion != "v21.0" {
		t.Fatalf("unexpected whatsapp api version: %+v", cfg)
	}
	if !strings.Contains(cfg.BaseURL, "/v21.0/123456789/messages") {
		t.Fatalf("unexpected whatsapp base url: %s", cfg.BaseURL)
	}
	if WhatsAppRateLimitPerSecond(false) != 80 || WhatsAppRateLimitPerSecond(true) != 1000 {
		t.Fatal("unexpected whatsapp rate limits")
	}
	if WhatsAppErrorAction(131047) != "queue_template" {
		t.Fatalf("unexpected whatsapp error mapping")
	}
}

func TestIMessageClientConfigAndRedisKeys(t *testing.T) {
	t.Parallel()

	cfg := NewIMessageClientConfig("https://msp.example.com/v1", "business-abc")
	if cfg.MSPBaseURL == "" || cfg.BusinessID == "" {
		t.Fatalf("unexpected imessage client config: %+v", cfg)
	}
	if IMessageRedisRateLimitKey("business-abc") != "rl:imessage:business-abc" {
		t.Fatal("unexpected imessage rl key")
	}
	if IMessageRedisCircuitKey("business-abc") != "cb:imessage:business-abc" {
		t.Fatal("unexpected imessage circuit key")
	}
}
