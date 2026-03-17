// Package browser implements the Go client for the browser-mcp TypeScript service.
package browser

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

// Client calls the browser-mcp HTTP service.
type Client struct {
	baseURL    string
	hmacKey    string
	httpClient *http.Client
}

// NewClient creates a Client for the browser-mcp service.
func NewClient() *Client {
	baseURL := os.Getenv("BROWSER_MCP_URL")
	if baseURL == "" {
		baseURL = "http://localhost:7788"
	}
	return &Client{
		baseURL:    baseURL,
		hmacKey:    os.Getenv("BROWSER_MCP_HMAC_KEY"),
		httpClient: &http.Client{Timeout: 60 * time.Second},
	}
}

func (c *Client) post(ctx context.Context, path string, body any, out any) error {
	data, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("browser.Client.post marshal: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("browser.Client.post request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if c.hmacKey != "" {
		mac := hmac.New(sha256.New, []byte(c.hmacKey))
		mac.Write(data)
		req.Header.Set("X-Brevio-HMAC", hex.EncodeToString(mac.Sum(nil)))
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("browser.Client.post %s: %w", path, err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return fmt.Errorf("browser.Client.post %s: HTTP %d: %s", path, resp.StatusCode, respBody)
	}
	if out != nil {
		if err := json.Unmarshal(respBody, out); err != nil {
			return fmt.Errorf("browser.Client.post %s unmarshal: %w", path, err)
		}
	}
	return nil
}

// StartSession creates a new browser session.
func (c *Client) StartSession(ctx context.Context, sessionID, workspaceID, url, sessionType string) error {
	return c.post(ctx, "/v1/browser/session/start", map[string]string{
		"session_id": sessionID, "workspace_id": workspaceID, "url": url, "session_type": sessionType,
	}, nil)
}

// NavigateResult is the response from a navigation operation.
type NavigateResult struct {
	URL        string `json:"url"`
	Title      string `json:"title"`
	StatusCode int    `json:"status_code"`
	BodyText   string `json:"body_text"`
}

// ScrapeResult is the response from a scrape operation.
type ScrapeResult struct {
	URL           string            `json:"url"`
	Title         string            `json:"title"`
	BodyText      string            `json:"body_text"`
	Links         []string          `json:"links"`
	ExtractedData map[string]string `json:"extracted_data"`
}

// FormFillResult is the response from a form fill operation.
type FormFillResult struct {
	Success      bool   `json:"success"`
	SubmissionID string `json:"submission_id"`
	FieldsFilled int    `json:"fields_filled"`
}

// ScreenshotResult is the response from a screenshot operation.
type ScreenshotResult struct {
	DataBase64 string `json:"data_base64"`
	Width      int    `json:"width"`
	Height     int    `json:"height"`
	Format     string `json:"format"`
}

// Navigate performs a simple page load.
func (c *Client) Navigate(ctx context.Context, sessionID, workspaceID, url, sessionType string) (*NavigateResult, error) {
	var resp struct {
		Result NavigateResult `json:"result"`
	}
	err := c.post(ctx, "/v1/browser/navigate", map[string]any{
		"session_id": sessionID, "workspace_id": workspaceID, "url": url, "session_type": sessionType,
	}, &resp)
	return &resp.Result, err
}

// Scrape navigates to the URL and extracts text and links.
func (c *Client) Scrape(ctx context.Context, sessionID, url string, selectors map[string]string) (*ScrapeResult, error) {
	var resp struct {
		Result ScrapeResult `json:"result"`
	}
	err := c.post(ctx, "/v1/browser/scrape", map[string]any{
		"session_id": sessionID, "url": url, "selectors": selectors,
	}, &resp)
	return &resp.Result, err
}

// FormFill fills form fields and optionally submits.
func (c *Client) FormFill(ctx context.Context, sessionID, url string, fields map[string]string, submitSelector string) (*FormFillResult, error) {
	var resp struct {
		Result FormFillResult `json:"result"`
	}
	err := c.post(ctx, "/v1/browser/form-fill", map[string]any{
		"session_id": sessionID, "url": url, "fields": fields, "submit_selector": submitSelector,
	}, &resp)
	return &resp.Result, err
}

// Screenshot captures the current browser viewport.
func (c *Client) Screenshot(ctx context.Context, sessionID string) (*ScreenshotResult, error) {
	var resp struct {
		Result ScreenshotResult `json:"result"`
	}
	err := c.post(ctx, "/v1/browser/screenshot", map[string]string{"session_id": sessionID}, &resp)
	return &resp.Result, err
}

// CloseSession terminates a browser session.
func (c *Client) CloseSession(ctx context.Context, sessionID string) error {
	return c.post(ctx, "/v1/browser/session/close", map[string]string{"session_id": sessionID}, nil)
}

// Health checks if the browser-mcp service is reachable.
func (c *Client) Health(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/health", nil)
	if err != nil {
		return err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("browser-mcp unreachable: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("browser-mcp health: HTTP %d", resp.StatusCode)
	}
	return nil
}
