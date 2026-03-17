// Package browser implements the ACI (Autonomous Computer Interface) agent
// that uses vision → LLM → action loops to execute browser objectives.
package browser

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

// aciSystemPrompt instructs claude-sonnet-4-20250514 to output a single raw JSON
// ACIAction — no markdown, no code fences, no surrounding text whatsoever.
const aciSystemPrompt = `You are a browser automation agent. You receive a screenshot of a browser viewport and an objective.

Respond with ONLY a raw JSON object. No markdown. No code fences. No text before or after the JSON.

JSON schema:
{
  "type":    string,   // REQUIRED. Exactly one of: click | type | key | scroll | done | failed
  "x":       int,      // Pixel x. Required when type=click.
  "y":       int,      // Pixel y. Required when type=click.
  "text":    string,   // Text to keyboard-type. Required when type=type.
  "key":     string,   // Key name e.g. Enter, Tab, Escape. Required when type=key.
  "dir":     string,   // Scroll direction: up | down. Required when type=scroll.
  "amount":  int,      // Scroll units 1-10. Required when type=scroll.
  "reason":  string,   // REQUIRED always. One sentence explaining this action choice.
  "done":    bool,     // true only when the objective is fully and verifiably complete.
  "failed":  bool,     // true only when the objective is definitively impossible.
  "failure": string    // Required when failed=true. Why completion is impossible.
}

Rules:
- Always populate "reason".
- Use exact pixel coordinates visible in the screenshot.
- Set done=true the moment the objective is achieved — do not take further actions.
- Set failed=true only for definitively unachievable objectives (page error, missing element after retries).
- Never output anything except the JSON object.`

// ACIAction is one decision returned by the vision-language model.
type ACIAction struct {
	Type    string `json:"type"`
	X       int    `json:"x"`
	Y       int    `json:"y"`
	Text    string `json:"text"`
	Key     string `json:"key"`
	Dir     string `json:"dir"`
	Amount  int    `json:"amount"`
	Reason  string `json:"reason"`
	Done    bool   `json:"done"`
	Failed  bool   `json:"failed"`
	Failure string `json:"failure"`
}

// ACITaskResult is returned by ExecuteTask.
type ACITaskResult struct {
	Success       bool       `json:"success"`
	StepsTaken    int        `json:"steps_taken"`
	FailureReason string     `json:"failure_reason,omitempty"`
	FinalAction   *ACIAction `json:"final_action,omitempty"`
}

// ACIAgent executes natural-language objectives against a live browser session.
type ACIAgent struct {
	browser    *Client
	apiKey     string
	httpClient *http.Client
}

// NewACIAgent constructs an ACIAgent.
// ANTHROPIC_API_KEY is read from environment per Rule R8.
func NewACIAgent(browser *Client) *ACIAgent {
	return &ACIAgent{
		browser: browser,
		apiKey:  os.Getenv("ANTHROPIC_API_KEY"),
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

// ExecuteTask runs the ACI loop until the objective is complete, a failure is
// declared, or maxSteps is exhausted. maxSteps <= 0 defaults to 10.
func (a *ACIAgent) ExecuteTask(
	ctx context.Context,
	sessionID, objective string,
	maxSteps int,
) (*ACITaskResult, error) {
	if maxSteps <= 0 {
		maxSteps = 10
	}
	prev := make([]string, 0, maxSteps)

	for step := 0; step < maxSteps; step++ {
		// (1) Screenshot → base64 PNG
		ssResult, err := a.browser.Screenshot(ctx, sessionID)
		if err != nil {
			return nil, fmt.Errorf("aci step %d screenshot: %w", step, err)
		}
		screenshotB64 := ssResult.DataBase64

		// (2) nextAction → ACIAction
		action, err := a.nextAction(ctx, screenshotB64, objective, prev)
		if err != nil {
			return nil, fmt.Errorf("aci step %d nextAction: %w", step, err)
		}

		// (3) terminal conditions
		if action.Done {
			return &ACITaskResult{
				Success:     true,
				StepsTaken:  step + 1,
				FinalAction: action,
			}, nil
		}
		if action.Failed {
			return &ACITaskResult{
				Success:       false,
				StepsTaken:    step + 1,
				FailureReason: action.Failure,
				FinalAction:   action,
			}, fmt.Errorf("aci failed at step %d: %s", step+1, action.Failure)
		}

		// (4) dispatch
		if err := a.dispatch(ctx, sessionID, action); err != nil {
			return nil, fmt.Errorf("aci step %d dispatch %q: %w", step, action.Type, err)
		}

		prev = append(prev, fmt.Sprintf("step %d [%s]: %s", step+1, action.Type, action.Reason))

		// (5) 500 ms page stabilisation
		time.Sleep(500 * time.Millisecond)
	}

	return &ACITaskResult{
		Success:       false,
		StepsTaken:    maxSteps,
		FailureReason: fmt.Sprintf("maxSteps %d exceeded without completing objective", maxSteps),
	}, fmt.Errorf("aci: exceeded maxSteps %d for %q", maxSteps, objective)
}

// dispatch executes one ACIAction.
func (a *ACIAgent) dispatch(ctx context.Context, sessionID string, action *ACIAction) error {
	switch action.Type {
	case "click":
		return a.browser.Click(ctx, sessionID, action.X, action.Y)
	case "type":
		return a.browser.Type(ctx, sessionID, action.Text)
	case "key":
		return a.browser.KeyPress(ctx, sessionID, action.Key)
	case "scroll":
		dir := action.Dir
		if dir == "" {
			dir = "down"
		}
		amount := action.Amount
		if amount <= 0 {
			amount = 3
		}
		return a.browser.Scroll(ctx, sessionID, dir, amount)
	default:
		return fmt.Errorf("unknown ACIAction type %q", action.Type)
	}
}

// ── Anthropic Messages API types ────────────────────────────────────────────

type aciRequest struct {
	Model     string       `json:"model"`
	MaxTokens int          `json:"max_tokens"`
	System    string       `json:"system"`
	Messages  []aciMessage `json:"messages"`
}

type aciMessage struct {
	Role    string       `json:"role"`
	Content []aciContent `json:"content"`
}

type aciContent struct {
	Type   string     `json:"type"`
	Source *aciSource `json:"source,omitempty"`
	Text   string     `json:"text,omitempty"`
}

type aciSource struct {
	Type      string `json:"type"`
	MediaType string `json:"media_type"`
	Data      string `json:"data"`
}

type aciResponse struct {
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	Error *struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// nextAction POSTs the screenshot and objective to claude-sonnet-4-20250514
// and returns the parsed ACIAction.
func (a *ACIAgent) nextAction(
	ctx context.Context,
	screenshotB64, objective string,
	prev []string,
) (*ACIAction, error) {
	userText := "Objective: " + objective
	if len(prev) > 0 {
		userText += "\n\nActions taken so far:\n"
		for _, p := range prev {
			userText += "  - " + p + "\n"
		}
		userText += "\nGiven the screenshot, what is the next action?"
	} else {
		userText += "\n\nThis is the initial state. What is the first action?"
	}

	payload := aciRequest{
		Model:     "claude-sonnet-4-20250514",
		MaxTokens: 512,
		System:    aciSystemPrompt,
		Messages: []aciMessage{
			{
				Role: "user",
				Content: []aciContent{
					{
						Type: "image",
						Source: &aciSource{
							Type:      "base64",
							MediaType: "image/png",
							Data:      screenshotB64,
						},
					},
					{Type: "text", Text: userText},
				},
			},
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal aciRequest: %w", err)
	}

	req, err := http.NewRequestWithContext(
		ctx, http.MethodPost,
		"https://api.anthropic.com/v1/messages",
		bytes.NewReader(body),
	)
	if err != nil {
		return nil, fmt.Errorf("build anthropic request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", a.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("anthropic http: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read anthropic body: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("anthropic status %d: %s", resp.StatusCode, raw)
	}

	var apiResp aciResponse
	if err := json.Unmarshal(raw, &apiResp); err != nil {
		return nil, fmt.Errorf("unmarshal anthropic response: %w", err)
	}
	if apiResp.Error != nil {
		return nil, fmt.Errorf("anthropic error [%s]: %s", apiResp.Error.Type, apiResp.Error.Message)
	}
	if len(apiResp.Content) == 0 {
		return nil, fmt.Errorf("anthropic returned empty content")
	}

	var action ACIAction
	if err := json.Unmarshal([]byte(apiResp.Content[0].Text), &action); err != nil {
		return nil, fmt.Errorf("parse ACIAction JSON (%q): %w", apiResp.Content[0].Text, err)
	}
	return &action, nil
}
