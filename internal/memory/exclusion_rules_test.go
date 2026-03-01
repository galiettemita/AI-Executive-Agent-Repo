package memory

import "testing"

func TestEvaluateMemoryExclusionRules(t *testing.T) {
	t.Parallel()

	rules := []ExclusionRule{
		{Pattern: "salary", Scope: "exact"},
		{Pattern: "medical", Scope: "semantic"},
		{Pattern: "divorce", Scope: "regex"},
	}
	if !EvaluateMemoryExclusionRules("my salary is private", 0.1, rules) {
		t.Fatal("expected exact exclusion match")
	}
	if !EvaluateMemoryExclusionRules("some unrelated text", 0.9, rules) {
		t.Fatal("expected semantic exclusion match")
	}
	if EvaluateMemoryExclusionRules("hello world", 0.1, rules) {
		t.Fatal("did not expect exclusion match")
	}
}
