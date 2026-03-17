// Package benchmark implements the BrevioGAIA agent benchmark suite.
package benchmark

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/google/uuid"
)

// GAIATask is a single benchmark task.
type GAIATask struct {
	ID               string   `json:"id"`
	Tier             string   `json:"tier"`
	Category         string   `json:"category"`
	Description      string   `json:"description"`
	Intent           string   `json:"intent"`
	ExpectedToolKeys []string `json:"expected_tool_keys"`
	PassCriteria     string   `json:"pass_criteria"`
	TimeoutSeconds   int      `json:"timeout_seconds"`
}

// GAIADataset is the full benchmark dataset.
type GAIADataset struct {
	Version     string     `json:"version"`
	Description string     `json:"description"`
	CreatedAt   string     `json:"created_at"`
	Tasks       []GAIATask `json:"tasks"`
}

// LoadDataset reads a GAIADataset from a JSON file.
func LoadDataset(path string) (*GAIADataset, error) {
	if path == "" {
		path = "evals/gaia/brevio_gaia_dataset.json"
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("benchmark.LoadDataset: read %s: %w", path, err)
	}
	var ds GAIADataset
	if err := json.Unmarshal(data, &ds); err != nil {
		return nil, fmt.Errorf("benchmark.LoadDataset: unmarshal: %w", err)
	}
	if len(ds.Tasks) == 0 {
		return nil, fmt.Errorf("benchmark.LoadDataset: no tasks in dataset")
	}
	return &ds, nil
}

// FilterByTier returns tasks matching the tier (empty = all).
func (ds *GAIADataset) FilterByTier(tier string) []GAIATask {
	if tier == "" {
		return ds.Tasks
	}
	var result []GAIATask
	for _, t := range ds.Tasks {
		if t.Tier == tier {
			result = append(result, t)
		}
	}
	return result
}

// BenchmarkRun is a single complete benchmark execution record.
type BenchmarkRun struct {
	ID              uuid.UUID  `json:"id"`
	RunNumber       int        `json:"run_number"`
	TriggeredBy     string     `json:"triggered_by"`
	ModelVersion    string     `json:"model_version"`
	TotalTasks      int        `json:"total_tasks"`
	Passed          int        `json:"passed"`
	Failed          int        `json:"failed"`
	Skipped         int        `json:"skipped"`
	PassRate        float64    `json:"pass_rate"`
	EasyPassRate    *float64   `json:"easy_pass_rate,omitempty"`
	MediumPassRate  *float64   `json:"medium_pass_rate,omitempty"`
	HardPassRate    *float64   `json:"hard_pass_rate,omitempty"`
	PriorPassRate   *float64   `json:"prior_pass_rate,omitempty"`
	DurationSeconds *float64   `json:"duration_seconds,omitempty"`
	Status          string     `json:"status"`
	RegressionAlert bool       `json:"regression_alert"`
	ErrorMessage    *string    `json:"error_message,omitempty"`
	StartedAt       time.Time  `json:"started_at"`
	CompletedAt     *time.Time `json:"completed_at,omitempty"`
}

// TaskResult is the result for one task in a benchmark run.
type TaskResult struct {
	ID            uuid.UUID `json:"id"`
	RunID         uuid.UUID `json:"run_id"`
	TaskID        string    `json:"task_id"`
	Tier          string    `json:"tier"`
	Category      string    `json:"category"`
	Intent        string    `json:"intent"`
	Passed        bool      `json:"passed"`
	PassDetail    string    `json:"pass_detail"`
	ToolsCalled   []string  `json:"tools_called"`
	ExpectedTools []string  `json:"expected_tools"`
	LatencyMs     *int      `json:"latency_ms,omitempty"`
	ErrorMessage  *string   `json:"error_message,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
}

// RegressionThreshold: alert if pass rate drops more than this from prior run.
const RegressionThreshold = 0.05

// GAIARunnerInput is the Temporal workflow input.
type GAIARunnerInput struct {
	DatasetPath  string   `json:"dataset_path"`
	Tiers        []string `json:"tiers,omitempty"`
	WorkspaceID  string   `json:"workspace_id"`
	TriggeredBy  string   `json:"triggered_by"`
	ModelVersion string   `json:"model_version"`
}

// TaskRunInput is the Temporal activity input for a single task.
type TaskRunInput struct {
	RunID       uuid.UUID `json:"run_id"`
	Task        GAIATask  `json:"task"`
	WorkspaceID string    `json:"workspace_id"`
}
