package llm

import (
	"context"
	"fmt"

	"github.com/brevio/brevio/internal/cache"
)

// RateLimitedClient wraps a Client and enforces per-workspace rate limits
// via Redis fixed-window counters. The key patterns follow the existing
// ProviderRateLimit.RedisKeyPattern convention from providers.go.
type RateLimitedClient struct {
	inner       Client
	redisCache  *cache.RedisClient
	providerID  string
	workspaceID string
	limits      ProviderRateLimit
}

// RateLimitedClientConfig holds the configuration for a rate-limited client.
type RateLimitedClientConfig struct {
	Inner       Client
	RedisCache  *cache.RedisClient
	ProviderID  string
	WorkspaceID string
	Limits      ProviderRateLimit
}

// NewRateLimitedClient creates a client that checks Redis rate limits before
// calling the inner provider. If Redis is nil, the inner client is called directly.
func NewRateLimitedClient(cfg RateLimitedClientConfig) *RateLimitedClient {
	return &RateLimitedClient{
		inner:       cfg.Inner,
		redisCache:  cfg.RedisCache,
		providerID:  cfg.ProviderID,
		workspaceID: cfg.WorkspaceID,
		limits:      cfg.Limits,
	}
}

// Generate checks rate limits before delegating to the inner client.
// On rate limit exceeded, returns a typed *cache.RateLimitError that Temporal
// retry policies can classify as retryable.
func (c *RateLimitedClient) Generate(ctx context.Context, req GenerateRequest) (*GenerateResponse, *Usage, error) {
	// Pre-flight rate limit check (request count only; tokens unknown yet).
	if c.redisCache != nil {
		err := c.redisCache.CheckRateLimit(ctx, c.providerID, c.workspaceID,
			c.limits.RequestsPerMinute, c.limits.TokensPerMinute, 0)
		if err != nil {
			// If it's a rate limit error, return it directly.
			if _, ok := err.(*cache.RateLimitError); ok {
				return nil, nil, fmt.Errorf("status 429: %w", err)
			}
			// For Redis errors, log and proceed without rate limiting.
		}
	}

	resp, usage, err := c.inner.Generate(ctx, req)
	if err != nil {
		return nil, nil, err
	}

	// Post-flight: record token usage in the rate limit window.
	if c.redisCache != nil && usage != nil {
		totalTokens := usage.InputTokens + usage.OutputTokens
		if limitErr := c.redisCache.CheckRateLimit(ctx, c.providerID, c.workspaceID,
			0, c.limits.TokensPerMinute, totalTokens); limitErr != nil {
			// Token limit exceeded after this call — the response is still valid
			// but subsequent calls will be blocked. We don't fail this call.
		}
	}

	return resp, usage, nil
}
