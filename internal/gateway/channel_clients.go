package gateway

import (
	"fmt"
	"strings"
	"time"
)

type RetryPolicy struct {
	BaseDelay   time.Duration
	MaxDelay    time.Duration
	Jitter      time.Duration
	MaxAttempts int
}

type CircuitBreakerPolicy struct {
	FailureThreshold int
	Window           time.Duration
	HalfOpenAfter    time.Duration
}

type WhatsAppClientConfig struct {
	APIVersion    string
	PhoneNumberID string
	BaseURL       string
}

type IMessageClientConfig struct {
	MSPBaseURL string
	BusinessID string
}

func DefaultConnectorRetryPolicy() RetryPolicy {
	return RetryPolicy{
		BaseDelay:   1 * time.Second,
		MaxDelay:    60 * time.Second,
		Jitter:      500 * time.Millisecond,
		MaxAttempts: 5,
	}
}

func DefaultConnectorCircuitBreakerPolicy() CircuitBreakerPolicy {
	return CircuitBreakerPolicy{
		FailureThreshold: 5,
		Window:           60 * time.Second,
		HalfOpenAfter:    300 * time.Second,
	}
}

func NewWhatsAppClientConfig(apiVersion, phoneNumberID string) WhatsAppClientConfig {
	if strings.TrimSpace(apiVersion) == "" {
		apiVersion = "v21.0"
	}
	return WhatsAppClientConfig{
		APIVersion:    apiVersion,
		PhoneNumberID: phoneNumberID,
		BaseURL:       fmt.Sprintf("https://graph.facebook.com/%s/%s/messages", apiVersion, phoneNumberID),
	}
}

func WhatsAppRateLimitPerSecond(verified bool) int {
	if verified {
		return 1000
	}
	return 80
}

func WhatsAppRedisRateLimitKey(phoneNumberID string) string {
	return "rl:whatsapp:" + phoneNumberID
}

func WhatsAppRedisCircuitKey(phoneNumberID string) string {
	return "cb:whatsapp:" + phoneNumberID
}

func WhatsAppErrorAction(code int) string {
	switch code {
	case 131026:
		return "backoff"
	case 131047:
		return "queue_template"
	case 368:
		return "open_circuit_breaker"
	default:
		return "generic_error"
	}
}

func NewIMessageClientConfig(baseURL, businessID string) IMessageClientConfig {
	return IMessageClientConfig{
		MSPBaseURL: strings.TrimSpace(baseURL),
		BusinessID: strings.TrimSpace(businessID),
	}
}

func IMessageRedisRateLimitKey(businessID string) string {
	return "rl:imessage:" + businessID
}

func IMessageRedisCircuitKey(businessID string) string {
	return "cb:imessage:" + businessID
}
