package llm

import (
	"context"
	"fmt"
	"time"
)

// HaikuHealthCheck performs a lightweight probe of the Haiku client at startup.
// A failed health check is logged as a WARNING (not fatal).
func HaikuHealthCheck(ctx context.Context, client LLMCompleter, timeout time.Duration) error {
	checkCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	result, err := client.Complete(checkCtx,
		"You are a health check endpoint. Reply with exactly: OK",
		"Health check")
	if err != nil {
		return fmt.Errorf("haiku health check failed: %w", err)
	}
	if result == "" {
		return fmt.Errorf("haiku health check: empty response")
	}
	return nil
}
