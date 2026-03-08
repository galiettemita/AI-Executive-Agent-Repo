package brain

import (
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/google/uuid"
)

// CalibrationBucket holds Platt scaling parameters and empirical accuracy for
// a confidence range.
type CalibrationBucket struct {
	ID                string    `json:"id"`
	WorkspaceID       string    `json:"workspace_id"`
	BucketLower       float64   `json:"bucket_lower"`
	BucketUpper       float64   `json:"bucket_upper"`
	PlattScaleA       float64   `json:"platt_scale_a"`
	PlattScaleB       float64   `json:"platt_scale_b"`
	EmpiricalAccuracy float64   `json:"empirical_accuracy"`
	SampleCount       int       `json:"sample_count"`
	CorrectCount      int       `json:"correct_count"`
	LastCalibratedAt  time.Time `json:"last_calibrated_at"`
}

// CalibrationService provides confidence calibration using Platt scaling.
type CalibrationService struct {
	mu      sync.Mutex
	buckets map[string][]CalibrationBucket // workspaceID -> buckets
	now     func() time.Time
}

// NewCalibrationService creates a new calibration service with default buckets.
func NewCalibrationService() *CalibrationService {
	return &CalibrationService{
		buckets: map[string][]CalibrationBucket{},
		now:     func() time.Time { return time.Now().UTC() },
	}
}

// ensureBuckets initializes default buckets for a workspace if not present.
func (s *CalibrationService) ensureBuckets(workspaceID string) {
	if _, ok := s.buckets[workspaceID]; ok {
		return
	}
	ranges := [][2]float64{
		{0.0, 0.2}, {0.2, 0.4}, {0.4, 0.6}, {0.6, 0.8}, {0.8, 1.0},
	}
	var buckets []CalibrationBucket
	for _, r := range ranges {
		buckets = append(buckets, CalibrationBucket{
			ID:               uuid.Must(uuid.NewV7()).String(),
			WorkspaceID:      workspaceID,
			BucketLower:      r[0],
			BucketUpper:      r[1],
			PlattScaleA:      -1.0, // default Platt parameters (identity-ish)
			PlattScaleB:      0.0,
			LastCalibratedAt: s.now(),
		})
	}
	s.buckets[workspaceID] = buckets
}

// findBucket returns the bucket index for a raw confidence value.
func findBucket(buckets []CalibrationBucket, rawConfidence float64) int {
	for i, b := range buckets {
		if rawConfidence >= b.BucketLower && rawConfidence < b.BucketUpper {
			return i
		}
	}
	// If exactly 1.0, use the last bucket.
	if rawConfidence >= 1.0 && len(buckets) > 0 {
		return len(buckets) - 1
	}
	return 0
}

// RecordOutcome records whether a prediction at a given raw confidence was correct.
func (s *CalibrationService) RecordOutcome(workspaceID string, rawConfidence float64, wasCorrect bool) error {
	if workspaceID == "" {
		return fmt.Errorf("workspace_id is required")
	}
	if rawConfidence < 0 || rawConfidence > 1 {
		return fmt.Errorf("raw_confidence must be between 0 and 1")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.ensureBuckets(workspaceID)

	idx := findBucket(s.buckets[workspaceID], rawConfidence)
	bucket := &s.buckets[workspaceID][idx]
	bucket.SampleCount++
	if wasCorrect {
		bucket.CorrectCount++
	}
	if bucket.SampleCount > 0 {
		bucket.EmpiricalAccuracy = float64(bucket.CorrectCount) / float64(bucket.SampleCount)
	}

	return nil
}

// Calibrate applies Platt scaling to transform a raw confidence into a calibrated one.
func (s *CalibrationService) Calibrate(workspaceID string, rawConfidence float64) (float64, error) {
	if workspaceID == "" {
		return 0, fmt.Errorf("workspace_id is required")
	}
	if rawConfidence < 0 || rawConfidence > 1 {
		return 0, fmt.Errorf("raw_confidence must be between 0 and 1")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.ensureBuckets(workspaceID)

	idx := findBucket(s.buckets[workspaceID], rawConfidence)
	bucket := s.buckets[workspaceID][idx]

	// Platt scaling: P(y=1|f) = 1 / (1 + exp(A*f + B))
	calibrated := 1.0 / (1.0 + math.Exp(bucket.PlattScaleA*rawConfidence+bucket.PlattScaleB))

	// Clamp to [0, 1].
	if calibrated < 0 {
		calibrated = 0
	}
	if calibrated > 1 {
		calibrated = 1
	}

	return calibrated, nil
}

// GetBuckets returns all calibration buckets for a workspace.
func (s *CalibrationService) GetBuckets(workspaceID string) []CalibrationBucket {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.ensureBuckets(workspaceID)

	out := make([]CalibrationBucket, len(s.buckets[workspaceID]))
	copy(out, s.buckets[workspaceID])
	return out
}

// Recalibrate updates Platt scaling parameters based on accumulated outcomes.
func (s *CalibrationService) Recalibrate(workspaceID string) error {
	if workspaceID == "" {
		return fmt.Errorf("workspace_id is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.ensureBuckets(workspaceID)

	now := s.now()
	for i := range s.buckets[workspaceID] {
		bucket := &s.buckets[workspaceID][i]
		if bucket.SampleCount < 5 {
			continue // not enough data to recalibrate
		}

		// Simple Platt parameter estimation:
		// Adjust A and B so that the sigmoid maps the bucket midpoint to the empirical accuracy.
		midpoint := (bucket.BucketLower + bucket.BucketUpper) / 2.0
		target := bucket.EmpiricalAccuracy
		if target <= 0 {
			target = 0.01
		}
		if target >= 1 {
			target = 0.99
		}

		// From: target = 1/(1+exp(A*mid + B)), solve for A with B=0:
		// A = ln(1/target - 1) / mid
		if midpoint > 0 {
			bucket.PlattScaleA = math.Log(1.0/target-1.0) / midpoint
		}
		bucket.PlattScaleB = 0
		bucket.LastCalibratedAt = now
	}

	return nil
}
