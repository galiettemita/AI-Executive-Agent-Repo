package llm

import (
	"encoding/json"
	"testing"
)

func TestDeterminismSameInput20Runs(t *testing.T) {
	t.Parallel()

	svc := NewService()
	req := Request{
		WorkspaceID: "ws1",
		PromptKey:   "brain.planner.v9",
		Input:       "plan my day",
		Tier:        "T2",
		ModelID:     "model-a",
		ProviderID:  "provider-a",
	}

	first := svc.Generate(req)
	for i := 0; i < 19; i++ {
		next := svc.Generate(req)
		if next.PlanJSON != first.PlanJSON {
			t.Fatalf("plan mismatch on run %d", i+2)
		}
	}
	if svc.ReplayHitCount() < 19 {
		t.Fatalf("expected replay hits >= 19, got %d", svc.ReplayHitCount())
	}
}

func TestShadowEvalRequiredBeforePromotionAndRollback(t *testing.T) {
	t.Parallel()

	svc := NewService()
	svc.RegisterPrompt(PromptVersion{PromptKey: "brain.system.v9", VersionInt: 1, Body: "v1", ShadowEvalPassed: true})
	svc.RegisterPrompt(PromptVersion{PromptKey: "brain.system.v9", VersionInt: 2, Body: "v2", ParentVersionInt: 1, ShadowEvalPassed: false})

	if err := svc.PromotePrompt("brain.system.v9", 2); err == nil {
		t.Fatal("expected promotion failure when shadow eval has not passed")
	}

	svc.RegisterPrompt(PromptVersion{PromptKey: "brain.system.v9", VersionInt: 3, Body: "v3", ParentVersionInt: 2, ShadowEvalPassed: true})
	if err := svc.PromotePrompt("brain.system.v9", 3); err != nil {
		t.Fatalf("unexpected promotion failure: %v", err)
	}
	if got := svc.ActivePromptVersion("brain.system.v9"); got != 3 {
		t.Fatalf("expected active prompt version 3, got %d", got)
	}

	if err := svc.RollbackPrompt("brain.system.v9", 1); err != nil {
		t.Fatalf("rollback prompt: %v", err)
	}
	if got := svc.ActivePromptVersion("brain.system.v9"); got != 1 {
		t.Fatalf("expected rollback to version 1, got %d", got)
	}
}

func TestTierMaxOutputTokenCap(t *testing.T) {
	t.Parallel()

	svc := NewService()
	resp := svc.Generate(Request{
		WorkspaceID:     "ws1",
		PromptKey:       "brain.planner.v9",
		Input:           "plan",
		Tier:            "T1",
		ModelID:         "model-a",
		ProviderID:      "provider-a",
		MaxOutputTokens: 2000,
	})

	var payload map[string]any
	if err := json.Unmarshal([]byte(resp.PlanJSON), &payload); err != nil {
		t.Fatalf("decode plan json: %v", err)
	}
	if int(payload["max_tokens"].(float64)) != 1024 {
		t.Fatalf("expected T1 cap of 1024 max tokens, got %v", payload["max_tokens"])
	}
	if payload["temperature"].(float64) != 0 || payload["top_p"].(float64) != 1 {
		t.Fatalf("expected deterministic generation params, got %+v", payload)
	}
}

func TestFallbackOnlyWhenNoOutputCommitted(t *testing.T) {
	t.Parallel()

	svc := NewService()
	req := Request{
		WorkspaceID: "ws1",
		PromptKey:   "brain.planner.v9",
		Input:       "plan",
		Tier:        "T2",
		ModelID:     "model-a",
		ProviderID:  "provider-primary",
	}

	fallback := svc.GenerateWithFallback(req, "provider-fallback", true, false)
	if fallback.ProviderID != "provider-fallback" {
		t.Fatalf("expected fallback provider, got %s", fallback.ProviderID)
	}
	if fallback.FailoverReason == "" {
		t.Fatal("expected failover reason when fallback is used")
	}

	noFallback := svc.GenerateWithFallback(req, "provider-fallback", true, true)
	if noFallback.ProviderID != "provider-primary" {
		t.Fatalf("expected primary provider when output already committed, got %s", noFallback.ProviderID)
	}
	if noFallback.FailoverReason != "" {
		t.Fatalf("expected no failover reason when output committed, got %s", noFallback.FailoverReason)
	}
}

func TestDefaultTierModelMapping(t *testing.T) {
	t.Parallel()

	mapping := DefaultTierModelMapping()
	if got := mapping["T0"]; got.PrimaryModel != ModelAnthropicHaiku || got.FallbackModel != ModelOpenAIGPT4oMini || got.MaxOutputTokens != 512 {
		t.Fatalf("unexpected T0 mapping: %+v", got)
	}
	if got := mapping["T3"]; got.PrimaryModel != ModelAnthropicSonnet || got.FallbackModel != ModelOpenAIGPT4o || got.MaxOutputTokens != 8192 {
		t.Fatalf("unexpected T3 mapping: %+v", got)
	}
}
