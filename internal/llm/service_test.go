package llm

import "testing"

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

func TestShadowEvalRequiredBeforePromotion(t *testing.T) {
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
}
