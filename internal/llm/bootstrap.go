package llm

import (
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/brevio/brevio/internal/cache"
	goredis "github.com/redis/go-redis/v9"
)

// BootstrapIntelligence creates an IntelligenceService from environment variables.
// Does NOT apply circuit breakers or rate limiting. Use BootstrapIntelligenceWithRedis
// for production hardening.
func BootstrapIntelligence() *IntelligenceService {
	return BootstrapIntelligenceWithRedis(nil)
}

// BootstrapIntelligenceWithRedis creates an IntelligenceService with circuit breakers
// and per-workspace Redis-backed rate limiting.
// redisClient may be nil — rate limiting is silently skipped when absent.
func BootstrapIntelligenceWithRedis(redisClient *cache.RedisClient) *IntelligenceService {
	anthropicKey := os.Getenv("ANTHROPIC_API_KEY")
	openaiKey := os.Getenv("OPENAI_API_KEY")

	if anthropicKey == "" && openaiKey == "" {
		log.Println("[LLM] No API keys configured (ANTHROPIC_API_KEY, OPENAI_API_KEY) — intelligence layer disabled")
		return nil
	}

	timeout := 60 * time.Second
	if v := os.Getenv("LLM_TIMEOUT_SECONDS"); v != "" {
		var seconds int
		if _, err := fmt.Sscanf(v, "%d", &seconds); err == nil && seconds > 0 {
			timeout = time.Duration(seconds) * time.Second
		}
	}

	cbCfg := DefaultCircuitBreakerConfig()
	limits := DefaultProviderRateLimits()

	var anthropicWrapped Client
	if anthropicKey != "" {
		raw, err := NewAnthropicClient(AnthropicConfig{APIKey: anthropicKey, Timeout: timeout})
		if err != nil {
			log.Printf("[LLM] Failed to create Anthropic client: %v", err)
		} else {
			// Wrap with circuit breaker.
			cbClient := Client(NewClientCircuitBreaker(raw, "anthropic", cbCfg))
			// Wrap with rate limiter when Redis available.
			if redisClient != nil {
				anthropicWrapped = NewRateLimitedClient(RateLimitedClientConfig{
					Inner: cbClient, RedisCache: redisClient,
					ProviderID: "anthropic", WorkspaceID: "global",
					Limits: limits["anthropic"],
				})
				log.Println("[LLM] Anthropic: circuit breaker + rate limiting active")
			} else {
				anthropicWrapped = cbClient
				log.Println("[LLM] Anthropic: circuit breaker active (no rate limiting)")
			}
		}
	}

	var openaiWrapped Client
	if openaiKey != "" {
		raw, err := NewOpenAIClient(OpenAIConfig{APIKey: openaiKey, Timeout: timeout})
		if err != nil {
			log.Printf("[LLM] Failed to create OpenAI client: %v", err)
		} else {
			cbClient := Client(NewClientCircuitBreaker(raw, "openai", cbCfg))
			if redisClient != nil {
				openaiWrapped = NewRateLimitedClient(RateLimitedClientConfig{
					Inner: cbClient, RedisCache: redisClient,
					ProviderID: "openai", WorkspaceID: "global",
					Limits: limits["openai"],
				})
				log.Println("[LLM] OpenAI: circuit breaker + rate limiting active")
			} else {
				openaiWrapped = cbClient
				log.Println("[LLM] OpenAI: circuit breaker active (no rate limiting)")
			}
		}
	}

	// Build failover chains. The failover client accepts the Client interface.
	classifierClient := buildWrappedFailoverClient(anthropicWrapped, openaiWrapped)
	plannerClient := buildWrappedFailoverClient(anthropicWrapped, openaiWrapped)
	synthesizerClient := buildWrappedFailoverClient(anthropicWrapped, openaiWrapped)

	if classifierClient == nil {
		log.Println("[LLM] No usable provider clients — intelligence layer disabled")
		return nil
	}

	intel := NewIntelligenceService(IntelligenceConfig{
		Classifier:  classifierClient,
		Planner:     plannerClient,
		Synthesizer: synthesizerClient,
	})
	log.Println("[LLM] Intelligence service bootstrapped with circuit breakers")
	return intel
}

// buildWrappedFailoverClient creates a FailoverClient from wrapped Client interfaces.
func buildWrappedFailoverClient(primary, fallback Client) Client {
	if primary != nil && fallback != nil {
		return &FailoverClient{
			Primary: primary, Fallback: fallback,
			PrimaryID: "primary", FallbackID: "fallback",
		}
	}
	if primary != nil {
		return primary
	}
	return fallback
}

// BootstrapService creates a fully wired Service with intelligence layer.
// Initializes Redis from REDIS_URL when available for rate limiting and replay cache.
func BootstrapService() *Service {
	svc := NewService()

	// Attempt Redis for rate limiting and replay cache.
	var redisClient *cache.RedisClient
	if redisURL := strings.TrimSpace(os.Getenv("REDIS_URL")); redisURL != "" {
		opts, err := goredis.ParseURL(redisURL)
		if err != nil {
			log.Printf("[LLM] REDIS_URL parse error: %v", err)
		} else {
			rdb := goredis.NewClient(opts)
			redisClient = cache.NewRedisClientFromRDB(rdb, 10*time.Minute)
			svc.SetRedisCache(redisClient)
			log.Println("[LLM] Redis cache active for rate limiting and replay")
		}
	}

	intel := BootstrapIntelligenceWithRedis(redisClient)
	if intel != nil {
		svc.SetIntelligence(intel)
	}
	return svc
}
