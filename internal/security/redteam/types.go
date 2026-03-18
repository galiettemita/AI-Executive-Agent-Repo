package redteam

import (
	"time"

	"github.com/google/uuid"
)

// RedTeamReport aggregates results from all attack suites in a single run.
type RedTeamReport struct {
	RunID          uuid.UUID       `json:"run_id"`
	GCGResults     []AttackResult  `json:"gcg_results"`
	AutoDANResults []AttackResult  `json:"autodan_results"`
	HarmBench      *HarmBenchReport `json:"harmbench"`
	RunAt          time.Time       `json:"run_at"`
	DurationMs     int64           `json:"duration_ms"`
}

// AttackResult records the outcome of a single adversarial attack attempt.
type AttackResult struct {
	AttackType  string    `json:"attack_type"`
	PayloadHash string    `json:"payload_hash"`
	Blocked     bool      `json:"blocked"`
	BlockLayer  string    `json:"block_layer"`
	LatencyMs   int64     `json:"latency_ms"`
	Timestamp   time.Time `json:"timestamp"`
}

// RedTeamAttempt is the DB row model for the red_team_attempts table.
type RedTeamAttempt struct {
	ID          uuid.UUID `json:"id"`
	AttackType  string    `json:"attack_type"`
	PayloadHash string    `json:"payload_hash"`
	Blocked     bool      `json:"blocked"`
	BlockLayer  string    `json:"block_layer"`
	LatencyMs   int64     `json:"latency_ms"`
	Timestamp   time.Time `json:"timestamp"`
}

// HarmBenchReport holds per-category and overall safety evaluation scores.
type HarmBenchReport struct {
	RunID           uuid.UUID          `json:"run_id"`
	OverallPassRate float64            `json:"overall_pass_rate"`
	CategoryScores  map[string]float64 `json:"category_scores"`
	TotalBehaviors  int                `json:"total_behaviors"`
	BlockedCount    int                `json:"blocked_count"`
	RunAt           time.Time          `json:"run_at"`
}

// HarmBenchBehavior represents a single adversarial behavior prompt.
type HarmBenchBehavior struct {
	ID       string `json:"id"`
	Category string `json:"category"`
	Prompt   string `json:"prompt"`
}

// HarmBenchBehaviorFile is the top-level structure for harmbench_behaviors.json.
type HarmBenchBehaviorFile struct {
	Version    string              `json:"version"`
	Behaviors  []HarmBenchBehavior `json:"behaviors"`
}

// SafetyEvalRequest is the payload sent to the brain safety evaluation endpoint.
type SafetyEvalRequest struct {
	Prompt string `json:"prompt"`
	Mode   string `json:"mode"`
}

// SafetyEvalResponse is the response from the brain safety evaluation endpoint.
type SafetyEvalResponse struct {
	Blocked    bool   `json:"blocked"`
	BlockLayer string `json:"block_layer"`
}

// SecurityScoreRow represents a row in the pg_security_scores table.
type SecurityScoreRow struct {
	ID       uuid.UUID `json:"id"`
	RunID    uuid.UUID `json:"run_id"`
	Category string    `json:"category"`
	PassRate float64   `json:"pass_rate"`
	RunAt    time.Time `json:"run_at"`
}

// PromptInjectionSuite is the structure for evals/prompt_injection_suite.json.
type PromptInjectionSuite struct {
	Version          string                   `json:"version"`
	Description      string                   `json:"description"`
	CIFailThreshold  float64                  `json:"ci_fail_threshold"`
	Cases            []PromptInjectionCase    `json:"cases"`
}

// PromptInjectionCase is a single case in the injection test suite.
type PromptInjectionCase struct {
	ID          string `json:"id"`
	Category    string `json:"category"`
	TrustSource string `json:"trust_source"`
	Input       string `json:"input"`
	Expected    string `json:"expected"`
	Severity    string `json:"severity"`
}

// CAIPrinciples defines the 8 Constitutional AI principles tested by GCG attacks.
var CAIPrinciples = []string{
	"C1-harmlessness",
	"C2-honesty",
	"C3-helpfulness",
	"C4-privacy",
	"C5-fairness",
	"C6-transparency",
	"C7-safety",
	"C8-lawfulness",
}
