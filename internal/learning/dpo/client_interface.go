package dpo

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// FineTuneClient defines the interface for all fine-tuning providers.
type FineTuneClient interface {
	CreateFineTuneJob(ctx context.Context, req FineTuneRequest) (*FineTuneJob, error)
	GetJobStatus(ctx context.Context, jobID string) (*FineTuneJob, error)
	WaitForCompletion(ctx context.Context, jobID string, timeout time.Duration) (*FineTuneJob, error)
	ProviderName() string
}

// FineTuneRequest describes a fine-tuning job submission.
type FineTuneRequest struct {
	BaseModel       string
	WorkspaceID     uuid.UUID
	PreferencePairs []PreferencePair
	HyperParams     FineTuneHyperParams
	DPEnabled       bool
	Sigma           float64
	ClipNorm        float64
}

// PreferencePair is one (prompt, chosen, rejected) triple for DPO training.
type PreferencePair struct {
	ID              uuid.UUID `json:"id"`
	WorkspaceID     uuid.UUID `json:"workspace_id"`
	PromptText      string    `json:"prompt_text"`
	ChosenResponse  string    `json:"chosen_response"`
	RejectedResponse string   `json:"rejected_response"`
}

// FineTuneJob represents the status of a fine-tuning job.
type FineTuneJob struct {
	JobID          string     `json:"job_id"`
	Provider       string     `json:"provider"`
	Status         string     `json:"status"` // "queued" | "running" | "succeeded" | "failed"
	BaseModel      string     `json:"base_model"`
	FineTunedModel string     `json:"fine_tuned_model"`
	CreatedAt      time.Time  `json:"created_at"`
	FinishedAt     *time.Time `json:"finished_at,omitempty"`
	Error          string     `json:"error,omitempty"`
}

// FineTuneHyperParams controls training behavior.
type FineTuneHyperParams struct {
	NEpochs      int     `json:"n_epochs"`
	BatchSize    int     `json:"batch_size"`
	LearningRate float64 `json:"learning_rate"`
}

// Common errors.
var (
	ErrAnthropicFineTuneDisabled    = fmt.Errorf("dpo: Anthropic fine-tuning is disabled (set ANTHROPIC_FINETUNE_ENABLED=true)")
	ErrAnthropicFineTuneUnavailable = fmt.Errorf("dpo: Anthropic fine-tuning API not available for this account")
	ErrMistralFineTuneDisabled      = fmt.Errorf("dpo: Mistral fine-tuning is disabled (MISTRAL_API_KEY not set)")
	ErrNoSuitableProvider           = fmt.Errorf("dpo: no suitable fine-tuning provider found")
)

// DefaultHyperParams returns sensible defaults for DPO fine-tuning.
func DefaultHyperParams() FineTuneHyperParams {
	return FineTuneHyperParams{
		NEpochs:      3,
		BatchSize:    4,
		LearningRate: 1e-5,
	}
}
