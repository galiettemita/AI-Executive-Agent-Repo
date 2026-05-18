package brain

import (
	"testing"
	"time"
)

func TestAddInterruptionRule(t *testing.T) {
	svc := NewProactiveInterruptionService()

	rule, err := svc.AddRule(InterruptionRule{
		WorkspaceID:     "ws1",
		TriggerType:     "deadline",
		Priority:        8,
		Condition:       "overdue task",
		CooldownMinutes: 30,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rule.ID == "" {
		t.Fatal("expected rule to have an ID")
	}

	// Invalid trigger type.
	_, err = svc.AddRule(InterruptionRule{WorkspaceID: "ws1", TriggerType: "invalid", Priority: 5})
	if err == nil {
		t.Fatal("expected error for invalid trigger_type")
	}

	// Invalid priority.
	_, err = svc.AddRule(InterruptionRule{WorkspaceID: "ws1", TriggerType: "anomaly", Priority: 0})
	if err == nil {
		t.Fatal("expected error for invalid priority")
	}

	// Missing workspace.
	_, err = svc.AddRule(InterruptionRule{TriggerType: "reminder", Priority: 5})
	if err == nil {
		t.Fatal("expected error for missing workspace_id")
	}
}

func TestGetRules(t *testing.T) {
	svc := NewProactiveInterruptionService()

	_, _ = svc.AddRule(InterruptionRule{WorkspaceID: "ws1", TriggerType: "deadline", Priority: 5, Condition: "test"})
	_, _ = svc.AddRule(InterruptionRule{WorkspaceID: "ws2", TriggerType: "anomaly", Priority: 3, Condition: "other"})
	_, _ = svc.AddRule(InterruptionRule{WorkspaceID: "ws1", TriggerType: "reminder", Priority: 2, Condition: "another"})

	rules := svc.GetRules("ws1")
	if len(rules) != 2 {
		t.Fatalf("expected 2 rules for ws1, got %d", len(rules))
	}
}

func TestEvaluateInterruptions(t *testing.T) {
	svc := NewProactiveInterruptionService()

	_, _ = svc.AddRule(InterruptionRule{
		WorkspaceID:     "ws1",
		TriggerType:     "deadline",
		Priority:        8,
		Condition:       "overdue",
		CooldownMinutes: 0,
	})
	_, _ = svc.AddRule(InterruptionRule{
		WorkspaceID:     "ws1",
		TriggerType:     "insight",
		Priority:        4,
		Condition:       "pattern found",
		CooldownMinutes: 0,
	})

	candidates := svc.EvaluateInterruptions("ws1", "there is an overdue task")
	if len(candidates) != 2 {
		t.Fatalf("expected 2 candidates, got %d", len(candidates))
	}

	// The overdue rule should have higher urgency (boosted by context match).
	var overdueCandidate InterruptionCandidate
	for _, c := range candidates {
		if c.Urgency > 0.8 {
			overdueCandidate = c
		}
	}
	if overdueCandidate.RuleID == "" {
		t.Fatal("expected high-urgency candidate for overdue rule")
	}
}

func TestEvaluateInterruptionsCooldown(t *testing.T) {
	svc := NewProactiveInterruptionService()
	now := time.Date(2025, 6, 1, 12, 0, 0, 0, time.UTC)
	svc.now = func() time.Time { return now }

	_, _ = svc.AddRule(InterruptionRule{
		WorkspaceID:     "ws1",
		TriggerType:     "reminder",
		Priority:        5,
		Condition:       "follow up",
		CooldownMinutes: 60,
	})

	// First evaluation should produce candidate.
	candidates := svc.EvaluateInterruptions("ws1", "follow up needed")
	if len(candidates) != 1 {
		t.Fatalf("expected 1 candidate, got %d", len(candidates))
	}

	// Second evaluation within cooldown should produce none.
	svc.now = func() time.Time { return now.Add(30 * time.Minute) }
	candidates = svc.EvaluateInterruptions("ws1", "follow up needed")
	if len(candidates) != 0 {
		t.Fatalf("expected 0 candidates during cooldown, got %d", len(candidates))
	}

	// After cooldown, should produce again.
	svc.now = func() time.Time { return now.Add(61 * time.Minute) }
	candidates = svc.EvaluateInterruptions("ws1", "follow up needed")
	if len(candidates) != 1 {
		t.Fatalf("expected 1 candidate after cooldown, got %d", len(candidates))
	}
}

func TestShouldInterrupt(t *testing.T) {
	svc := NewProactiveInterruptionService()

	if svc.ShouldInterrupt(InterruptionCandidate{Urgency: 0.3}) {
		t.Fatal("expected no interrupt for low urgency")
	}
	if !svc.ShouldInterrupt(InterruptionCandidate{Urgency: 0.7}) {
		t.Fatal("expected interrupt for high urgency")
	}
	if !svc.ShouldInterrupt(InterruptionCandidate{Urgency: 0.5}) {
		t.Fatal("expected interrupt at threshold")
	}
}
