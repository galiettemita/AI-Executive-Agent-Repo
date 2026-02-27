package eval

import "testing"

func TestRAGEvalGate(t *testing.T) {
	s := NewService()
	pass := s.Evaluate("collection_1", 0.85, 0.80)
	if !pass.Pass {
		t.Fatalf("expected passing eval score: %#v", pass)
	}
	fail := s.Evaluate("collection_2", 0.70, 0.60)
	if fail.Pass {
		t.Fatalf("expected failing eval score: %#v", fail)
	}
	if _, ok := s.Get("collection_1"); !ok {
		t.Fatalf("expected eval score lookup")
	}
}
