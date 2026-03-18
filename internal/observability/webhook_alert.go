package observability

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"
)

// WebhookAlertClient sends alerts to a generic webhook URL.
// Used as fallback when ALERT_PROVIDER is unset or unrecognized.
type WebhookAlertClient struct {
	webhookURL string
	httpClient *http.Client
	logger     *slog.Logger
}

// NewWebhookAlertClient creates a webhook alert client.
func NewWebhookAlertClient(webhookURL string, logger *slog.Logger) *WebhookAlertClient {
	return &WebhookAlertClient{
		webhookURL: webhookURL,
		httpClient: &http.Client{Timeout: 10 * time.Second},
		logger:     logger,
	}
}

// TriggerAlert sends the alert event as JSON to the webhook URL.
func (c *WebhookAlertClient) TriggerAlert(ctx context.Context, event AlertEvent) error {
	if c.webhookURL == "" {
		c.logger.Warn("webhook_alert_no_url", "event_type", event.EventType, "summary", event.Summary)
		return nil
	}

	body, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal webhook payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.webhookURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("webhook request: %w", err)
	}
	resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	return fmt.Errorf("webhook returned status %d", resp.StatusCode)
}
