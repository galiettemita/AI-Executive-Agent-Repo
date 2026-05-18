package call

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// RetellClient communicates with the Retell AI API as a failover provider.
type RetellClient struct {
	httpClient    *http.Client
	apiKey        string
	phoneNumberID string
	webhookURL    string
	webhookSecret string
	baseURL       string
}

// NewRetellClient returns a configured Retell AI client.
func NewRetellClient(apiKey, phoneNumberID, webhookURL, webhookSecret string) *RetellClient {
	return &RetellClient{
		httpClient:    &http.Client{Timeout: 30 * time.Second},
		apiKey:        apiKey,
		phoneNumberID: phoneNumberID,
		webhookURL:    webhookURL,
		webhookSecret: webhookSecret,
		baseURL:       "https://api.retellai.com",
	}
}

func (r *RetellClient) Name() string { return "retell" }

// CreateCall initiates a new outbound call via Retell AI.
func (r *RetellClient) CreateCall(ctx context.Context, req CreateCallRequest) (*CallResponse, error) {
	body := map[string]any{
		"from_number":          r.phoneNumberID,
		"to_number":            req.PhoneNumber,
		"override_agent_id":    nil,
		"retell_llm_dynamic_variables": map[string]any{
			"system_prompt": req.AssistantPrompt,
			"first_message": req.FirstMessage,
		},
		"metadata": req.Metadata,
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("retell: marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, r.baseURL+"/v2/create-phone-call", bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("retell: create http request: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+r.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := r.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("retell: http call: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("retell: read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("retell: api error status=%d body=%s", resp.StatusCode, string(respBody))
	}

	var raw struct {
		CallID    string `json:"call_id"`
		Status    string `json:"call_status"`
		ToNumber  string `json:"to_number"`
		CreatedAt int64  `json:"start_timestamp"`
	}
	if err := json.Unmarshal(respBody, &raw); err != nil {
		return nil, fmt.Errorf("retell: unmarshal response: %w", err)
	}

	return &CallResponse{
		CallID:      raw.CallID,
		Status:      raw.Status,
		PhoneNumber: raw.ToNumber,
		CreatedAt:   time.Unix(raw.CreatedAt/1000, (raw.CreatedAt%1000)*int64(time.Millisecond)),
	}, nil
}

// GetCall retrieves the current status of a call.
func (r *RetellClient) GetCall(ctx context.Context, callID string) (*CallStatus, error) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, r.baseURL+"/v2/get-call/"+callID, nil)
	if err != nil {
		return nil, fmt.Errorf("retell: create get request: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+r.apiKey)

	resp, err := r.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("retell: http get call: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("retell: read get response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("retell: get call error status=%d body=%s", resp.StatusCode, string(respBody))
	}

	var raw struct {
		CallID        string `json:"call_id"`
		Status        string `json:"call_status"`
		Duration      int    `json:"duration_ms"`
		DisconnectReason string `json:"disconnect_reason"`
	}
	if err := json.Unmarshal(respBody, &raw); err != nil {
		return nil, fmt.Errorf("retell: unmarshal call status: %w", err)
	}

	return &CallStatus{
		CallID:    raw.CallID,
		Status:    raw.Status,
		Duration:  raw.Duration / 1000,
		EndReason: raw.DisconnectReason,
	}, nil
}

// CancelCall cancels an in-progress call.
func (r *RetellClient) CancelCall(ctx context.Context, callID string) error {
	body := map[string]string{"call_id": callID}
	payload, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("retell: marshal cancel: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, r.baseURL+"/v2/end-call", bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("retell: create cancel request: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+r.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := r.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("retell: http cancel call: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("retell: cancel call error status=%d body=%s", resp.StatusCode, string(respBody))
	}
	return nil
}
