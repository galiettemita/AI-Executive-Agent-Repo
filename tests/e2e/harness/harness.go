// Package harness provides a self-contained E2E test environment using
// in-memory services and httptest, following the project's established
// test patterns (no external containers required).
package harness

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/brevio/brevio/internal/admin"
	"github.com/brevio/brevio/internal/compliance"
	"github.com/brevio/brevio/internal/control"
)

const (
	E2ETimeout   = 60 * time.Second
	SuiteTimeout = 300 * time.Second
)

// Harness holds all infrastructure for an E2E test run.
// Uses in-memory services and httptest (no external containers).
type Harness struct {
	T          *testing.T
	Server     *httptest.Server
	GatewayURL string
	Workspace  uuid.UUID
	UserID     uuid.UUID

	// Internal services for direct inspection in tests.
	controlSvc    *control.Service
	adminSvc      *admin.Service
	complianceSvc *compliance.Service
	mux           *http.ServeMux
}

// New creates a new Harness with fully wired in-memory services.
// Call defer h.Teardown() immediately after New().
func New(t *testing.T) *Harness {
	t.Helper()

	h := &Harness{
		T:         t,
		Workspace: uuid.New(),
		UserID:    uuid.New(),
	}

	// Create in-memory services (same pattern as control mux tests).
	h.controlSvc = control.NewService("e2e-test-secret")
	h.mux = control.NewMux(h.controlSvc)

	// Start httptest server.
	h.Server = httptest.NewServer(h.mux)
	h.GatewayURL = h.Server.URL

	t.Logf("E2E harness ready: gateway=%s workspace=%s", h.GatewayURL, h.Workspace)
	return h
}

// Teardown stops the httptest server and cleans up resources.
func (h *Harness) Teardown() {
	if h.Server != nil {
		h.Server.Close()
	}
}

// RestartGatewayContainer stops and restarts the gateway HTTP server,
// simulating a pod restart. Same in-memory services are re-wired to a new mux.
func (h *Harness) RestartGatewayContainer(t *testing.T) {
	t.Helper()
	if h.Server != nil {
		h.Server.Close()
	}
	// Re-create mux with same control service (preserves in-memory state).
	h.mux = control.NewMux(h.controlSvc)
	h.Server = httptest.NewServer(h.mux)
	h.GatewayURL = h.Server.URL
	t.Logf("Gateway restarted: %s", h.GatewayURL)
}

// CallJSON sends a JSON request to the gateway and returns the response.
func (h *Harness) CallJSON(t *testing.T, method, path string, body []byte) *http.Response {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), E2ETimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, method, h.GatewayURL+path, nil)
	if err != nil {
		t.Fatalf("harness.CallJSON: new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if body != nil {
		req.Body = http.NoBody
		req, _ = http.NewRequestWithContext(ctx, method, h.GatewayURL+path, bytesReader(body))
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("harness.CallJSON: do request: %v", err)
	}
	return resp
}

// WorkspaceID returns the test workspace ID as a string.
func (h *Harness) WorkspaceID() string {
	return h.Workspace.String()
}

// UserIDStr returns the test user ID as a string.
func (h *Harness) UserIDStr() string {
	return h.UserID.String()
}

// BaseURL returns the gateway base URL.
func (h *Harness) BaseURL() string {
	return h.GatewayURL
}

// DefaultWorkflowID returns a deterministic workflow ID for a test label.
func (h *Harness) DefaultWorkflowID(label string) string {
	return fmt.Sprintf("e2e-%s-%s", label, h.Workspace.String())
}
