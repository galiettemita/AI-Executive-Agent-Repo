package dpo

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"strings"
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
	apiKey := os.Getenv("OPENAI_API_KEY")

	// Default baseURL is the full OpenAI fine-tuning jobs endpoint.
	// Override the API root via OPENAI_FINETUNE_BASE_URL (e.g. for staging).
	baseURL := "https://api.openai.com/v1/fine_tuning/jobs"
	if u := os.Getenv("OPENAI_FINETUNE_BASE_URL"); u != "" {
		baseURL = strings.TrimRight(u, "/") + "/fine_tuning/jobs"
	}

	return &FineTuneClient{
		apiKey:     apiKey,
		baseURL:    baseURL,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}, nil
}

// SubmitDPOJob implements the two-step OpenAI fine-tuning flow:
// (a) build JSONL → upload to /v1/files → get file_id
// (b) POST to /v1/fine_tuning/jobs with training_file=file_id, model=baseModel
func (c *FineTuneClient) SubmitDPOJob(ctx context.Context, baseModel string, pairs []PreferencePair) (string, error) {
	if len(pairs) == 0 {
		return "", fmt.Errorf("dpo.FineTuneClient.SubmitDPOJob: no pairs provided")
	}

	// ── Step (a): Build JSONL ──────────────────────────────────────────────
	var jsonlBuf bytes.Buffer
	for _, p := range pairs {
		line := map[string]interface{}{
			"messages": []map[string]string{
				{"role": "user", "content": p.PromptText},
				{"role": "assistant", "content": p.ChosenResponse},
			},
		}
		if err := json.NewEncoder(&jsonlBuf).Encode(line); err != nil {
			return "", fmt.Errorf("dpo: jsonl encode: %w", err)
		}
	}

	// ── Step (b): Upload JSONL to /v1/files with purpose=fine-tune ─────────
	filesURL := strings.Replace(c.baseURL, "/fine_tuning/jobs", "/files", 1)

	var formBuf bytes.Buffer
	mw := multipart.NewWriter(&formBuf)
	if err := mw.WriteField("purpose", "fine-tune"); err != nil {
		return "", fmt.Errorf("dpo: write purpose field: %w", err)
	}
	fw, err := mw.CreateFormFile("file", "dpo_training.jsonl")
	if err != nil {
		return "", fmt.Errorf("dpo: create form file: %w", err)
	}
	if _, err := io.Copy(fw, &jsonlBuf); err != nil {
		return "", fmt.Errorf("dpo: copy jsonl: %w", err)
	}
	mw.Close()

	uploadReq, err := http.NewRequestWithContext(ctx, http.MethodPost, filesURL, &formBuf)
	if err != nil {
		return "", fmt.Errorf("dpo: build upload request: %w", err)
	}
	uploadReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	uploadReq.Header.Set("Content-Type", mw.FormDataContentType())

	uploadResp, err := c.httpClient.Do(uploadReq)
	if err != nil {
		return "", fmt.Errorf("dpo: upload file: %w", err)
	}
	defer uploadResp.Body.Close()
	if uploadResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(uploadResp.Body)
		return "", fmt.Errorf("dpo: upload status %d: %s", uploadResp.StatusCode, body)
	}
	var uploadResult struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(uploadResp.Body).Decode(&uploadResult); err != nil {
		return "", fmt.Errorf("dpo: decode upload response: %w", err)
	}
	if uploadResult.ID == "" {
		return "", fmt.Errorf("dpo: upload response missing file id")
	}

	// ── Step (c): Create fine-tuning job ───────────────────────────────────
	jobPayload := map[string]string{
		"training_file": uploadResult.ID,
		"model":         baseModel,
	}
	jobBody, err := json.Marshal(jobPayload)
	if err != nil {
		return "", fmt.Errorf("dpo: marshal job payload: %w", err)
	}

	jobReq, err := http.NewRequestWithContext(
		ctx, http.MethodPost, c.baseURL, bytes.NewReader(jobBody),
	)
	if err != nil {
		return "", fmt.Errorf("dpo: build job request: %w", err)
	}
	jobReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	jobReq.Header.Set("Content-Type", "application/json")

	jobResp, err := c.httpClient.Do(jobReq)
	if err != nil {
		return "", fmt.Errorf("dpo: create job: %w", err)
	}
	defer jobResp.Body.Close()
	if jobResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(jobResp.Body)
		return "", fmt.Errorf("dpo: create job status %d: %s", jobResp.StatusCode, body)
	}
	var jobResult struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(jobResp.Body).Decode(&jobResult); err != nil {
		return "", fmt.Errorf("dpo: decode job response: %w", err)
	}
	if jobResult.ID == "" {
		return "", fmt.Errorf("dpo: job response missing id")
	}

	return jobResult.ID, nil
}

// PollJobStatus checks fine-tuning job status via OpenAI API.
func (c *FineTuneClient) PollJobStatus(ctx context.Context, jobID string) (CheckpointStatus, error) {
	url := c.baseURL + "/" + jobID

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return CheckpointStatus{}, fmt.Errorf("dpo: build poll request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return CheckpointStatus{}, fmt.Errorf("dpo: poll request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return CheckpointStatus{}, fmt.Errorf("dpo: poll status %d: %s", resp.StatusCode, body)
	}

	var raw struct {
		Status         string `json:"status"`           // queued|running|succeeded|failed
		FineTunedModel string `json:"fine_tuned_model"` // set on succeeded
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return CheckpointStatus{}, fmt.Errorf("dpo: decode poll response: %w", err)
	}

	cs := CheckpointStatus{
		JobID:  jobID,
		Status: raw.Status,
	}
	if raw.FineTunedModel != "" {
		cs.CheckpointID = &raw.FineTunedModel
	}

	return cs, nil
}
