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

const pagerDutyEventsURL = "https://events.pagerduty.com/v2/enqueue"

// PagerDutyClient sends alerts to PagerDuty Events API v2.
type PagerDutyClient struct {
	routingKey string
	httpClient *http.Client
	logger     *slog.Logger
}

// NewPagerDutyClient creates a PagerDuty client.
func NewPagerDutyClient(routingKey string, logger *slog.Logger) *PagerDutyClient {
	return &PagerDutyClient{
		routingKey: routingKey,
		httpClient: &http.Client{Timeout: 10 * time.Second},
		logger:     logger,
	}
}

type pagerDutyEvent struct {
	RoutingKey  string            `json:"routing_key"`
	EventAction string            `json:"event_action"`
	DedupKey    string            `json:"dedup_key"`
	Payload     pagerDutyPayload  `json:"payload"`
}

type pagerDutyPayload struct {
	Summary       string                 `json:"summary"`
	Source        string                 `json:"source"`
	Severity      string                 `json:"severity"`
	CustomDetails map[string]interface{} `json:"custom_details,omitempty"`
}

// TriggerAlert sends an alert to PagerDuty.
func (c *PagerDutyClient) TriggerAlert(ctx context.Context, event AlertEvent) error {
	dedupKey := event.EventType + ":" + event.WorkspaceID + ":" + event.WindowKey

	pdEvent := pagerDutyEvent{
		RoutingKey:  c.routingKey,
		EventAction: "trigger",
		DedupKey:    dedupKey,
		Payload: pagerDutyPayload{
			Summary:       event.Summary,
			Source:        "brevio-ai-agent",
			Severity:      PriorityToSeverity(event.Priority),
			CustomDetails: event.Details,
		},
	}

	body, err := json.Marshal(pdEvent)
	if err != nil {
		return fmt.Errorf("marshal PagerDuty event: %w", err)
	}

	// Try up to 2 times (initial + 1 retry on 5xx).
	for attempt := 0; attempt < 2; attempt++ {
		req, reqErr := http.NewRequestWithContext(ctx, http.MethodPost, pagerDutyEventsURL, bytes.NewReader(body))
		if reqErr != nil {
			return fmt.Errorf("create request: %w", reqErr)
		}
		req.Header.Set("Content-Type", "application/json")

		resp, doErr := c.httpClient.Do(req)
		if doErr != nil {
			if attempt == 0 {
				continue
			}
			return fmt.Errorf("PagerDuty request: %w", doErr)
		}
		resp.Body.Close()

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return nil
		}
		if resp.StatusCode >= 500 && attempt == 0 {
			continue
		}
		return fmt.Errorf("PagerDuty returned status %d", resp.StatusCode)
	}

	return fmt.Errorf("PagerDuty request failed after retry")
}
