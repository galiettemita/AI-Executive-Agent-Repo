package cognitive

import (
	"fmt"
	"math"
	"sync"
	"time"
)

// BehavioralBaseline captures the statistical baseline for a metric.
type BehavioralBaseline struct {
	WorkspaceID string
	Metric      string
	Mean        float64
	StdDev      float64
	SampleCount int
	UpdatedAt   time.Time
}

// DriftResult describes whether drift has been detected and its severity.
type DriftResult struct {
	Drifted  bool
	ZScore   float64
	Severity string // none, mild, moderate, severe
	Baseline *BehavioralBaseline
}

// DriftDetector monitors behavioral metrics for concept drift.
type DriftDetector struct {
	mu        sync.RWMutex
	baselines map[string]*BehavioralBaseline // key: workspaceID:metric
}

// NewDriftDetector creates a new DriftDetector.
func NewDriftDetector() *DriftDetector {
	return &DriftDetector{
		baselines: make(map[string]*BehavioralBaseline),
	}
}

func baselineKey(workspaceID, metric string) string {
	return workspaceID + ":" + metric
}

// EstablishBaseline creates a statistical baseline from initial values.
func (d *DriftDetector) EstablishBaseline(workspaceID, metric string, values []float64) (*BehavioralBaseline, error) {
	if len(values) < 2 {
		return nil, fmt.Errorf("need at least 2 values to establish baseline")
	}

	mean := computeMean(values)
	stddev := computeStdDev(values, mean)

	b := &BehavioralBaseline{
		WorkspaceID: workspaceID,
		Metric:      metric,
		Mean:        mean,
		StdDev:      stddev,
		SampleCount: len(values),
		UpdatedAt:   time.Now(),
	}

	d.mu.Lock()
	d.baselines[baselineKey(workspaceID, metric)] = b
	d.mu.Unlock()

	return b, nil
}

// DetectDrift checks if the current value has drifted from the baseline using z-score.
func (d *DriftDetector) DetectDrift(workspaceID, metric string, currentValue float64) (*DriftResult, error) {
	d.mu.RLock()
	b, ok := d.baselines[baselineKey(workspaceID, metric)]
	d.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("no baseline found for workspace %s, metric %s", workspaceID, metric)
	}

	var zScore float64
	if b.StdDev == 0 {
		if currentValue == b.Mean {
			zScore = 0
		} else {
			zScore = 10.0 // Extreme drift if stddev is zero and value differs.
		}
	} else {
		zScore = math.Abs(currentValue-b.Mean) / b.StdDev
	}

	severity := "none"
	drifted := false

	switch {
	case zScore >= 3.0:
		severity = "severe"
		drifted = true
	case zScore >= 2.0:
		severity = "moderate"
		drifted = true
	case zScore >= 1.5:
		severity = "mild"
		drifted = true
	}

	return &DriftResult{
		Drifted:  drifted,
		ZScore:   zScore,
		Severity: severity,
		Baseline: b,
	}, nil
}

// UpdateBaseline performs a rolling update of the baseline with a new value.
func (d *DriftDetector) UpdateBaseline(workspaceID, metric string, newValue float64) {
	d.mu.Lock()
	defer d.mu.Unlock()

	key := baselineKey(workspaceID, metric)
	b, ok := d.baselines[key]
	if !ok {
		return
	}

	// Welford's online algorithm for updating mean and variance.
	b.SampleCount++
	n := float64(b.SampleCount)
	delta := newValue - b.Mean
	b.Mean += delta / n
	delta2 := newValue - b.Mean
	// Running variance update.
	// We track M2 implicitly: M2 = stddev^2 * (n-1)
	m2 := b.StdDev * b.StdDev * (n - 1)
	m2 += delta * delta2
	if n > 1 {
		b.StdDev = math.Sqrt(m2 / (n - 1))
	}
	b.UpdatedAt = time.Now()
}

func computeMean(values []float64) float64 {
	sum := 0.0
	for _, v := range values {
		sum += v
	}
	return sum / float64(len(values))
}

func computeStdDev(values []float64, mean float64) float64 {
	if len(values) < 2 {
		return 0
	}
	sumSq := 0.0
	for _, v := range values {
		d := v - mean
		sumSq += d * d
	}
	return math.Sqrt(sumSq / float64(len(values)-1))
}
