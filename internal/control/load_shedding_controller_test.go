package control

import (
	"testing"
	"time"
)

func TestTargetLoadSheddingTier(t *testing.T) {
	t.Parallel()

	if got := TargetLoadSheddingTier(LoadSheddingMetrics{CPUPercent: 81}); got != "D1" {
		t.Fatalf("unexpected D1 trigger: %s", got)
	}
	if got := TargetLoadSheddingTier(LoadSheddingMetrics{LLMProviderDegraded: true}); got != "D2" {
		t.Fatalf("unexpected D2 trigger: %s", got)
	}
	if got := TargetLoadSheddingTier(LoadSheddingMetrics{MultipleProviderFailures: true}); got != "D3" {
		t.Fatalf("unexpected D3 trigger: %s", got)
	}
	if got := TargetLoadSheddingTier(LoadSheddingMetrics{DBPoolUtilizationPercent: 91}); got != "D4" {
		t.Fatalf("unexpected D4 trigger: %s", got)
	}
}

func TestNextLoadSheddingTierEscalationAndRecovery(t *testing.T) {
	t.Parallel()

	if got := NextLoadSheddingTier("D0", LoadSheddingMetrics{CPUPercent: 82}); got != "D1" {
		t.Fatalf("expected D0->D1, got %s", got)
	}
	if got := NextLoadSheddingTier("D1", LoadSheddingMetrics{CPUPercent: 90, CurrentConditionPersisted: 5 * time.Minute}); got != "D2" {
		t.Fatalf("expected D1->D2, got %s", got)
	}
	if got := NextLoadSheddingTier("D2", LoadSheddingMetrics{CPUPercent: 92, CurrentConditionPersisted: 5 * time.Minute}); got != "D3" {
		t.Fatalf("expected D2->D3, got %s", got)
	}
	if got := NextLoadSheddingTier("D3", LoadSheddingMetrics{CPUPercent: 96, CurrentConditionPersisted: 10 * time.Minute}); got != "D4" {
		t.Fatalf("expected D3->D4, got %s", got)
	}
	if got := NextLoadSheddingTier("D4", LoadSheddingMetrics{OperatorConfirmedRecovery: true, ResolvedConditionPersisted: 5 * time.Minute}); got != "D3" {
		t.Fatalf("expected manual D4->D3 recovery, got %s", got)
	}
	if got := NextLoadSheddingTier("D5", LoadSheddingMetrics{}); got != "D5" {
		t.Fatalf("expected D5 to hold, got %s", got)
	}
	if got := NextLoadSheddingTier("D5", LoadSheddingMetrics{OperatorConfirmedRecovery: true}); got != "D4" {
		t.Fatalf("expected manual D5->D4 recovery, got %s", got)
	}
	if got := NextLoadSheddingTier("D2", LoadSheddingMetrics{ResolvedConditionPersisted: 5 * time.Minute}); got != "D1" {
		t.Fatalf("expected D2->D1 recovery, got %s", got)
	}
}

func TestLoadSheddingManualEmergencyD5(t *testing.T) {
	t.Parallel()

	if got := NextLoadSheddingTier("D4", LoadSheddingMetrics{ManualOperatorEmergencyD5: true}); got != "D5" {
		t.Fatalf("expected manual D5, got %s", got)
	}
	if !LoadSheddingTierChanged("D2", "D3") {
		t.Fatal("expected tier-change detection")
	}
}
