package temporal

import (
	"context"
	"testing"
)

func TestClusterCorrectionsActivity_Valid(t *testing.T) {
	result, err := ClusterCorrectionsActivity(context.Background(), ClusterCorrectionsInput{
		WorkspaceID: "ws-1",
		BatchSize:   50,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected result")
	}
}

func TestClusterCorrectionsActivity_MissingWorkspace(t *testing.T) {
	_, err := ClusterCorrectionsActivity(context.Background(), ClusterCorrectionsInput{})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestDetectConflictsActivity_Valid(t *testing.T) {
	result, err := DetectConflictsActivity(context.Background(), DetectConflictsInput{
		WorkspaceID: "ws-1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected result")
	}
}

func TestResolveConflictActivity_ValidResolution(t *testing.T) {
	result, err := ResolveConflictActivity(context.Background(), ResolveConflictInput{
		ConflictID: "c-1",
		Resolution: "keep_a",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Fatal("expected success")
	}
}

func TestResolveConflictActivity_InvalidResolution(t *testing.T) {
	_, err := ResolveConflictActivity(context.Background(), ResolveConflictInput{
		ConflictID: "c-1",
		Resolution: "invalid",
	})
	if err == nil {
		t.Fatal("expected error for invalid resolution")
	}
}

func TestProposeRulesActivity_Valid(t *testing.T) {
	result, err := ProposeRulesActivity(context.Background(), ProposeRulesInput{
		WorkspaceID: "ws-1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected result")
	}
}
