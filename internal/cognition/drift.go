package cognition

import (
	"fmt"
	"math"
	"strings"
	"sync"
	"time"
)

// BehavioralBaseline represents the statistical baseline for a metric.
type BehavioralBaseline struct {
	WorkspaceID string    `json:"workspace_id"`
	Metric      string    `json:"metric"`
	Mean        float64   `json:"mean"`
	StdDev      float64   `json:"std_dev"`
	SampleCount int       `json:"sample_count"`
	ComputedAt  time.Time `json:"computed_at"`
}

// DriftResult contains the result of a drift detection analysis.
type DriftResult struct {
	Detected   bool               `json:"detected"`
	Metric     string             `json:"metric"`
	Divergence float64            `json:"divergence"`
	Severity   string             `json:"severity"` // none, mild, moderate, severe
	Baseline   *BehavioralBaseline `json:"baseline"`
}

// DriftDetector monitors concept drift in behavioral metrics.
type DriftDetector struct {
	mu        sync.Mutex
	baselines map[string]*BehavioralBaseline // key: workspaceID::metric
	drifts    []DriftResult
}

// NewDriftDetector creates a new DriftDetector.
func NewDriftDetector() *DriftDetector {
	return &DriftDetector{
		baselines: make(map[string]*BehavioralBaseline),
		drifts:    []DriftResult{},
	}
}

func driftKey(workspaceID, metric string) string {
	return workspaceID + "::" + metric
}

// ComputeBaseline computes a statistical baseline from a set of values.
func (d *DriftDetector) ComputeBaseline(workspaceID, metric string, values []float64) (*BehavioralBaseline, error) {
	if strings.TrimSpace(workspaceID) == "" {
		return nil, fmt.Errorf("workspace_id is required")
	}
	if strings.TrimSpace(metric) == "" {
		return nil, fmt.Errorf("metric is required")
	}
	if len(values) < 2 {
		return nil, fmt.Errorf("at least 2 values are required")
	}

	mean := computeMean(values)
	stdDev := computeStdDev(values, mean)

	d.mu.Lock()
	defer d.mu.Unlock()

	baseline := &BehavioralBaseline{
		WorkspaceID: workspaceID,
		Metric:      metric,
		Mean:        mean,
		StdDev:      stdDev,
		SampleCount: len(values),
		ComputedAt:  time.Now().UTC(),
	}

	d.baselines[driftKey(workspaceID, metric)] = baseline
	return baseline, nil
}

// DetectDrift compares recent values against the baseline using JS-divergence approximation.
func (d *DriftDetector) DetectDrift(workspaceID, metric string, recentValues []float64) (*DriftResult, error) {
	if len(recentValues) == 0 {
		return nil, fmt.Errorf("recent values are required")
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	key := driftKey(workspaceID, metric)
	baseline, ok := d.baselines[key]
	if !ok {
		return nil, fmt.Errorf("no baseline found for metric: %s", metric)
	}

	recentMean := computeMean(recentValues)
	recentStdDev := computeStdDev(recentValues, recentMean)

	// Simplified JS-divergence: compare distributions via mean/stddev shift
	divergence := jsDivergenceApprox(baseline.Mean, baseline.StdDev, recentMean, recentStdDev)

	severity := classifySeverity(divergence)
	detected := severity != "none"

	result := &DriftResult{
		Detected:   detected,
		Metric:     metric,
		Divergence: divergence,
		Severity:   severity,
		Baseline:   baseline,
	}

	if detected {
		d.drifts = append(d.drifts, *result)
	}

	return result, nil
}

// jsDivergenceApprox computes a simplified Jensen-Shannon divergence between two Gaussian distributions.
func jsDivergenceApprox(mean1, std1, mean2, std2 float64) float64 {
	if std1 < 1e-10 {
		std1 = 1e-10
	}
	if std2 < 1e-10 {
		std2 = 1e-10
	}

	// KL(P||M) + KL(Q||M) / 2 approximated via normal distribution KL
	kl1 := klDivergenceNormal(mean1, std1, mean2, std2)
	kl2 := klDivergenceNormal(mean2, std2, mean1, std1)

	return (kl1 + kl2) / 2.0
}

// klDivergenceNormal computes KL divergence between two normal distributions.
func klDivergenceNormal(mean1, std1, mean2, std2 float64) float64 {
	return math.Log(std2/std1) + (std1*std1+(mean1-mean2)*(mean1-mean2))/(2*std2*std2) - 0.5
}

func classifySeverity(divergence float64) string {
	if divergence < 0.1 {
		return "none"
	}
	if divergence < 0.5 {
		return "mild"
	}
	if divergence < 1.0 {
		return "moderate"
	}
	return "severe"
}

func computeMean(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
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
		diff := v - mean
		sumSq += diff * diff
	}
	return math.Sqrt(sumSq / float64(len(values)))
}

// GetBaseline returns the baseline for a given workspace and metric.
func (d *DriftDetector) GetBaseline(workspaceID, metric string) (*BehavioralBaseline, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	baseline, ok := d.baselines[driftKey(workspaceID, metric)]
	if !ok {
		return nil, fmt.Errorf("no baseline found for metric: %s", metric)
	}
	return baseline, nil
}

// ListDrifts returns all detected drifts for a workspace.
func (d *DriftDetector) ListDrifts(workspaceID string) []DriftResult {
	d.mu.Lock()
	defer d.mu.Unlock()

	var results []DriftResult
	for _, dr := range d.drifts {
		if dr.Baseline != nil && dr.Baseline.WorkspaceID == workspaceID {
			results = append(results, dr)
		}
	}
	return results
}
