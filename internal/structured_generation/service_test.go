package structured_generation

import "testing"

func TestCanonicalizeProposalSortsDeterministically(t *testing.T) {
	t.Parallel()

	svc := NewService()
	proposal := ActionProposal{
		Intent: "execute workflow",
		Actions: []Action{
			{
				Tool:           "tasks.create_task",
				Operation:      "create",
				Params:         map[string]any{"priority": "high"},
				IdempotencyKey: "idem_abcdefghijklmnop",
			},
			{
				Tool:           "calendar.create_event",
				Operation:      "create",
				Params:         map[string]any{"title": "Standup"},
				IdempotencyKey: "idem_abcdefghijklmnpp",
			},
		},
		Risk:             Risk{Impact: "low", RollbackPlan: "delete created entities"},
		RequiresApproval: false,
	}

	canonical, err := svc.CanonicalizeProposal(proposal)
	if err != nil {
		t.Fatalf("canonicalize proposal: %v", err)
	}
	if canonical.Actions[0].Tool != "calendar.create_event" {
		t.Fatalf("expected lexical action ordering, got %#v", canonical.Actions)
	}

	jsonA, err := svc.CanonicalJSON(proposal)
	if err != nil {
		t.Fatalf("canonical json A: %v", err)
	}
	jsonB, err := svc.CanonicalJSON(proposal)
	if err != nil {
		t.Fatalf("canonical json B: %v", err)
	}
	if jsonA != jsonB {
		t.Fatalf("expected deterministic canonical json, got A=%s B=%s", jsonA, jsonB)
	}
}

func TestValidateProposalRejectsInvalidToolKey(t *testing.T) {
	t.Parallel()

	svc := NewService()
	err := svc.ValidateProposal(ActionProposal{
		Intent: "run",
		Actions: []Action{
			{
				Tool:           "Calendar.CreateEvent",
				Operation:      "create",
				IdempotencyKey: "idem_abcdefghijklmnop",
			},
		},
		Risk: Risk{Impact: "medium", RollbackPlan: "manual rollback"},
	})
	if err == nil {
		t.Fatal("expected invalid tool key error")
	}
}

func TestValidateProposalRejectsInvalidIdempotencyKey(t *testing.T) {
	t.Parallel()

	svc := NewService()
	err := svc.ValidateProposal(ActionProposal{
		Intent: "run",
		Actions: []Action{
			{
				Tool:           "calendar.create_event",
				Operation:      "create",
				IdempotencyKey: "bad_key",
			},
		},
		Risk: Risk{Impact: "medium", RollbackPlan: "manual rollback"},
	})
	if err == nil {
		t.Fatal("expected invalid idempotency key error")
	}
}

func TestValidateProposalRejectsActionOverflow(t *testing.T) {
	t.Parallel()

	svc := NewService()
	actions := make([]Action, 0, 9)
	for idx := 0; idx < 9; idx++ {
		actions = append(actions, Action{
			Tool:           "calendar.create_event",
			Operation:      "create",
			IdempotencyKey: "idem_abcdefghijklmnop",
		})
	}
	err := svc.ValidateProposal(ActionProposal{
		Intent:           "run",
		Actions:          actions,
		Risk:             Risk{Impact: "medium", RollbackPlan: "manual rollback"},
		RequiresApproval: true,
	})
	if err == nil {
		t.Fatal("expected max action overflow error")
	}
}
