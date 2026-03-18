package cai

import (
	"time"

	"github.com/google/uuid"
)

// ConstitutionalPrinciple represents an active or proposed CAI principle.
type ConstitutionalPrinciple struct {
	ID          uuid.UUID  `json:"id"`
	PrincipleID string     `json:"principle_id"`
	Version     int        `json:"version"`
	Text        string     `json:"text"`
	Status      string     `json:"status"`
	ApprovedBy  *uuid.UUID `json:"approved_by,omitempty"`
	ActivatedAt *time.Time `json:"activated_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
}

// Violation records a detected CAI violation.
type Violation struct {
	ID             uuid.UUID  `json:"id"`
	PrincipleID    string     `json:"principle_id"`
	ViolationType  string     `json:"violation_type"`
	UserCorrection string     `json:"user_correction,omitempty"`
	WorkspaceID    *uuid.UUID `json:"workspace_id,omitempty"`
	RequestID      *uuid.UUID `json:"request_id,omitempty"`
	ViolatedAt     time.Time  `json:"violated_at"`
}

// ProposedPrinciple is a candidate principle awaiting review.
type ProposedPrinciple struct {
	ID                    uuid.UUID `json:"id"`
	Description           string    `json:"description"`
	FailureExamples       []string  `json:"failure_examples"`
	CoverageRate          float64   `json:"coverage_rate"`
	ConflictWithExisting  []string  `json:"conflict_with_existing,omitempty"`
	ProposedAt            time.Time `json:"proposed_at"`
	Status                string    `json:"status"`
}

// ABTestResult holds the outcome of a principle A/B test.
type ABTestResult struct {
	PrincipleID            string  `json:"principle_id"`
	ORMImprovement         float64 `json:"orm_improvement"`
	CorrectionRateReduction float64 `json:"correction_rate_reduction"`
	PValue                 float64 `json:"p_value"`
	Significant            bool    `json:"significant"`
	Recommendation         string  `json:"recommendation"`
}

// LLMClient generates completions for principle discovery.
type LLMClient interface {
	Complete(ctx interface{}, systemPrompt, userPrompt string) (string, error)
}

// FeatureFlagClient manages feature flag rollouts.
type FeatureFlagClient interface {
	EnableForFraction(ctx interface{}, flagKey string, fraction float64, workspaceID string) error
	EnableForAll(ctx interface{}, flagKey string) error
}
