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

// VAPIClient communicates with the VAPI.ai voice API.
type VAPIClient struct {
	httpClient    *http.Client
	apiKey        string
	phoneNumberID string
	webhookURL    string
	webhookSecret string
	baseURL       string
}

// NewVAPIClient returns a configured VAPI client.
func NewVAPIClient(apiKey, phoneNumberID, webhookURL, webhookSecret string) *VAPIClient {
	return &VAPIClient{
		httpClient:    &http.Client{Timeout: 30 * time.Second},
		apiKey:        apiKey,
		phoneNumberID: phoneNumberID,
		webhookURL:    webhookURL,
		webhookSecret: webhookSecret,
		baseURL:       "https://api.vapi.ai",
	}
}

// CreateCallRequest is the payload for initiating a VAPI call.
type CreateCallRequest struct {
	PhoneNumber        string         `json:"phoneNumber"`
	AssistantPrompt    string         `json:"assistantPrompt"`
	FirstMessage       string         `json:"firstMessage"`
	MaxDurationSeconds int            `json:"maxDurationSeconds"`
	Metadata           map[string]any `json:"metadata,omitempty"`
}

// CallResponse is the API response after creating a call.
type CallResponse struct {
	CallID      string    `json:"id"`
	Status      string    `json:"status"`
	PhoneNumber string    `json:"phoneNumber"`
	CreatedAt   time.Time `json:"createdAt"`
}

// CallStatus represents the current state of a call.
type CallStatus struct {
	CallID    string `json:"id"`
	Status    string `json:"status"` // queued, ringing, in_progress, completed, failed
	Duration  int    `json:"duration"`
	EndReason string `json:"endReason,omitempty"`
}

func (v *VAPIClient) Name() string { return "vapi" }

// CreateCall initiates a new outbound call via VAPI.
func (v *VAPIClient) CreateCall(ctx context.Context, req CreateCallRequest) (*CallResponse, error) {
	body := map[string]any{
		"phoneNumberId": v.phoneNumberID,
		"customer": map[string]any{
			"number": req.PhoneNumber,
		},
		"assistant": map[string]any{
			"firstMessage":       req.FirstMessage,
			"model":              map[string]any{"messages": []map[string]string{{"role": "system", "content": req.AssistantPrompt}}},
			"maxDurationSeconds": req.MaxDurationSeconds,
		},
		"metadata": req.Metadata,
		"server": map[string]any{
			"url":    v.webhookURL,
			"secret": v.webhookSecret,
		},
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("vapi: marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, v.baseURL+"/call", bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("vapi: create http request: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+v.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := v.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("vapi: http call: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("vapi: read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("vapi: api error status=%d body=%s", resp.StatusCode, string(respBody))
	}

	var cr CallResponse
	if err := json.Unmarshal(respBody, &cr); err != nil {
		return nil, fmt.Errorf("vapi: unmarshal response: %w", err)
	}
	return &cr, nil
}

// GetCall retrieves the current status of a call.
func (v *VAPIClient) GetCall(ctx context.Context, callID string) (*CallStatus, error) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, v.baseURL+"/call/"+callID, nil)
	if err != nil {
		return nil, fmt.Errorf("vapi: create get request: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+v.apiKey)

	resp, err := v.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("vapi: http get call: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("vapi: read get response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("vapi: get call error status=%d body=%s", resp.StatusCode, string(respBody))
	}

	var cs CallStatus
	if err := json.Unmarshal(respBody, &cs); err != nil {
		return nil, fmt.Errorf("vapi: unmarshal call status: %w", err)
	}
	return &cs, nil
}

// CancelCall cancels an in-progress call.
func (v *VAPIClient) CancelCall(ctx context.Context, callID string) error {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodDelete, v.baseURL+"/call/"+callID, nil)
	if err != nil {
		return fmt.Errorf("vapi: create cancel request: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+v.apiKey)

	resp, err := v.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("vapi: http cancel call: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("vapi: cancel call error status=%d body=%s", resp.StatusCode, string(respBody))
	}
	return nil
}
