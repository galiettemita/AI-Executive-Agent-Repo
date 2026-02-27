package errorlayer

import "testing"

func TestErrorServiceLifecycle(t *testing.T) {
	s := NewService()

	taxonomy := s.ListTaxonomy()
	if len(taxonomy) == 0 {
		t.Fatalf("expected taxonomy items")
	}

	template := s.UpsertTemplate(Template{
		WorkspaceID: "ws_1",
		Persona:     "executive",
		CodePattern: "BUDGET_*",
		Template:    "Budget capacity reached; approval required for additional actions.",
	})
	if template.ID == "" {
		t.Fatalf("expected template id")
	}

	templates := s.ListTemplates("ws_1")
	if len(templates) != 1 {
		t.Fatalf("expected 1 template, got %d", len(templates))
	}
}
