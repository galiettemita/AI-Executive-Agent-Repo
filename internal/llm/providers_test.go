package llm

import (
	"testing"
	"time"
)

func TestProviderRegistryAndRateLimits(t *testing.T) {
	t.Parallel()

	providers := DefaultProviderRegistry()
	if len(providers) != 2 || providers["anthropic"].BaseURL == "" || providers["openai"].BaseURL == "" {
		t.Fatalf("unexpected provider registry: %+v", providers)
	}
	limits := DefaultProviderRateLimits()
	if limits["anthropic"].RequestsPerMinute != 4000 || limits["openai"].TokensPerMinute != 800000 {
		t.Fatalf("unexpected provider limits: %+v", limits)
	}
}

func TestModelCatalogTracksMultimodalCapabilities(t *testing.T) {
	t.Parallel()

	catalog := DefaultModelCatalog()
	entry, ok := catalog["gpt-5.4"]
	if !ok {
		t.Fatal("expected gpt-5.4 in model catalog")
	}
	if entry.MaxContextTokens < 1_000_000 || !entry.SupportsTools || !entry.SupportsStreaming {
		t.Fatalf("expected frontier multimodal metadata, got %+v", entry)
	}
	visionModels := ModelsForInputModality("image")
	if len(visionModels) == 0 {
		t.Fatal("expected image-capable model routing candidates")
	}
}

func TestEstimateModelCostAndFailoverRules(t *testing.T) {
	t.Parallel()

	cost, err := EstimateModelCostUSD("gpt-4o-mini", TokenUsage{InputTokens: 1000, OutputTokens: 1000})
	if err != nil || cost <= 0 {
		t.Fatalf("expected positive model cost, cost=%f err=%v", cost, err)
	}
	if _, err := EstimateModelCostUSD("unknown", TokenUsage{}); err == nil {
		t.Fatal("expected unknown model error")
	}
	if !ShouldFailoverOnPrimaryError(500, 0, false, "T2") {
		t.Fatal("expected 5xx failover")
	}
	if !ShouldFailoverOnPrimaryError(429, 11*time.Second, false, "T1") {
		t.Fatal("expected 429 failover when retry-after > 10s")
	}
	if ShouldFailoverOnPrimaryError(429, 5*time.Second, false, "T1") {
		t.Fatal("did not expect immediate failover for short retry-after")
	}
}
