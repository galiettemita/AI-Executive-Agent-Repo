package cognitive

import (
	"testing"
)

func TestStoreCaseValidation(t *testing.T) {
	t.Parallel()

	cl := NewCaseLibrary()

	c, err := cl.StoreCase("ws1", "server is slow", "restart nginx", "success")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if c.ID == "" {
		t.Fatal("expected non-empty ID")
	}
	if c.UseCount != 0 {
		t.Fatalf("expected use count 0, got %d", c.UseCount)
	}

	_, err = cl.StoreCase("ws1", "", "solution", "success")
	if err == nil {
		t.Fatal("expected error for empty problem")
	}

	_, err = cl.StoreCase("ws1", "problem", "", "success")
	if err == nil {
		t.Fatal("expected error for empty solution")
	}
}

func TestFindSimilarCases(t *testing.T) {
	t.Parallel()

	cl := NewCaseLibrary()
	cl.StoreCase("ws1", "database connection timeout error", "increase pool size", "success")
	cl.StoreCase("ws1", "database slow query performance", "add index", "success")
	cl.StoreCase("ws1", "frontend CSS layout broken", "fix flexbox", "partial")
	cl.StoreCase("ws2", "database connection issue", "restart db", "success")

	results := cl.FindSimilar("ws1", "database connection error", 2)
	if len(results) == 0 {
		t.Fatal("expected at least one similar case")
	}
	if len(results) > 2 {
		t.Fatalf("expected at most 2 results, got %d", len(results))
	}
	// First result should be the most similar.
	if results[0].Similarity <= 0 {
		t.Fatalf("expected positive similarity, got %f", results[0].Similarity)
	}
}

func TestRetrieveAndAdaptCase(t *testing.T) {
	t.Parallel()

	cl := NewCaseLibrary()
	cl.StoreCase("ws1", "user authentication failed error", "check JWT token expiry", "success")
	cl.StoreCase("ws1", "user login failed invalid credentials", "reset password", "failure")

	adapted, err := cl.RetrieveAndAdapt("ws1", "user authentication error")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if adapted.AdaptedSolution == "" {
		t.Fatal("expected non-empty adapted solution")
	}
	if adapted.Confidence <= 0 {
		t.Fatalf("expected positive confidence, got %f", adapted.Confidence)
	}

	// No match case.
	_, err = cl.RetrieveAndAdapt("ws1", "zzzzz completely unrelated qqqqq")
	if err == nil {
		t.Fatal("expected error when no similar cases found")
	}
}

func TestPruneLowReuseCases(t *testing.T) {
	t.Parallel()

	cl := NewCaseLibrary()
	cl.StoreCase("ws1", "problem one detail", "solution one", "success")
	c2, _ := cl.StoreCase("ws1", "problem two detail", "solution two", "success")

	// Manually bump use count by retrieving.
	cl.RetrieveAndAdapt("ws1", "problem two detail")
	_ = c2

	pruned := cl.PruneLowReuseCases("ws1", 1)
	// One case has UseCount=0, one has UseCount=1.
	if pruned != 1 {
		t.Fatalf("expected 1 pruned case, got %d", pruned)
	}
}

func TestJaccardSimilarity(t *testing.T) {
	t.Parallel()

	a := tokenize("hello world foo")
	b := tokenize("hello world bar")

	sim := jaccardSimilarity(a, b)
	if sim <= 0 || sim >= 1 {
		t.Fatalf("expected partial similarity, got %f", sim)
	}

	// Identical sets.
	same := jaccardSimilarity(a, a)
	if same != 1.0 {
		t.Fatalf("expected similarity 1.0 for identical sets, got %f", same)
	}

	// Empty sets.
	empty := jaccardSimilarity(map[string]bool{}, map[string]bool{})
	if empty != 0 {
		t.Fatalf("expected similarity 0 for empty sets, got %f", empty)
	}
}
