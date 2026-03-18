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

// SlackAlertClient sends alerts to a Slack incoming webhook.
type SlackAlertClient struct {
	webhookURL string
	httpClient *http.Client
	logger     *slog.Logger
}

// NewSlackAlertClient creates a Slack alert client.
func NewSlackAlertClient(webhookURL string, logger *slog.Logger) *SlackAlertClient {
	return &SlackAlertClient{
		webhookURL: webhookURL,
		httpClient: &http.Client{Timeout: 10 * time.Second},
		logger:     logger,
	}
}

type slackPayload struct {
	Text        string            `json:"text"`
	Attachments []slackAttachment `json:"attachments,omitempty"`
}

type slackAttachment struct {
	Color  string `json:"color"`
	Title  string `json:"title"`
	Text   string `json:"text"`
	Footer string `json:"footer"`
	Ts     int64  `json:"ts"`
}

// TriggerAlert sends an alert to Slack.
func (c *SlackAlertClient) TriggerAlert(ctx context.Context, event AlertEvent) error {
	color := "good"
	switch event.Priority {
	case 1:
		color = "danger"
	case 2:
		color = "warning"
	}

	payload := slackPayload{
		Text: fmt.Sprintf("[P%d ALERT] %s", event.Priority, event.Summary),
		Attachments: []slackAttachment{
			{
				Color:  color,
				Title:  event.EventType,
				Text:   fmt.Sprintf("Workspace: %s", event.WorkspaceID),
				Footer: "Brevio Alert System",
				Ts:     time.Now().Unix(),
			},
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal Slack payload: %w", err)
	}

	for attempt := 0; attempt < 2; attempt++ {
		req, reqErr := http.NewRequestWithContext(ctx, http.MethodPost, c.webhookURL, bytes.NewReader(body))
		if reqErr != nil {
			return fmt.Errorf("create request: %w", reqErr)
		}
		req.Header.Set("Content-Type", "application/json")

		resp, doErr := c.httpClient.Do(req)
		if doErr != nil {
			if attempt == 0 {
				continue
			}
			return fmt.Errorf("Slack request: %w", doErr)
		}
		resp.Body.Close()

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return nil
		}
		if resp.StatusCode >= 500 && attempt == 0 {
			continue
		}
		return fmt.Errorf("Slack returned status %d", resp.StatusCode)
	}

	return fmt.Errorf("Slack request failed after retry")
}
