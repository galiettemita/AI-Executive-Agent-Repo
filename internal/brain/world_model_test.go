package brain

import "testing"

func TestWorldModelAddAndGetFacts(t *testing.T) {
	t.Parallel()

	wm := NewWorldModelService()
	fact, err := wm.AddFact("ws1", "email_tool", "rate_limit", "100/min", "documentation")
	if err != nil {
		t.Fatalf("add fact: %v", err)
	}
	if fact.Subject != "email_tool" {
		t.Fatalf("expected subject email_tool, got %s", fact.Subject)
	}

	facts := wm.GetFacts("ws1", "email_tool")
	if len(facts) != 1 {
		t.Fatalf("expected 1 fact, got %d", len(facts))
	}
}

func TestWorldModelUpdateFromFailure(t *testing.T) {
	t.Parallel()

	wm := NewWorldModelService()
	fact, err := wm.UpdateFromFailure("ws1", "calendar_api", "rate limit exceeded")
	if err != nil {
		t.Fatalf("update from failure: %v", err)
	}
	if fact.Predicate != "last_failure" {
		t.Fatalf("expected predicate last_failure, got %s", fact.Predicate)
	}
	if fact.Value != "rate limit exceeded" {
		t.Fatalf("expected error msg, got %s", fact.Value)
	}
	if fact.Source != "failure_observation" {
		t.Fatalf("expected source failure_observation, got %s", fact.Source)
	}
}

func TestWorldModelCheckFact(t *testing.T) {
	t.Parallel()

	wm := NewWorldModelService()
	_, _ = wm.AddFact("ws1", "api_server", "status", "healthy", "health_check")

	fact, found := wm.CheckFact("ws1", "api_server", "status")
	if !found {
		t.Fatal("expected to find fact")
	}
	if fact.Value != "healthy" {
		t.Fatalf("expected healthy, got %s", fact.Value)
	}

	_, found = wm.CheckFact("ws1", "api_server", "nonexistent")
	if found {
		t.Fatal("expected not to find nonexistent predicate")
	}
}

func TestWorldModelWorkspaceIsolation(t *testing.T) {
	t.Parallel()

	wm := NewWorldModelService()
	_, _ = wm.AddFact("ws1", "tool_a", "status", "active", "test")
	_, _ = wm.AddFact("ws2", "tool_b", "status", "active", "test")

	facts := wm.GetFacts("ws1", "tool_b")
	if len(facts) != 0 {
		t.Fatal("expected no facts from other workspace")
	}
}

func TestWorldModelValidation(t *testing.T) {
	t.Parallel()

	wm := NewWorldModelService()
	_, err := wm.AddFact("", "subject", "pred", "val", "src")
	if err == nil {
		t.Fatal("expected error for empty workspace_id")
	}
	_, err = wm.AddFact("ws1", "", "pred", "val", "src")
	if err == nil {
		t.Fatal("expected error for empty subject")
	}
	_, err = wm.UpdateFromFailure("", "tool", "error")
	if err == nil {
		t.Fatal("expected error for empty workspace_id")
	}
	_, err = wm.UpdateFromFailure("ws1", "", "error")
	if err == nil {
		t.Fatal("expected error for empty tool_name")
	}
}
