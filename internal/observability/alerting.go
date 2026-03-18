package observability

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

// AlertEvent describes a routable alert.
type AlertEvent struct {
	EventType   string                 `json:"event_type"`
	WorkspaceID string                 `json:"workspace_id"`
	Priority    int                    `json:"priority"` // 1-3
	Summary     string                 `json:"summary"`
	Details     map[string]interface{} `json:"details,omitempty"`
	WindowKey   string                 `json:"window_key"` // for dedup: e.g. 5-min window
}

// AlertProvider is the interface for sending alerts to external systems.
type AlertProvider interface {
	TriggerAlert(ctx context.Context, event AlertEvent) error
}

// AlertRouter routes alerts to the configured provider with deduplication.
type AlertRouter struct {
	provider AlertProvider
	redis    *goredis.Client
	logger   *slog.Logger
}

// AlertConfig holds configuration for alert routing.
type AlertConfig struct {
	Provider            string
	PagerDutyRoutingKey string
	OpsGenieAPIKey      string
	SlackWebhookURL     string
	AlertWebhookURL     string
}

// NewAlertRouter creates an alert router with the appropriate provider.
func NewAlertRouter(cfg AlertConfig, redis *goredis.Client, logger *slog.Logger) *AlertRouter {
	var provider AlertProvider

	switch cfg.Provider {
	case "pagerduty":
		provider = NewPagerDutyClient(cfg.PagerDutyRoutingKey, logger)
	case "opsgenie":
		provider = NewOpsGenieClient(cfg.OpsGenieAPIKey, logger)
	case "slack":
		provider = NewSlackAlertClient(cfg.SlackWebhookURL, logger)
	default:
		provider = NewWebhookAlertClient(cfg.AlertWebhookURL, logger)
	}

	return &AlertRouter{
		provider: provider,
		redis:    redis,
		logger:   logger,
	}
}

// SendAlert routes an alert to the configured provider with deduplication.
// Deduplication window: 5 minutes (300 seconds).
func (r *AlertRouter) SendAlert(ctx context.Context, event AlertEvent) error {
	if event.WindowKey == "" {
		event.WindowKey = time.Now().Truncate(5 * time.Minute).Format(time.RFC3339)
	}

	dedupKey := computeDedupKey(event)

	// Check deduplication via Redis.
	if r.redis != nil {
		existing, err := r.redis.Get(ctx, "alert:dedup:"+dedupKey).Result()
		if err == nil && existing != "" {
			r.logger.Info("alert_deduplicated",
				"event_type", event.EventType,
				"workspace_id", event.WorkspaceID,
				"dedup_key", dedupKey,
			)
			return nil
		}

		// Set dedup key with 5-minute TTL.
		_ = r.redis.Set(ctx, "alert:dedup:"+dedupKey, "1", 300*time.Second).Err()
	}

	if err := r.provider.TriggerAlert(ctx, event); err != nil {
		r.logger.Error("alert_send_failed",
			"event_type", event.EventType,
			"error", err,
		)
		return fmt.Errorf("send alert: %w", err)
	}

	r.logger.Info("alert_sent",
		"event_type", event.EventType,
		"workspace_id", event.WorkspaceID,
		"priority", event.Priority,
	)
	return nil
}

func computeDedupKey(event AlertEvent) string {
	raw := event.EventType + ":" + event.WorkspaceID + ":" + event.WindowKey
	h := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(h[:8])
}

// PriorityToSeverity maps alert priority to severity string.
func PriorityToSeverity(priority int) string {
	switch priority {
	case 1:
		return "critical"
	case 2:
		return "error"
	case 3:
		return "warning"
	default:
		return "info"
	}
}
