package temporal

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/brevio/brevio/internal/security"
)

// HTTPOutboxDispatcher dispatches outbox entries via HTTP POST to target URLs.
// It enforces SSRF protections: only allowed schemes, no private/loopback targets.
type HTTPOutboxDispatcher struct {
	client *http.Client
}

// NewHTTPOutboxDispatcher creates a dispatcher with strict timeouts.
func NewHTTPOutboxDispatcher(timeout time.Duration) *HTTPOutboxDispatcher {
	return &HTTPOutboxDispatcher{
		client: &http.Client{
			Timeout: timeout,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= 3 {
					return fmt.Errorf("too many redirects")
				}
				if err := validateDispatchTarget(req.URL.String()); err != nil {
					return fmt.Errorf("redirect blocked: %w", err)
				}
				return nil
			},
		},
	}
}

// Dispatch sends the payload to the target URL via HTTP POST.
// Returns an error if the target is blocked, unreachable, or returns non-2xx.
func (d *HTTPOutboxDispatcher) Dispatch(ctx context.Context, target string, payload []byte) error {
	if err := validateDispatchTarget(target); err != nil {
		return fmt.Errorf("dispatch target blocked: %w", err)
	}

	// Validate payload is well-formed JSON.
	if !json.Valid(payload) {
		return fmt.Errorf("dispatch payload is not valid JSON")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, target, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("dispatch request creation failed: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "brevio-outbox-dispatcher/1.0")

	resp, err := d.client.Do(req)
	if err != nil {
		return fmt.Errorf("dispatch request failed: %w", err)
	}
	defer resp.Body.Close()

	// Consume body to allow connection reuse.
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<16))

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("dispatch target returned HTTP %d", resp.StatusCode)
	}

	return nil
}

// validateDispatchTarget delegates to the shared SSRF validator in
// internal/security, ensuring consistent protection across all outbound targets.
func validateDispatchTarget(raw string) error {
	return security.ValidateTargetURL(raw)
}
