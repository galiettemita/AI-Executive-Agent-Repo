package cognition

import (
	"math"
	"testing"
)

func TestComputeBaseline(t *testing.T) {
	d := NewDriftDetector()
	b, err := d.ComputeBaseline("ws1", "latency", []float64{100, 110, 105, 95, 108})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if b.SampleCount != 5 {
		t.Fatalf("expected 5 samples, got %d", b.SampleCount)
	}
	if b.Mean < 90 || b.Mean > 120 {
		t.Fatalf("mean out of expected range: %f", b.Mean)
	}
}

func TestComputeBaselineValidation(t *testing.T) {
	d := NewDriftDetector()
	_, err := d.ComputeBaseline("", "metric", []float64{1, 2})
	if err == nil {
		t.Fatal("expected error for empty workspace")
	}
	_, err = d.ComputeBaseline("ws1", "", []float64{1, 2})
	if err == nil {
		t.Fatal("expected error for empty metric")
	}
	_, err = d.ComputeBaseline("ws1", "m", []float64{1})
	if err == nil {
		t.Fatal("expected error for insufficient values")
	}
}

func TestDetectDriftNone(t *testing.T) {
	d := NewDriftDetector()
	_, _ = d.ComputeBaseline("ws1", "latency", []float64{100, 105, 98, 102, 101})

	result, err := d.DetectDrift("ws1", "latency", []float64{100, 104, 99, 102, 101})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Detected {
		t.Fatalf("expected no drift for similar distributions, divergence: %f", result.Divergence)
	}
	if result.Severity != "none" {
		t.Fatalf("expected severity none, got %s", result.Severity)
	}
}

func TestDetectDriftSevere(t *testing.T) {
	d := NewDriftDetector()
	_, _ = d.ComputeBaseline("ws1", "latency", []float64{100, 105, 98, 102, 101})

	result, err := d.DetectDrift("ws1", "latency", []float64{500, 600, 550, 480, 520})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Detected {
		t.Fatal("expected drift to be detected")
	}
	if result.Severity != "severe" {
		t.Fatalf("expected severe drift, got %s", result.Severity)
	}
}

func TestDetectDriftNoBaseline(t *testing.T) {
	d := NewDriftDetector()
	_, err := d.DetectDrift("ws1", "unknown_metric", []float64{1, 2})
	if err == nil {
		t.Fatal("expected error for missing baseline")
	}
}

func TestDetectDriftEmptyValues(t *testing.T) {
	d := NewDriftDetector()
	_, _ = d.ComputeBaseline("ws1", "m", []float64{1, 2})
	_, err := d.DetectDrift("ws1", "m", nil)
	if err == nil {
		t.Fatal("expected error for empty values")
	}
}

func TestGetBaseline(t *testing.T) {
	d := NewDriftDetector()
	_, _ = d.ComputeBaseline("ws1", "cpu", []float64{50, 55, 52, 48, 51})

	b, err := d.GetBaseline("ws1", "cpu")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if b.Metric != "cpu" {
		t.Fatalf("expected metric cpu, got %s", b.Metric)
	}
}

func TestListDrifts(t *testing.T) {
	d := NewDriftDetector()
	_, _ = d.ComputeBaseline("ws1", "latency", []float64{100, 105, 98})
	_, _ = d.DetectDrift("ws1", "latency", []float64{500, 600, 550})

	drifts := d.ListDrifts("ws1")
	if len(drifts) < 1 {
		t.Fatal("expected at least 1 drift")
	}
}

func TestComputeMeanStdDev(t *testing.T) {
	values := []float64{2, 4, 4, 4, 5, 5, 7, 9}
	mean := computeMean(values)
	if math.Abs(mean-5.0) > 0.01 {
		t.Fatalf("expected mean 5.0, got %f", mean)
	}

	stdDev := computeStdDev(values, mean)
	if stdDev < 1 || stdDev > 3 {
		t.Fatalf("stddev out of expected range: %f", stdDev)
	}
}
