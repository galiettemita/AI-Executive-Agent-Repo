package errorlayer

import (
	"strings"
	"testing"
)

func TestTaxonomyIncludesAppendixBPolicyCodes(t *testing.T) {
	t.Parallel()

	s := NewService()
	taxonomy := s.ListTaxonomy()
	codes := map[string]struct{}{}
	for _, item := range taxonomy {
		codes[item.Code] = struct{}{}
	}

	required := []string{
		"BUDGET_CALLS_EXHAUSTED",
		"CONTEXT_BUDGET_EXCEEDED",
		"RAG_BUDGET_EXCEEDED",
		"GUARDRAIL_BLOCK_ACTIVE",
		"TOOL_QUARANTINED",
		"FEATURE_DISABLED",
		"MODEL_TIER_EXCEEDED",
		"PII_ENCRYPTION_REQUIRED",
		"SANDBOX_VIOLATION",
		"EVENT_SCHEMA_INVALID",
		"EVIDENCE_HASH_MISSING",
		"GOAL_RATE_LIMIT",
		"LESSON_CAP_REACHED",
		"EXPORT_RATE_LIMIT",
		"SELF_MODIFICATION_DENIED",
		"PROMOTION_EXCEEDS_SYSTEM_CAP",
	}
	for _, code := range required {
		if _, ok := codes[code]; !ok {
			t.Fatalf("missing required taxonomy code: %s", code)
		}
	}
}

func TestErrorServiceLifecycle(t *testing.T) {
	t.Parallel()

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

func TestRenderMessagePersonaAwareTemplateSelection(t *testing.T) {
	t.Parallel()

	s := NewService()
	s.UpsertTemplate(Template{
		WorkspaceID: "ws_1",
		Persona:     "executive",
		CodePattern: "FEATURE_DISABLED",
		Template:    "Executive feature access is currently disabled.",
		Status:      "active",
	})
	s.UpsertTemplate(Template{
		WorkspaceID: "ws_1",
		Persona:     "default",
		CodePattern: "FEATURE_*",
		Template:    "Feature is currently disabled by policy.",
		Status:      "active",
	})

	message := s.RenderMessage("ws_1", "executive", "FEATURE_DISABLED", "")
	if message.UserMessage != "Executive feature access is currently disabled." {
		t.Fatalf("unexpected persona-aware message: %#v", message)
	}
	if message.ErrorCode != "FEATURE_DISABLED" {
		t.Fatalf("unexpected error code: %#v", message)
	}
	if message.NextAction == "" {
		t.Fatalf("expected next action guidance: %#v", message)
	}
}

func TestRenderMessageSanitizesInternalReferences(t *testing.T) {
	t.Parallel()

	s := NewService()
	message := s.RenderMessage(
		"ws_1",
		"default",
		"TOOL_QUARANTINED",
		"trace_abcd1234 request_qwerty987654321 550e8400-e29b-41d4-a716-446655440000",
	)
	if strings.Contains(message.UserMessage, "trace_abcd1234") || strings.Contains(message.UserMessage, "550e8400") {
		t.Fatalf("expected internal references to be redacted: %s", message.UserMessage)
	}
	if !strings.Contains(message.UserMessage, "[redacted]") {
		t.Fatalf("expected redaction marker in message: %s", message.UserMessage)
	}
}
