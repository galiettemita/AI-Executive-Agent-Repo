package memory

import (
	"testing"
	"time"
)

func TestWriteGateEnforcesExclusionRules(t *testing.T) {
	t.Parallel()

	svc := NewService()
	svc.AddExclusionRule("ws1", "u1", "do not store")
	_, err := svc.Write("ws1", "u1", "preference", "This says do not store secret")
	if err == nil {
		t.Fatal("expected exclusion rule to block write")
	}
}

func TestWritePolicyRequiresConfirmationAndBlocksDataClass(t *testing.T) {
	t.Parallel()

	svc := NewService()
	svc.SetWritePolicy("ws_policy", WritePolicy{
		RequireConfirmationForTypes: map[string]struct{}{"rule": {}},
		BlockedDataClasses:          map[string]struct{}{"restricted": {}},
	})

	item, err := svc.WriteWithRequest(WriteRequest{
		WorkspaceID:       "ws_policy",
		UserID:            "u_policy",
		MemoryType:        "rule",
		Body:              "always verify recipient",
		DataClass:         "internal",
		SensitivityLabel:  "moderate",
		RetentionPolicyID: "default",
		AllowedProcessors: []string{"control"},
		ContentTrust:      "trusted",
	})
	if err != nil {
		t.Fatalf("write with policy: %v", err)
	}
	if item.Status != StatusNeedsConfirmation {
		t.Fatalf("expected status needs_confirmation, got %s", item.Status)
	}

	_, err = svc.WriteWithRequest(WriteRequest{
		WorkspaceID:       "ws_policy",
		UserID:            "u_policy",
		MemoryType:        "semantic",
		Body:              "sensitive record",
		DataClass:         "restricted",
		SensitivityLabel:  "high",
		RetentionPolicyID: "strict",
		AllowedProcessors: []string{"control"},
		ContentTrust:      "trusted",
	})
	if err == nil {
		t.Fatal("expected restricted data class to be blocked by policy")
	}
}

func TestLifecycleStatusTransitions(t *testing.T) {
	t.Parallel()

	svc := NewService()
	item, err := svc.Write("ws1", "u1", "semantic", "alpha")
	if err != nil {
		t.Fatalf("write item: %v", err)
	}

	item, err = svc.TransitionStatus(item.ID, StatusNeedsConfirmation)
	if err != nil {
		t.Fatalf("transition to needs_confirmation: %v", err)
	}
	item, err = svc.TransitionStatus(item.ID, StatusActive)
	if err != nil {
		t.Fatalf("transition to active: %v", err)
	}
	_, err = svc.TransitionStatus(item.ID, StatusProposed)
	if err == nil {
		t.Fatal("expected invalid reverse transition to fail")
	}
}

func TestRetrievalWorkspaceIsolationAndTrustFilter(t *testing.T) {
	t.Parallel()

	svc := NewService()
	if _, err := svc.WriteWithRequest(WriteRequest{
		WorkspaceID:       "ws1",
		UserID:            "u1",
		MemoryType:        "semantic",
		Body:              "alpha",
		DataClass:         "internal",
		SensitivityLabel:  "moderate",
		RetentionPolicyID: "default",
		AllowedProcessors: []string{"brain"},
		ContentTrust:      "trusted",
	}); err != nil {
		t.Fatalf("write ws1 trusted: %v", err)
	}
	if _, err := svc.WriteWithRequest(WriteRequest{
		WorkspaceID:       "ws1",
		UserID:            "u1",
		MemoryType:        "semantic",
		Body:              "beta",
		DataClass:         "internal",
		SensitivityLabel:  "moderate",
		RetentionPolicyID: "default",
		AllowedProcessors: []string{"brain"},
		ContentTrust:      "untrusted",
	}); err != nil {
		t.Fatalf("write ws1 untrusted: %v", err)
	}
	if _, err := svc.Write("ws2", "u2", "semantic", "gamma"); err != nil {
		t.Fatalf("write ws2: %v", err)
	}

	items := svc.Retrieve("ws1")
	if len(items) != 2 {
		t.Fatalf("expected 2 items for ws1, got %d", len(items))
	}
	trustedOnly := svc.RetrieveWithTrust("ws1", []string{"trusted"})
	if len(trustedOnly) != 1 {
		t.Fatalf("expected 1 trusted item, got %d", len(trustedOnly))
	}
	if trustedOnly[0].ContentTrust != "trusted" {
		t.Fatalf("unexpected trust level: %s", trustedOnly[0].ContentTrust)
	}
}

func TestConsolidationMergesDuplicatesAndExpiresStale(t *testing.T) {
	t.Parallel()

	svc := NewService()
	item1, _ := svc.Write("ws1", "u1", "semantic", "buy milk")
	item2, _ := svc.Write("ws1", "u1", "semantic", "Buy Milk")
	expiresAt := time.Now().UTC().Add(-time.Minute)
	_, _ = svc.WriteWithRequest(WriteRequest{
		WorkspaceID:       "ws1",
		UserID:            "u1",
		MemoryType:        "semantic",
		Body:              "expired memory",
		DataClass:         "internal",
		SensitivityLabel:  "moderate",
		RetentionPolicyID: "default",
		AllowedProcessors: []string{"brain"},
		ContentTrust:      "mixed",
		ExpiresAt:         &expiresAt,
	})

	consolidated := svc.Consolidate("ws1")
	if len(consolidated) != 1 {
		t.Fatalf("expected one consolidated active item, got %d", len(consolidated))
	}
	if consolidated[0].Body != item1.Body {
		t.Fatalf("expected first duplicate to remain canonical, got %s", consolidated[0].Body)
	}

	superseded, ok := svc.GetItem(item2.ID)
	if !ok {
		t.Fatal("expected duplicate item to be present")
	}
	if superseded.Status != StatusSuperseded {
		t.Fatalf("expected duplicate item to be superseded, got %s", superseded.Status)
	}

	canonical, ok := svc.GetItem(item1.ID)
	if !ok {
		t.Fatal("expected canonical item to be present")
	}
	if canonical.EmbeddingVersion <= 1 {
		t.Fatalf("expected canonical embedding version increment, got %d", canonical.EmbeddingVersion)
	}
}
