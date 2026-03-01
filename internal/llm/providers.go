package llm

import (
	"fmt"
	"strings"
	"time"
)

type ProviderConfig struct {
	ProviderID string
	BaseURL    string
	AuthMethod string
	Models     []string
}

type ProviderRateLimit struct {
	RequestsPerMinute int
	TokensPerMinute   int
	RedisKeyPattern   string
}

type ModelCatalogEntry struct {
	ModelKey           string
	ProviderID         string
	CostPerInputToken  float64
	CostPerOutputToken float64
	MaxContextTokens   int
	Capabilities       []string
}

type TokenUsage struct {
	InputTokens  int
	OutputTokens int
}

func DefaultProviderRegistry() map[string]ProviderConfig {
	return map[string]ProviderConfig{
		"anthropic": {
			ProviderID: "anthropic",
			BaseURL:    "https://api.anthropic.com/v1",
			AuthMethod: "x-api-key",
			Models:     []string{"claude-sonnet-4-20250514", "claude-haiku-4-5-20250929"},
		},
		"openai": {
			ProviderID: "openai",
			BaseURL:    "https://api.openai.com/v1",
			AuthMethod: "authorization_bearer",
			Models:     []string{"gpt-4o", "gpt-4o-mini"},
		},
	}
}

func DefaultProviderRateLimits() map[string]ProviderRateLimit {
	return map[string]ProviderRateLimit{
		"anthropic": {
			RequestsPerMinute: 4000,
			TokensPerMinute:   400000,
			RedisKeyPattern:   "rl:llm:anthropic:{workspace_id}",
		},
		"openai": {
			RequestsPerMinute: 5000,
			TokensPerMinute:   800000,
			RedisKeyPattern:   "rl:llm:openai:{workspace_id}",
		},
	}
}

func DefaultModelCatalog() map[string]ModelCatalogEntry {
	return map[string]ModelCatalogEntry{
		"claude-sonnet-4-20250514": {
			ModelKey:           "claude-sonnet-4-20250514",
			ProviderID:         "anthropic",
			CostPerInputToken:  0.000003,
			CostPerOutputToken: 0.000015,
			MaxContextTokens:   200000,
			Capabilities:       []string{"planning", "synthesis", "extraction", "critique"},
		},
		"claude-haiku-4-5-20250929": {
			ModelKey:           "claude-haiku-4-5-20250929",
			ProviderID:         "anthropic",
			CostPerInputToken:  0.0000008,
			CostPerOutputToken: 0.000004,
			MaxContextTokens:   200000,
			Capabilities:       []string{"classification", "extraction", "routing", "simple_synthesis"},
		},
		"gpt-4o": {
			ModelKey:           "gpt-4o",
			ProviderID:         "openai",
			CostPerInputToken:  0.0000025,
			CostPerOutputToken: 0.00001,
			MaxContextTokens:   128000,
			Capabilities:       []string{"planning", "synthesis", "extraction", "critique"},
		},
		"gpt-4o-mini": {
			ModelKey:           "gpt-4o-mini",
			ProviderID:         "openai",
			CostPerInputToken:  0.00000015,
			CostPerOutputToken: 0.0000006,
			MaxContextTokens:   128000,
			Capabilities:       []string{"classification", "extraction", "routing", "simple_synthesis"},
		},
	}
}

func EstimateModelCostUSD(modelKey string, usage TokenUsage) (float64, error) {
	entry, ok := DefaultModelCatalog()[strings.TrimSpace(modelKey)]
	if !ok {
		return 0, fmt.Errorf("unknown model key: %s", modelKey)
	}
	if usage.InputTokens < 0 || usage.OutputTokens < 0 {
		return 0, fmt.Errorf("token usage cannot be negative")
	}
	return (entry.CostPerInputToken * float64(usage.InputTokens)) + (entry.CostPerOutputToken * float64(usage.OutputTokens)), nil
}

func ShouldFailoverOnPrimaryError(httpStatusCode int, retryAfter time.Duration, isTimeout bool, tier string) bool {
	switch {
	case httpStatusCode >= 500:
		return true
	case httpStatusCode == 429:
		return retryAfter > 10*time.Second
	case isTimeout:
		normalizedTier := strings.ToUpper(strings.TrimSpace(tier))
		if normalizedTier == "T0" || normalizedTier == "T1" {
			return true
		}
		return true
	default:
		return false
	}
}
