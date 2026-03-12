package llm

import (
	"fmt"
	"log"
	"os"
	"time"
)

// BootstrapIntelligence creates an IntelligenceService from environment variables.
// It configures provider clients based on available API keys and sets up
// failover chains per the tier model mapping.
//
// Environment variables:
//   - ANTHROPIC_API_KEY: Anthropic API key (required for primary Anthropic provider)
//   - OPENAI_API_KEY: OpenAI API key (required for primary OpenAI provider)
//   - LLM_TIMEOUT_SECONDS: HTTP timeout for provider calls (default: 60)
//
// Returns nil if no API keys are configured (degraded mode).
func BootstrapIntelligence() *IntelligenceService {
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

	var anthropicClient *AnthropicClient
	if anthropicKey != "" {
		var err error
		anthropicClient, err = NewAnthropicClient(AnthropicConfig{
			APIKey:  anthropicKey,
			Timeout: timeout,
		})
		if err != nil {
			log.Printf("[LLM] Failed to create Anthropic client: %v", err)
		} else {
			log.Println("[LLM] Anthropic client initialized")
		}
	}

	var openaiClient *OpenAIClient
	if openaiKey != "" {
		var err error
		openaiClient, err = NewOpenAIClient(OpenAIConfig{
			APIKey:  openaiKey,
			Timeout: timeout,
		})
		if err != nil {
			log.Printf("[LLM] Failed to create OpenAI client: %v", err)
		} else {
			log.Println("[LLM] OpenAI client initialized")
		}
	}

	// Build failover clients per tier model mapping.
	// T0/T1 (classification): Anthropic Haiku primary, OpenAI GPT-4o-mini fallback.
	// T2/T3 (planning/synthesis): Anthropic Sonnet primary, OpenAI GPT-4o fallback.
	classifierClient := buildFailoverClient(anthropicClient, openaiClient, "anthropic", "openai")
	plannerClient := buildFailoverClient(anthropicClient, openaiClient, "anthropic", "openai")
	synthesizerClient := buildFailoverClient(anthropicClient, openaiClient, "anthropic", "openai")

	if classifierClient == nil {
		log.Println("[LLM] No usable provider clients — intelligence layer disabled")
		return nil
	}

	intel := NewIntelligenceService(IntelligenceConfig{
		Classifier:  classifierClient,
		Planner:     plannerClient,
		Synthesizer: synthesizerClient,
	})

	log.Println("[LLM] Intelligence service bootstrapped successfully")
	return intel
}

// buildFailoverClient creates a FailoverClient from available provider clients.
// Returns nil only if no clients are available.
func buildFailoverClient(anthropic *AnthropicClient, openai *OpenAIClient, primaryID, fallbackID string) Client {
	if anthropic != nil && openai != nil {
		return &FailoverClient{
			Primary:    anthropic,
			Fallback:   openai,
			PrimaryID:  primaryID,
			FallbackID: fallbackID,
		}
	}
	if anthropic != nil {
		return anthropic
	}
	if openai != nil {
		return openai
	}
	return nil
}

// BootstrapService creates a fully wired Service with intelligence layer.
// This is the primary entry point for production initialization.
func BootstrapService() *Service {
	svc := NewService()
	intel := BootstrapIntelligence()
	if intel != nil {
		svc.SetIntelligence(intel)
	}
	return svc
}
