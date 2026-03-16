//go:build smoke

package smoke_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

var (
	gatewayURL  = envOr("GATEWAY_URL", "http://localhost:18080")
	brainURL    = envOr("BRAIN_URL", "http://localhost:18081")
	handsURL    = envOr("HANDS_URL", "http://localhost:18082")
	executorURL = envOr("EXECUTOR_URL", "http://localhost:18083")
	client      = &http.Client{Timeout: 15 * time.Second}
)

func TestSmoke_GatewayHealthReady(t *testing.T) {
	resp, err := http.Get(gatewayURL + "/healthz/ready")
	assertHTTP(t, resp, err, 200)
}

func TestSmoke_BrainHealthReady(t *testing.T) {
	resp, err := http.Get(brainURL + "/healthz/ready")
	assertHTTP(t, resp, err, 200)
}

func TestSmoke_BrainHealthDeep(t *testing.T) {
	resp, err := http.Get(brainURL + "/health/deep")
	if err != nil {
		t.Fatalf("GET /health/deep: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 && resp.StatusCode != 503 {
		t.Fatalf("expected 200 or 503, got %d", resp.StatusCode)
	}
	var body struct {
		Status string            `json:"status"`
		Checks map[string]string `json:"checks"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.Status == "" {
		t.Error("expected non-empty status")
	}
	if len(body.Checks) == 0 {
		t.Error("expected at least one check")
	}
}

func TestSmoke_IngestValidMessage(t *testing.T) {
	payload, _ := json.Marshal(map[string]any{
		"id":           fmt.Sprintf("smoke-%d", time.Now().UnixNano()),
		"channel":      "whatsapp",
		"content":      "hello",
		"workspace_id": "ws-smoke-test",
	})
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, brainURL+"/v1/brain/ingest", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("POST ingest: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 202 && resp.StatusCode != 200 {
		t.Errorf("expected 2xx, got %d", resp.StatusCode)
	}
}

func TestSmoke_IngestMalformedBody_Returns4xx(t *testing.T) {
	req, _ := http.NewRequest(http.MethodPost, brainURL+"/v1/brain/ingest",
		strings.NewReader(`{"not_valid": true}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 422 && resp.StatusCode != 400 {
		t.Errorf("expected 422 or 400, got %d", resp.StatusCode)
	}
}

func TestSmoke_AllServicesHealthy(t *testing.T) {
	services := map[string]string{
		"gateway": gatewayURL, "brain": brainURL,
		"hands": handsURL, "executor": executorURL,
	}
	for name, base := range services {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			resp, err := http.Get(base + "/healthz/ready")
			if err != nil {
				t.Skipf("%s unreachable: %v", name, err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != 200 {
				t.Errorf("%s returned %d", name, resp.StatusCode)
			}
		})
	}
}

func TestSmoke_MetricsEndpointAvailable(t *testing.T) {
	resp, err := http.Get(brainURL + "/metrics")
	if err != nil {
		t.Fatalf("GET /metrics: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func assertHTTP(t *testing.T, resp *http.Response, err error, wantStatus int) {
	t.Helper()
	if err != nil {
		t.Fatalf("HTTP error: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != wantStatus {
		t.Errorf("expected %d, got %d", wantStatus, resp.StatusCode)
	}
}
