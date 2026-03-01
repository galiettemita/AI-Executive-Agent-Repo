package memory

import "testing"

func TestMemoryConsolidationPolicyThresholds(t *testing.T) {
	t.Parallel()

	if got := ConsolidationCadenceHours(); got != 6 {
		t.Fatalf("unexpected consolidation cadence: %d", got)
	}
	if got := DuplicateMergeThreshold(5000); got != 0.92 {
		t.Fatalf("unexpected standard duplicate threshold: %f", got)
	}
	if got := DuplicateMergeThreshold(10001); got != 0.85 {
		t.Fatalf("unexpected high-volume duplicate threshold: %f", got)
	}
	if !ShouldExpireByStaleness(90) {
		t.Fatal("expected staleness expiry at 90 days")
	}
	if !ShouldSupersedeByConfidence(0.29, 100) {
		t.Fatal("expected low confidence supersede")
	}
	if !ShouldSupersedeByConfidence(0.49, 20000) {
		t.Fatal("expected stricter high-volume confidence supersede")
	}
}

func TestMemoryWriteGatePolicies(t *testing.T) {
	t.Parallel()

	if !ShouldAutoApproveMemoryWrite("task_fact", 0.9, false) {
		t.Fatal("expected task_fact with high confidence to auto-approve")
	}
	if !ShouldAutoApproveMemoryWrite("daily_log", 0.2, false) {
		t.Fatal("expected daily_log to auto-approve")
	}
	if ShouldRequireMemoryConfirmation("rule", 0.95, "PUBLIC", false) == false {
		t.Fatal("expected rule memory writes to require confirmation")
	}
	if ShouldRequireMemoryConfirmation("semantic", 0.65, "PUBLIC", false) == false {
		t.Fatal("expected low-confidence memory writes to require confirmation")
	}
	if ShouldRequireMemoryConfirmation("semantic", 0.95, "HEALTH", false) == false {
		t.Fatal("expected health-class memory writes to require confirmation")
	}
}
