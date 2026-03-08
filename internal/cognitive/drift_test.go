package cognitive

import (
	"testing"
)

func TestEstablishBaselineRequiresMinValues(t *testing.T) {
	t.Parallel()

	dd := NewDriftDetector()

	_, err := dd.EstablishBaseline("ws1", "latency", []float64{100})
	if err == nil {
		t.Fatal("expected error for < 2 values")
	}

	b, err := dd.EstablishBaseline("ws1", "latency", []float64{100, 110, 90, 105, 95})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if b.Mean != 100.0 {
		t.Fatalf("expected mean 100.0, got %f", b.Mean)
	}
	if b.StdDev <= 0 {
		t.Fatalf("expected positive stddev, got %f", b.StdDev)
	}
	if b.SampleCount != 5 {
		t.Fatalf("expected sample count 5, got %d", b.SampleCount)
	}
}

func TestDetectDriftSevere(t *testing.T) {
	t.Parallel()

	dd := NewDriftDetector()
	dd.EstablishBaseline("ws1", "response_time", []float64{50, 52, 48, 51, 49})

	// Far from mean should be severe drift.
	result, err := dd.DetectDrift("ws1", "response_time", 200)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !result.Drifted {
		t.Fatal("expected drift detected for extreme value")
	}
	if result.Severity != "severe" {
		t.Fatalf("expected severe severity, got %s", result.Severity)
	}
}

func TestDetectDriftNone(t *testing.T) {
	t.Parallel()

	dd := NewDriftDetector()
	dd.EstablishBaseline("ws1", "cpu", []float64{50, 52, 48, 51, 49})

	result, err := dd.DetectDrift("ws1", "cpu", 50)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Drifted {
		t.Fatalf("expected no drift for value close to mean, zscore=%f", result.ZScore)
	}
	if result.Severity != "none" {
		t.Fatalf("expected severity none, got %s", result.Severity)
	}
}

func TestDetectDriftNoBaseline(t *testing.T) {
	t.Parallel()

	dd := NewDriftDetector()
	_, err := dd.DetectDrift("ws1", "nonexistent", 50)
	if err == nil {
		t.Fatal("expected error when no baseline exists")
	}
}

func TestUpdateBaselineWelford(t *testing.T) {
	t.Parallel()

	dd := NewDriftDetector()
	dd.EstablishBaseline("ws1", "throughput", []float64{100, 100})

	dd.UpdateBaseline("ws1", "throughput", 200)

	dd.mu.RLock()
	b := dd.baselines[baselineKey("ws1", "throughput")]
	dd.mu.RUnlock()

	if b.SampleCount != 3 {
		t.Fatalf("expected sample count 3 after update, got %d", b.SampleCount)
	}
	// Mean should shift toward 200.
	if b.Mean <= 100.0 {
		t.Fatalf("expected mean > 100 after adding 200, got %f", b.Mean)
	}
}
