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

const opsGenieAlertsURL = "https://api.opsgenie.com/v2/alerts"

// OpsGenieClient sends alerts to OpsGenie Alerts API v2.
type OpsGenieClient struct {
	apiKey     string
	httpClient *http.Client
	logger     *slog.Logger
}

// NewOpsGenieClient creates an OpsGenie client.
func NewOpsGenieClient(apiKey string, logger *slog.Logger) *OpsGenieClient {
	return &OpsGenieClient{
		apiKey:     apiKey,
		httpClient: &http.Client{Timeout: 10 * time.Second},
		logger:     logger,
	}
}

type opsGenieAlert struct {
	Message  string                 `json:"message"`
	Alias    string                 `json:"alias"`
	Priority string                 `json:"priority"`
	Details  map[string]interface{} `json:"details,omitempty"`
}

// TriggerAlert sends an alert to OpsGenie.
func (c *OpsGenieClient) TriggerAlert(ctx context.Context, event AlertEvent) error {
	alert := opsGenieAlert{
		Message:  event.Summary,
		Alias:    event.EventType + ":" + event.WorkspaceID + ":" + event.WindowKey,
		Priority: fmt.Sprintf("P%d", event.Priority),
		Details:  event.Details,
	}

	body, err := json.Marshal(alert)
	if err != nil {
		return fmt.Errorf("marshal OpsGenie alert: %w", err)
	}

	for attempt := 0; attempt < 2; attempt++ {
		req, reqErr := http.NewRequestWithContext(ctx, http.MethodPost, opsGenieAlertsURL, bytes.NewReader(body))
		if reqErr != nil {
			return fmt.Errorf("create request: %w", reqErr)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "GenieKey "+c.apiKey)

		resp, doErr := c.httpClient.Do(req)
		if doErr != nil {
			if attempt == 0 {
				continue
			}
			return fmt.Errorf("OpsGenie request: %w", doErr)
		}
		resp.Body.Close()

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return nil
		}
		if resp.StatusCode >= 500 && attempt == 0 {
			continue
		}
		return fmt.Errorf("OpsGenie returned status %d", resp.StatusCode)
	}

	return fmt.Errorf("OpsGenie request failed after retry")
}
