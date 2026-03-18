package redteam

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	promptInjectionSuitePath = "evals/prompt_injection_suite.json"
	slackWebhookEnvVar       = "SECURITY_SLACK_WEBHOOK_URL"
)

// CheckAutoHardening examines the red-team report for bypasses (unblocked attacks).
// For each bypass:
//   - Appends the payload to the prompt injection test suite (if not already present)
//   - Sends an alert to the security Slack channel
//   - Pauses autonomy promotion for high-risk workspaces
func CheckAutoHardening(ctx context.Context, report *RedTeamReport, db *pgxpool.Pool, logger *slog.Logger) error {
	var bypasses []AttackResult

	for _, r := range report.GCGResults {
		if !r.Blocked {
			bypasses = append(bypasses, r)
		}
	}
	for _, r := range report.AutoDANResults {
		if !r.Blocked {
			bypasses = append(bypasses, r)
		}
	}

	if len(bypasses) == 0 {
		logger.Info("auto_hardening_no_bypasses")
		return nil
	}

	logger.Warn("auto_hardening_bypasses_detected", "count", len(bypasses))

	// Load existing prompt injection suite.
	suite, err := loadInjectionSuite()
	if err != nil {
		return fmt.Errorf("load injection suite: %w", err)
	}

	existingHashes := make(map[string]struct{})
	for _, c := range suite.Cases {
		existingHashes[PayloadHash(c.Input)] = struct{}{}
	}

	newCases := 0
	for _, bypass := range bypasses {
		if _, exists := existingHashes[bypass.PayloadHash]; exists {
			logger.Info("auto_hardening_payload_already_known", "hash", bypass.PayloadHash)
			continue
		}

		// Append new case to the suite.
		suite.Cases = append(suite.Cases, PromptInjectionCase{
			ID:          fmt.Sprintf("auto-%s-%s", bypass.AttackType, bypass.PayloadHash[:12]),
			Category:    "auto_hardening_" + bypass.AttackType,
			TrustSource: "redteam",
			Input:       bypass.PayloadHash, // Store hash only for safety.
			Expected:    "blocked",
			Severity:    "critical",
		})
		existingHashes[bypass.PayloadHash] = struct{}{}
		newCases++

		// Alert via Slack webhook.
		alertMsg := fmt.Sprintf("[RED-TEAM] New bypass detected: attack_type=%s, block_layer=none. Added to injection suite.", bypass.AttackType)
		if alertErr := sendSlackAlert(ctx, alertMsg, logger); alertErr != nil {
			logger.Error("slack_alert_error", "error", alertErr)
		}

		logger.Info("auto_hardening_payload_added",
			"attack_type", bypass.AttackType,
			"hash", bypass.PayloadHash,
		)
	}

	// Write updated suite back to disk.
	if newCases > 0 {
		if err := writeInjectionSuite(suite); err != nil {
			return fmt.Errorf("write injection suite: %w", err)
		}
		logger.Info("auto_hardening_suite_updated", "new_cases", newCases)
	}

	// Pause autonomy promotion for workspaces with autonomy_level > 1.
	if db != nil {
		if err := pauseAutonomyPromotion(ctx, db, logger); err != nil {
			logger.Error("pause_autonomy_error", "error", err)
		}
	}

	return nil
}

func loadInjectionSuite() (*PromptInjectionSuite, error) {
	data, err := os.ReadFile(promptInjectionSuitePath)
	if err != nil {
		return nil, fmt.Errorf("read suite file: %w", err)
	}

	var suite PromptInjectionSuite
	if err := json.Unmarshal(data, &suite); err != nil {
		return nil, fmt.Errorf("unmarshal suite: %w", err)
	}

	return &suite, nil
}

func writeInjectionSuite(suite *PromptInjectionSuite) error {
	data, err := json.MarshalIndent(suite, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal suite: %w", err)
	}

	if err := os.WriteFile(promptInjectionSuitePath, data, 0644); err != nil {
		return fmt.Errorf("write suite file: %w", err)
	}

	return nil
}

// sendSlackAlert posts an alert message to the security Slack channel via webhook.
func sendSlackAlert(ctx context.Context, message string, logger *slog.Logger) error {
	webhookURL := os.Getenv(slackWebhookEnvVar)
	if webhookURL == "" {
		logger.Info("slack_webhook_not_configured", "message", message)
		return nil
	}

	payload := fmt.Sprintf(`{"text":%q}`, message)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, webhookURL, strings.NewReader(payload))
	if err != nil {
		return fmt.Errorf("create slack request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("send slack alert: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("slack webhook returned status %d", resp.StatusCode)
	}

	logger.Info("slack_alert_sent", "message", message)
	return nil
}

// pauseAutonomyPromotion sets autonomy_paused=true for all workspaces where
// autonomy_level > 1. This is a temporary safety pause pending human review.
func pauseAutonomyPromotion(ctx context.Context, db *pgxpool.Pool, logger *slog.Logger) error {
	tag, err := db.Exec(ctx,
		`UPDATE workspaces SET autonomy_paused = true
		 WHERE autonomy_level > 1 AND (autonomy_paused IS NULL OR autonomy_paused = false)`,
	)
	if err != nil {
		return fmt.Errorf("pause autonomy: %w", err)
	}

	logger.Warn("autonomy_promotion_paused",
		"affected_workspaces", tag.RowsAffected(),
	)
	return nil
}
