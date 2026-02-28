package codebase_intel

import "testing"

func TestCodebaseIntelLifecycle(t *testing.T) {
	t.Parallel()

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

func TestCrossRepoAnalysisDeterministicSharedSignals(t *testing.T) {
	t.Parallel()

	s := NewService()
	s.IngestRepository("ws_2", "repo_alpha",
		[]Dependency{
			{Name: "pgx", Version: "v5.7.4"},
			{Name: "chi", Version: "v5.0.12"},
		},
		[]Pattern{
			{Name: "deterministic_handlers", Description: "sorted handler registry"},
		},
	)
	s.IngestRepository("ws_2", "repo_beta",
		[]Dependency{
			{Name: "pgx", Version: "v5.7.4"},
			{Name: "zap", Version: "v1.27.0"},
		},
		[]Pattern{
			{Name: "deterministic_handlers", Description: "same pattern in another repo"},
		},
	)

	report := s.AnalyzeCrossRepo("ws_2")
	if len(report.SharedDependencies) != 1 {
		t.Fatalf("expected one shared dependency, got %+v", report.SharedDependencies)
	}
	if report.SharedDependencies[0].Name != "pgx" || report.SharedDependencies[0].Occurrences != 2 {
		t.Fatalf("unexpected shared dependency report: %+v", report.SharedDependencies[0])
	}
	if len(report.SharedPatterns) != 1 {
		t.Fatalf("expected one shared pattern, got %+v", report.SharedPatterns)
	}
	if report.SharedPatterns[0].Name != "deterministic_handlers" || report.SharedPatterns[0].Occurrences != 2 {
		t.Fatalf("unexpected shared pattern report: %+v", report.SharedPatterns[0])
	}

	deps := s.ListDependencies("ws_2")
	if len(deps) < 2 {
		t.Fatalf("expected repo ingested dependencies, got %+v", deps)
	}
	patterns := s.ListPatterns("ws_2")
	if len(patterns) == 0 {
		t.Fatalf("expected repo ingested patterns")
	}
}
