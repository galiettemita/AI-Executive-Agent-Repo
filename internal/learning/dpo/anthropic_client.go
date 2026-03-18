package dpo

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"time"
)

const anthropicFineTuneURL = "https://api.anthropic.com/v1/fine-tuning/jobs"

// AnthropicFineTuneClient submits DPO jobs to Anthropic's fine-tuning API.
type AnthropicFineTuneClient struct {
	apiKey     string
	httpClient *http.Client
	logger     *slog.Logger
}

// NewAnthropicFineTuneClient creates an Anthropic fine-tuning client.
func NewAnthropicFineTuneClient(logger *slog.Logger) (*AnthropicFineTuneClient, error) {
	apiKey := os.Getenv("ANTHROPIC_FINETUNE_API_KEY")
	if apiKey == "" {
		apiKey = os.Getenv("ANTHROPIC_API_KEY")
	}
	if apiKey == "" {
		return nil, fmt.Errorf("ANTHROPIC_FINETUNE_API_KEY or ANTHROPIC_API_KEY required")
	}

	return &AnthropicFineTuneClient{
		apiKey:     apiKey,
		httpClient: &http.Client{Timeout: 60 * time.Second},
		logger:     logger,
	}, nil
}

func (c *AnthropicFineTuneClient) ProviderName() string { return "anthropic" }

func (c *AnthropicFineTuneClient) CreateFineTuneJob(ctx context.Context, req FineTuneRequest) (*FineTuneJob, error) {
	if os.Getenv("ANTHROPIC_FINETUNE_ENABLED") != "true" {
		return nil, ErrAnthropicFineTuneDisabled
	}

	// Build JSONL training data.
	var jsonlBuf bytes.Buffer
	for _, p := range req.PreferencePairs {
		line := map[string]string{
			"prompt":   p.PromptText,
			"chosen":   p.ChosenResponse,
			"rejected": p.RejectedResponse,
		}
		_ = json.NewEncoder(&jsonlBuf).Encode(line)
	}

	payload := map[string]interface{}{
		"model":         req.BaseModel,
		"training_data": jsonlBuf.String(),
		"hyperparameters": map[string]interface{}{
			"n_epochs": req.HyperParams.NEpochs,
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, anthropicFineTuneURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("x-api-key", c.apiKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("anthropic request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrAnthropicFineTuneUnavailable
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("anthropic returned status %d: %s", resp.StatusCode, respBody)
	}

	var result struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &FineTuneJob{
		JobID:     result.ID,
		Provider:  "anthropic",
		Status:    result.Status,
		BaseModel: req.BaseModel,
		CreatedAt: time.Now(),
	}, nil
}

func (c *AnthropicFineTuneClient) GetJobStatus(ctx context.Context, jobID string) (*FineTuneJob, error) {
	url := anthropicFineTuneURL + "/" + jobID
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("x-api-key", c.apiKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("anthropic request: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		ID             string `json:"id"`
		Status         string `json:"status"`
		FineTunedModel string `json:"fine_tuned_model"`
		Error          string `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &FineTuneJob{
		JobID:          result.ID,
		Provider:       "anthropic",
		Status:         result.Status,
		FineTunedModel: result.FineTunedModel,
		Error:          result.Error,
	}, nil
}

func (c *AnthropicFineTuneClient) WaitForCompletion(ctx context.Context, jobID string, timeout time.Duration) (*FineTuneJob, error) {
	deadline := time.Now().Add(timeout)
	for {
		if time.Now().After(deadline) {
			return nil, context.DeadlineExceeded
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(30 * time.Second):
		}
		job, err := c.GetJobStatus(ctx, jobID)
		if err != nil {
			continue
		}
		if job.Status == "succeeded" || job.Status == "failed" {
			return job, nil
		}
	}
}

// Compile-time interface check.
var _ FineTuneClient = (*AnthropicFineTuneClient)(nil)
