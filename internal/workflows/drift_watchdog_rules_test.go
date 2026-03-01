package workflows

import "testing"

func TestEvaluateDriftWatchdog(t *testing.T) {
	t.Parallel()

	out := EvaluateDriftWatchdog(DriftEvaluationInput{ConsecutiveHealthCheckFailures: 3})
	if !out.Quarantine || out.DriftSeverity != "low" || out.RequiresReview {
		t.Fatalf("unexpected low-severity quarantine output: %+v", out)
	}

	out = EvaluateDriftWatchdog(DriftEvaluationInput{SchemaHashMismatch: true})
	if !out.Quarantine || out.DriftSeverity != "elevated" || !out.RequiresReview {
		t.Fatalf("unexpected elevated drift output: %+v", out)
	}

	out = EvaluateDriftWatchdog(DriftEvaluationInput{DeepHealthCheckFailure: true})
	if !out.Quarantine || out.DriftSeverity != "critical" || !out.RequiresReview {
		t.Fatalf("unexpected critical drift output: %+v", out)
	}

	out = EvaluateDriftWatchdog(DriftEvaluationInput{CurrentSeverity: "low", NextHealthCheckPassed: true})
	if out.Quarantine || !out.AutoRestored {
		t.Fatalf("expected low auto-restore, got %+v", out)
	}
}
