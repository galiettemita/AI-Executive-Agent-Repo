package a2a

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	openAIBaseURL    = "https://api.openai.com/v1"
	openAIBetaHeader = "assistants=v2"
	httpClientTimeout = 30 * time.Second
	pollInterval      = 2 * time.Second
	runTimeout        = 5 * time.Minute
)

var terminalRunStatuses = map[string]bool{
	"completed": true,
	"failed":    true,
	"cancelled": true,
	"expired":   true,
}

// OpenAIAgentsBridge provides Brevio interoperability with the OpenAI Agents
// SDK via the OpenAI Assistants API v2.
type OpenAIAgentsBridge struct {
	httpClient *http.Client
	apiKey     string
}

// Message represents a single message in an OpenAI thread.
type Message struct {
	ID      string `json:"id"`
	Role    string `json:"role"`
	Content string `json:"content"`
}

// NewOpenAIAgentsBridge returns a new OpenAIAgentsBridge configured with the
// given API key.
func NewOpenAIAgentsBridge(apiKey string) *OpenAIAgentsBridge {
	return &OpenAIAgentsBridge{
		httpClient: &http.Client{Timeout: httpClientTimeout},
		apiKey:     apiKey,
	}
}

func (b *OpenAIAgentsBridge) doRequest(
	ctx context.Context,
	method, path string,
	body interface{},
) (*http.Response, error) {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("openai_bridge: marshal request body for %s %s: %w",
				method, path, err)
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, openAIBaseURL+path, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("openai_bridge: create request %s %s: %w", method, path, err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+b.apiKey)
	req.Header.Set("OpenAI-Beta", openAIBetaHeader)

	resp, err := b.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("openai_bridge: execute %s %s: %w", method, path, err)
	}
	return resp, nil
}

func drainAndDecode(resp *http.Response, dest interface{}) error {
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("openai_bridge: read response body: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("openai_bridge: API error status=%d body=%s",
			resp.StatusCode, string(raw))
	}
	if dest == nil {
		return nil
	}
	if err := json.Unmarshal(raw, dest); err != nil {
		return fmt.Errorf("openai_bridge: decode response (status=%d): %w",
			resp.StatusCode, err)
	}
	return nil
}

// CreateThread creates a new empty thread on the OpenAI Assistants API and
// returns its ID.
func (b *OpenAIAgentsBridge) CreateThread(ctx context.Context) (string, error) {
	resp, err := b.doRequest(ctx, http.MethodPost, "/threads", struct{}{})
	if err != nil {
		return "", fmt.Errorf("openai_bridge: CreateThread: %w", err)
	}

	var result struct {
		ID string `json:"id"`
	}
	if err := drainAndDecode(resp, &result); err != nil {
		return "", fmt.Errorf("openai_bridge: CreateThread: %w", err)
	}
	if result.ID == "" {
		return "", fmt.Errorf("openai_bridge: CreateThread: API returned empty thread ID")
	}
	return result.ID, nil
}

// AddMessage appends a message to an existing thread.
func (b *OpenAIAgentsBridge) AddMessage(
	ctx context.Context,
	threadID, role, content string,
) error {
	if threadID == "" {
		return fmt.Errorf("openai_bridge: AddMessage: threadID is required")
	}
	payload := map[string]string{
		"role":    role,
		"content": content,
	}
	resp, err := b.doRequest(ctx, http.MethodPost,
		"/threads/"+threadID+"/messages", payload)
	if err != nil {
		return fmt.Errorf("openai_bridge: AddMessage: %w", err)
	}
	if err := drainAndDecode(resp, nil); err != nil {
		return fmt.Errorf("openai_bridge: AddMessage: %w", err)
	}
	return nil
}

// RunAndWait starts a run on the given thread with the specified assistant and
// blocks until the run reaches a terminal status or the 5-minute hard timeout
// expires.
func (b *OpenAIAgentsBridge) RunAndWait(
	ctx context.Context,
	threadID, assistantID string,
) (string, error) {
	if threadID == "" {
		return "", fmt.Errorf("openai_bridge: RunAndWait: threadID is required")
	}
	if assistantID == "" {
		return "", fmt.Errorf("openai_bridge: RunAndWait: assistantID is required")
	}

	ctx, cancel := context.WithTimeout(ctx, runTimeout)
	defer cancel()

	payload := map[string]string{"assistant_id": assistantID}
	startResp, err := b.doRequest(ctx, http.MethodPost,
		"/threads/"+threadID+"/runs", payload)
	if err != nil {
		return "", fmt.Errorf("openai_bridge: RunAndWait start: %w", err)
	}

	var run struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	}
	if err := drainAndDecode(startResp, &run); err != nil {
		return "", fmt.Errorf("openai_bridge: RunAndWait start: %w", err)
	}
	if run.ID == "" {
		return "", fmt.Errorf("openai_bridge: RunAndWait: API returned empty run ID")
	}

	if terminalRunStatuses[run.Status] {
		if run.Status != "completed" {
			return run.ID, fmt.Errorf(
				"openai_bridge: RunAndWait: run %s reached terminal status=%s immediately",
				run.ID, run.Status)
		}
		return run.ID, nil
	}

	pollPath := fmt.Sprintf("/threads/%s/runs/%s", threadID, run.ID)
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return "", fmt.Errorf(
				"openai_bridge: RunAndWait: timed out waiting for run %s: %w",
				run.ID, ctx.Err())

		case <-ticker.C:
			pollResp, err := b.doRequest(ctx, http.MethodGet, pollPath, nil)
			if err != nil {
				return "", fmt.Errorf("openai_bridge: RunAndWait poll: %w", err)
			}

			var pollData struct {
				ID     string `json:"id"`
				Status string `json:"status"`
			}
			if err := drainAndDecode(pollResp, &pollData); err != nil {
				return "", fmt.Errorf("openai_bridge: RunAndWait poll decode: %w", err)
			}

			if terminalRunStatuses[pollData.Status] {
				if pollData.Status != "completed" {
					return run.ID, fmt.Errorf(
						"openai_bridge: RunAndWait: run %s ended with status=%s",
						run.ID, pollData.Status)
				}
				return run.ID, nil
			}
		}
	}
}

// GetMessages retrieves all messages from the given thread in ascending
// chronological order (oldest first).
func (b *OpenAIAgentsBridge) GetMessages(
	ctx context.Context,
	threadID string,
) ([]Message, error) {
	if threadID == "" {
		return nil, fmt.Errorf("openai_bridge: GetMessages: threadID is required")
	}

	resp, err := b.doRequest(ctx, http.MethodGet,
		"/threads/"+threadID+"/messages?order=asc", nil)
	if err != nil {
		return nil, fmt.Errorf("openai_bridge: GetMessages: %w", err)
	}

	var envelope struct {
		Data []struct {
			ID      string `json:"id"`
			Role    string `json:"role"`
			Content []struct {
				Type string `json:"type"`
				Text struct {
					Value string `json:"value"`
				} `json:"text"`
			} `json:"content"`
		} `json:"data"`
	}
	if err := drainAndDecode(resp, &envelope); err != nil {
		return nil, fmt.Errorf("openai_bridge: GetMessages: %w", err)
	}

	messages := make([]Message, 0, len(envelope.Data))
	for _, d := range envelope.Data {
		content := ""
		for _, c := range d.Content {
			if c.Type == "text" {
				content += c.Text.Value
			}
		}
		messages = append(messages, Message{
			ID:      d.ID,
			Role:    d.Role,
			Content: content,
		})
	}
	return messages, nil
}
