package gateway

import (
	"fmt"
	"strings"
)

type EnvConfig struct {
	Environment           string
	WebhookSecret         string
	IMessageWebhookAPIKey string
	ListenAddr            string
	ServiceVersion        string
}

func LoadEnvConfig(getenv func(string) string) (EnvConfig, error) {
	if getenv == nil {
		return EnvConfig{}, fmt.Errorf("getenv is required")
	}

	env := normalizeEnv(getenv("BREVIO_ENV"))
	webhookSecret := strings.TrimSpace(getenv("GATEWAY_WEBHOOK_SECRET"))
	imessageAPIKey := strings.TrimSpace(getenv("IMESSAGE_WEBHOOK_API_KEY"))
	listenAddr := strings.TrimSpace(getenv("GATEWAY_LISTEN_ADDR"))
	serviceVersion := strings.TrimSpace(getenv("SERVICE_VERSION"))

	if listenAddr == "" {
		listenAddr = ":18080"
	}
	if serviceVersion == "" {
		serviceVersion = "0.1.0"
	}

	if webhookSecret == "" {
		if isLocalLikeEnv(env) {
			webhookSecret = "dev-secret"
		} else {
			return EnvConfig{}, fmt.Errorf("GATEWAY_WEBHOOK_SECRET is required when BREVIO_ENV=%s", env)
		}
	}
	if imessageAPIKey == "" {
		if isLocalLikeEnv(env) {
			imessageAPIKey = "dev-imessage-key"
		} else {
			return EnvConfig{}, fmt.Errorf("IMESSAGE_WEBHOOK_API_KEY is required when BREVIO_ENV=%s", env)
		}
	}

	return EnvConfig{
		Environment:           env,
		WebhookSecret:         webhookSecret,
		IMessageWebhookAPIKey: imessageAPIKey,
		ListenAddr:            listenAddr,
		ServiceVersion:        serviceVersion,
	}, nil
}

func normalizeEnv(raw string) string {
	trimmed := strings.ToLower(strings.TrimSpace(raw))
	if trimmed == "" {
		return "local"
	}
	return trimmed
}

func isLocalLikeEnv(env string) bool {
	switch normalizeEnv(env) {
	case "local", "dev", "test":
		return true
	default:
		return false
	}
}
