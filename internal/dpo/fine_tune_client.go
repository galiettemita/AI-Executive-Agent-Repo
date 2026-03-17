package dpo

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

// FineTuneRequest is the payload sent to the Anthropic fine-tuning API.
type FineTuneRequest struct {
	Model        string         `json:"model"`
	Method       string         `json:"method"`
	TrainingData []FineTunePair `json:"training_data"`
}

// FineTunePair is a single DPO training example.
type FineTunePair struct {
	Prompt   string `json:"prompt"`
	Chosen   string `json:"chosen"`
	Rejected string `json:"rejected"`
}

// FineTuneResponse contains the fine-tuning job ID.
type FineTuneResponse struct {
	JobID  string `json:"fine_tuning_job_id"`
	Status string `json:"status"`
}

// CheckpointStatus is the poll response.
type CheckpointStatus struct {
	JobID        string  `json:"fine_tuning_job_id"`
	Status       string  `json:"status"`
	CheckpointID *string `json:"fine_tuned_model,omitempty"`
	Error        *string `json:"error,omitempty"`
}

// FineTuneClient submits DPO jobs to the Anthropic fine-tuning API.
type FineTuneClient struct {
	httpClient *http.Client
	baseURL    string
	apiKey     string
}

// NewFineTuneClient creates a FineTuneClient.
func NewFineTuneClient() (*FineTuneClient, error) {
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("dpo.NewFineTuneClient: ANTHROPIC_API_KEY not set")
	}
	baseURL := os.Getenv("ANTHROPIC_FINETUNE_URL")
	if baseURL == "" {
		baseURL = "https://api.anthropic.com/v1/fine-tuning/jobs"
	}
	return &FineTuneClient{
		httpClient: &http.Client{Timeout: 30 * time.Second},
		baseURL:    baseURL,
		apiKey:     apiKey,
	}, nil
}

// SubmitDPOJob converts PreferencePairs to API format and starts a fine-tuning job.
func (c *FineTuneClient) SubmitDPOJob(ctx context.Context, baseModel string, pairs []PreferencePair) (string, error) {
	if len(pairs) == 0 {
		return "", fmt.Errorf("dpo.FineTuneClient.SubmitDPOJob: no pairs provided")
	}
	ftPairs := make([]FineTunePair, len(pairs))
	for i, p := range pairs {
		ftPairs[i] = FineTunePair{Prompt: p.PromptText, Chosen: p.ChosenResponse, Rejected: p.RejectedResponse}
	}
	body, err := json.Marshal(FineTuneRequest{Model: baseModel, Method: "dpo", TrainingData: ftPairs})
	if err != nil {
		return "", fmt.Errorf("dpo.FineTuneClient.SubmitDPOJob marshal: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("x-api-key", c.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("dpo.FineTuneClient.SubmitDPOJob: %w", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return "", fmt.Errorf("dpo.FineTuneClient.SubmitDPOJob: API %d: %s", resp.StatusCode, respBody)
	}
	var ftResp FineTuneResponse
	if err := json.Unmarshal(respBody, &ftResp); err != nil {
		return "", fmt.Errorf("dpo.FineTuneClient.SubmitDPOJob unmarshal: %w", err)
	}
	return ftResp.JobID, nil
}

// PollJobStatus checks fine-tuning job status.
func (c *FineTuneClient) PollJobStatus(ctx context.Context, jobID string) (CheckpointStatus, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/"+jobID, nil)
	if err != nil {
		return CheckpointStatus{}, err
	}
	req.Header.Set("x-api-key", c.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return CheckpointStatus{}, err
	}
	defer resp.Body.Close()
	var status CheckpointStatus
	return status, json.NewDecoder(resp.Body).Decode(&status)
}
