package sessions

import (
	"context"
	"testing"
)

func TestCoreferenceResolvePronouns(t *testing.T) {
	t.Parallel()

	svc := NewService()
	svc.EnsureSession("sess-1", "ws-1", "user-1")
	svc.SetEntities("sess-1", []Entity{
		{Key: "object", Value: "invoice #1234"},
		{Key: "person", Value: "Alice"},
	})

	resolver := NewCoreferenceResolver(svc)

	result, err := resolver.Resolve(context.Background(), "sess-1", "Can you send it to her?", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.OriginalText != "Can you send it to her?" {
		t.Fatalf("original text mismatch: %s", result.OriginalText)
	}
	if len(result.Substitutions) < 2 {
		t.Fatalf("expected at least 2 substitutions, got %d", len(result.Substitutions))
	}

	// Check that pronouns were replaced.
	foundIt := false
	foundHer := false
	for _, sub := range result.Substitutions {
		if sub.Original == "it" {
			foundIt = true
			if sub.Resolved != "invoice #1234" {
				t.Fatalf("expected 'it' resolved to 'invoice #1234', got %q", sub.Resolved)
			}
		}
		if sub.Original == "her" {
			foundHer = true
			if sub.Resolved != "Alice" {
				t.Fatalf("expected 'her' resolved to 'Alice', got %q", sub.Resolved)
			}
		}
	}
	if !foundIt {
		t.Fatalf("expected 'it' pronoun resolution")
	}
	if !foundHer {
		t.Fatalf("expected 'her' pronoun resolution")
	}
}

func TestCoreferenceResolveNoEntities(t *testing.T) {
	t.Parallel()

	svc := NewService()
	svc.EnsureSession("sess-2", "ws-1", "user-1")

	resolver := NewCoreferenceResolver(svc)

	result, err := resolver.Resolve(context.Background(), "sess-2", "Tell me about it", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// With no entities, pronouns cannot be resolved.
	if result.ResolvedText != result.OriginalText {
		t.Fatalf("expected no resolution without entities, got %q", result.ResolvedText)
	}
}

func TestCoreferenceResolveWithProvidedEntities(t *testing.T) {
	t.Parallel()

	svc := NewService()
	svc.EnsureSession("sess-3", "ws-1", "user-1")

	resolver := NewCoreferenceResolver(svc)

	result, err := resolver.Resolve(context.Background(), "sess-3", "Update it please", map[string]string{
		"object": "report Q4",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Substitutions) != 1 {
		t.Fatalf("expected 1 substitution, got %d", len(result.Substitutions))
	}
	if result.Substitutions[0].Resolved != "report Q4" {
		t.Fatalf("expected 'report Q4', got %q", result.Substitutions[0].Resolved)
	}
}

func TestCoreferenceResolveEmptySessionID(t *testing.T) {
	t.Parallel()

	svc := NewService()
	resolver := NewCoreferenceResolver(svc)

	_, err := resolver.Resolve(context.Background(), "", "test", nil)
	if err == nil {
		t.Fatalf("expected error for empty session_id")
	}
}

func TestIsFollowUp(t *testing.T) {
	t.Parallel()

	cases := []struct {
		text   string
		expect bool
	}{
		{"also include the summary", true},
		{"And then send it", true},
		{"what about the budget?", true},
		{"how about next quarter?", true},
		{"Create a new report", false},
		{"Show me the dashboard", false},
		{"additionally check the logs", true},
	}
	for _, tc := range cases {
		got := IsFollowUp(tc.text)
		if got != tc.expect {
			t.Errorf("IsFollowUp(%q) = %v, want %v", tc.text, got, tc.expect)
		}
	}
}

func TestCoreferenceResolveFollowUp(t *testing.T) {
	t.Parallel()

	svc := NewService()
	svc.EnsureSession("sess-fu", "ws-1", "user-1")

	resolver := NewCoreferenceResolver(svc)
	resolver.SetContinuation("sess-fu", "generate_report", map[string]string{
		"object": "monthly report",
	})

	result, err := resolver.ResolveFollowUp(context.Background(), "sess-fu", "also include last quarter")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.OriginalText != "also include last quarter" {
		t.Fatalf("original text mismatch")
	}
}

func TestCoreferenceSetContinuation(t *testing.T) {
	t.Parallel()

	svc := NewService()
	resolver := NewCoreferenceResolver(svc)

	resolver.SetContinuation("s1", "intent_a", map[string]string{"topic": "sales"})

	resolver.mu.RLock()
	c, ok := resolver.continuations["s1"]
	resolver.mu.RUnlock()

	if !ok {
		t.Fatalf("expected continuation to be stored")
	}
	if c.PreviousIntent != "intent_a" {
		t.Fatalf("expected intent_a, got %s", c.PreviousIntent)
	}
	if c.Entities["topic"] != "sales" {
		t.Fatalf("expected sales, got %s", c.Entities["topic"])
	}
}
