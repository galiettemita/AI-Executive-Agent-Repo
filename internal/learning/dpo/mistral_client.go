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

const (
	mistralFineTuneURL = "https://api.mistral.ai/v1/fine-tuning/jobs"
	mistralFilesURL    = "https://api.mistral.ai/v1/files"
)

// MistralFineTuneClient submits DPO jobs to Mistral's fine-tuning API.
type MistralFineTuneClient struct {
	apiKey     string
	httpClient *http.Client
	logger     *slog.Logger
}

// NewMistralFineTuneClient creates a Mistral fine-tuning client.
func NewMistralFineTuneClient(logger *slog.Logger) (*MistralFineTuneClient, error) {
	apiKey := os.Getenv("MISTRAL_API_KEY")
	if apiKey == "" {
		return nil, ErrMistralFineTuneDisabled
	}

	return &MistralFineTuneClient{
		apiKey:     apiKey,
		httpClient: &http.Client{Timeout: 60 * time.Second},
		logger:     logger,
	}, nil
}

func (c *MistralFineTuneClient) ProviderName() string { return "mistral" }

func (c *MistralFineTuneClient) CreateFineTuneJob(ctx context.Context, req FineTuneRequest) (*FineTuneJob, error) {
	// Step 1: Upload training data as JSONL file.
	fileID, err := c.uploadTrainingFile(ctx, req.PreferencePairs)
	if err != nil {
		return nil, fmt.Errorf("upload training file: %w", err)
	}

	// Step 2: Create fine-tuning job.
	trainingSteps := req.HyperParams.NEpochs * len(req.PreferencePairs)
	if trainingSteps < 1 {
		trainingSteps = len(req.PreferencePairs)
	}

	payload := map[string]interface{}{
		"model":          req.BaseModel,
		"training_files": []string{fileID},
		"hyperparameters": map[string]interface{}{
			"training_steps": trainingSteps,
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, mistralFineTuneURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("mistral request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("mistral returned status %d: %s", resp.StatusCode, respBody)
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
		Provider:  "mistral",
		Status:    result.Status,
		BaseModel: req.BaseModel,
		CreatedAt: time.Now(),
	}, nil
}

func (c *MistralFineTuneClient) uploadTrainingFile(ctx context.Context, pairs []PreferencePair) (string, error) {
	var jsonlBuf bytes.Buffer
	for _, p := range pairs {
		line := map[string]string{
			"prompt":   p.PromptText,
			"chosen":   p.ChosenResponse,
			"rejected": p.RejectedResponse,
		}
		_ = json.NewEncoder(&jsonlBuf).Encode(line)
	}

	payload := map[string]interface{}{
		"purpose": "fine-tune",
		"data":    jsonlBuf.String(),
	}
	body, _ := json.Marshal(payload)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, mistralFilesURL, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("create upload request: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("upload request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("upload returned status %d: %s", resp.StatusCode, respBody)
	}

	var result struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode upload response: %w", err)
	}

	return result.ID, nil
}

func (c *MistralFineTuneClient) GetJobStatus(ctx context.Context, jobID string) (*FineTuneJob, error) {
	url := mistralFineTuneURL + "/" + jobID
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		ID             string `json:"id"`
		Status         string `json:"status"`
		FineTunedModel string `json:"fine_tuned_model"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&result)

	return &FineTuneJob{
		JobID:          result.ID,
		Provider:       "mistral",
		Status:         result.Status,
		FineTunedModel: result.FineTunedModel,
	}, nil
}

func (c *MistralFineTuneClient) WaitForCompletion(ctx context.Context, jobID string, timeout time.Duration) (*FineTuneJob, error) {
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

var _ FineTuneClient = (*MistralFineTuneClient)(nil)
