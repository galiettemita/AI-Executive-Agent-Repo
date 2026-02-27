package codebase_intel

import "testing"

func TestCodebaseIntelLifecycle(t *testing.T) {
	s := NewService()
	if len(s.ListDependencies("default")) == 0 {
		t.Fatalf("expected seeded dependencies")
	}

	debt := s.UpsertDebt("", DebtItem{
		WorkspaceID: "ws_1",
		Title:       "Refactor handler duplication",
		Severity:    "high",
		Status:      "open",
	})
	task := s.AddDebtTask(debt.ID, DebtTask{WorkspaceID: "ws_1", Title: "Extract shared helper"})
	if task.ID == "" {
		t.Fatalf("expected debt task id")
	}
	if _, ok := s.GetDebtTask(debt.ID, task.ID); !ok {
		t.Fatalf("expected debt task lookup")
	}
	s.UpsertDebtTask(debt.ID, task.ID, DebtTask{WorkspaceID: "ws_1", Title: "Extract shared helper", Status: "completed"})

	template := s.AddTemplate(ProjectTemplate{WorkspaceID: "ws_1", Name: "service_template"})
	if template.ID == "" {
		t.Fatalf("expected template id")
	}
	export := s.CreateContextExport(ContextExport{WorkspaceID: "ws_1", Format: "markdown"})
	if _, ok := s.GetContextExport(export.ID); !ok {
		t.Fatalf("expected context export lookup")
	}
}
