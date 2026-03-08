package brain

import (
	"testing"
)

func TestRecordOutcome(t *testing.T) {
	svc := NewCalibrationService()

	err := svc.RecordOutcome("ws1", 0.75, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = svc.RecordOutcome("ws1", 0.75, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	buckets := svc.GetBuckets("ws1")
	// 0.75 falls in [0.6, 0.8) bucket.
	var target CalibrationBucket
	for _, b := range buckets {
		if b.BucketLower == 0.6 {
			target = b
		}
	}
	if target.SampleCount != 2 {
		t.Fatalf("expected 2 samples, got %d", target.SampleCount)
	}
	if target.EmpiricalAccuracy != 0.5 {
		t.Fatalf("expected 0.5 accuracy, got %f", target.EmpiricalAccuracy)
	}
}

func TestRecordOutcome_Validation(t *testing.T) {
	svc := NewCalibrationService()

	err := svc.RecordOutcome("", 0.5, true)
	if err == nil {
		t.Fatal("expected error for empty workspace")
	}

	err = svc.RecordOutcome("ws1", 1.5, true)
	if err == nil {
		t.Fatal("expected error for confidence > 1")
	}

	err = svc.RecordOutcome("ws1", -0.1, true)
	if err == nil {
		t.Fatal("expected error for negative confidence")
	}
}

func TestCalibrate(t *testing.T) {
	svc := NewCalibrationService()

	calibrated, err := svc.Calibrate("ws1", 0.5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if calibrated < 0 || calibrated > 1 {
		t.Fatalf("calibrated value out of range: %f", calibrated)
	}
}

func TestGetBuckets(t *testing.T) {
	svc := NewCalibrationService()

	buckets := svc.GetBuckets("ws1")
	if len(buckets) != 5 {
		t.Fatalf("expected 5 buckets, got %d", len(buckets))
	}

	// Check bucket ranges.
	if buckets[0].BucketLower != 0.0 || buckets[0].BucketUpper != 0.2 {
		t.Fatalf("unexpected first bucket range: [%f, %f)", buckets[0].BucketLower, buckets[0].BucketUpper)
	}
	if buckets[4].BucketLower != 0.8 || buckets[4].BucketUpper != 1.0 {
		t.Fatalf("unexpected last bucket range: [%f, %f)", buckets[4].BucketLower, buckets[4].BucketUpper)
	}
}

func TestRecalibrate(t *testing.T) {
	svc := NewCalibrationService()

	// Add enough samples to trigger recalibration.
	for i := 0; i < 8; i++ {
		_ = svc.RecordOutcome("ws1", 0.7, i < 6) // 6/8 = 0.75 accuracy
	}

	err := svc.Recalibrate("ws1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// After recalibration, the Platt parameters should be adjusted.
	buckets := svc.GetBuckets("ws1")
	var target CalibrationBucket
	for _, b := range buckets {
		if b.BucketLower == 0.6 {
			target = b
		}
	}
	if target.PlattScaleA == -1.0 {
		t.Fatal("expected Platt A to be recalibrated from default")
	}

	// Calibrating 0.7 should now give a value closer to 0.75.
	calibrated, _ := svc.Calibrate("ws1", 0.7)
	if calibrated < 0.5 || calibrated > 0.95 {
		t.Fatalf("expected calibrated value near 0.75, got %f", calibrated)
	}
}
