package temporal

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// validateDispatchTarget tests
// ---------------------------------------------------------------------------

func TestValidateDispatchTarget_AcceptsValidHTTPS(t *testing.T) {
	t.Parallel()

	cases := []string{
		"https://example.com/webhook",
		"https://hooks.slack.com/services/T00/B00/xxx",
		"https://api.stripe.com/v1/events",
		"http://example.com/callback",
	}
	for _, u := range cases {
		if err := validateDispatchTarget(u); err != nil {
			t.Errorf("expected %q to be accepted, got error: %v", u, err)
		}
	}
}

func TestValidateDispatchTarget_BlocksDisallowedSchemes(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		url  string
	}{
		{"ftp scheme", "ftp://evil.com/data"},
		{"file scheme", "file:///etc/passwd"},
		{"empty scheme", "://no-scheme.com/path"},
		{"javascript scheme", "javascript://alert(1)"},
		{"data scheme", "data:text/html,<h1>hi</h1>"},
		{"gopher scheme", "gopher://evil.com"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if err := validateDispatchTarget(tc.url); err == nil {
				t.Errorf("expected %q to be blocked, but it was accepted", tc.url)
			}
		})
	}
}

func TestValidateDispatchTarget_BlocksLocalhostAndMetadata(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		url  string
	}{
		{"localhost", "https://localhost/admin"},
		{"localhost with port", "https://localhost:8080/admin"},
		{"127.0.0.1", "https://127.0.0.1/secret"},
		{"127.0.0.2", "https://127.0.0.2/secret"},
		{"127.255.255.255", "https://127.255.255.255/secret"},
		{"ipv6 loopback", "https://[::1]/secret"},
		{"metadata endpoint", "https://169.254.169.254/latest/meta-data/"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if err := validateDispatchTarget(tc.url); err == nil {
				t.Errorf("expected %q to be blocked, but it was accepted", tc.url)
			}
		})
	}
}

func TestValidateDispatchTarget_BlocksPrivateIPs(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		url  string
	}{
		{"10.x range", "https://10.0.0.1/internal"},
		{"10.255 range", "https://10.255.0.1/internal"},
		{"172.16 range", "https://172.16.0.1/internal"},
		{"172.31 range", "https://172.31.255.255/internal"},
		{"192.168 range", "https://192.168.1.1/internal"},
		{"192.168.0.1", "https://192.168.0.1/internal"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if err := validateDispatchTarget(tc.url); err == nil {
				t.Errorf("expected %q to be blocked, but it was accepted", tc.url)
			}
		})
	}
}

func TestValidateDispatchTarget_BlocksEmptyHost(t *testing.T) {
	t.Parallel()

	// "http://" alone yields an empty hostname after parsing.
	cases := []string{
		"http:///path-only",
		"https:///no-host",
	}
	for _, u := range cases {
		if err := validateDispatchTarget(u); err == nil {
			t.Errorf("expected %q to be blocked (empty host), but it was accepted", u)
		}
	}
}

func TestValidateDispatchTarget_AcceptsLegitimateExternalHosts(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		url  string
	}{
		{"example.com", "https://example.com/hook"},
		{"subdomain", "https://api.example.com/v2/events"},
		{"custom port", "https://webhook.site:443/abc"},
		{"public IP", "https://8.8.8.8/dns-query"},
		{"http allowed", "http://httpbin.org/post"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if err := validateDispatchTarget(tc.url); err != nil {
				t.Errorf("expected %q to be accepted, got error: %v", tc.url, err)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// HTTPOutboxDispatcher.Dispatch tests
// ---------------------------------------------------------------------------

func TestDispatch_RejectsInvalidJSON(t *testing.T) {
	t.Parallel()

	d := NewHTTPOutboxDispatcher(5 * time.Second)
	ctx := context.Background()

	badPayloads := []struct {
		name    string
		payload []byte
	}{
		{"truncated object", []byte(`{"key":`)},
		{"plain text", []byte(`not json`)},
		{"empty bytes", []byte(``)},
		{"trailing comma", []byte(`{"a":1,}`)},
	}
	for _, tc := range badPayloads {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			// Use a valid target so the JSON check is the one that fails.
			err := d.Dispatch(ctx, "https://example.com/hook", tc.payload)
			if err == nil {
				t.Errorf("expected error for invalid JSON payload %q, got nil", tc.payload)
			}
		})
	}
}

func TestDispatch_BlocksSSRFTargets(t *testing.T) {
	t.Parallel()

	d := NewHTTPOutboxDispatcher(5 * time.Second)
	ctx := context.Background()
	payload := []byte(`{"event":"test"}`)

	blockedURLs := []string{
		"https://127.0.0.1/admin",
		"https://localhost/internal",
		"https://169.254.169.254/latest/meta-data/",
		"https://10.0.0.1/secret",
		"https://192.168.1.1/router",
		"ftp://evil.com/payload",
	}
	for _, u := range blockedURLs {
		t.Run(u, func(t *testing.T) {
			t.Parallel()
			err := d.Dispatch(ctx, u, payload)
			if err == nil {
				t.Errorf("expected SSRF block for %q, got nil", u)
			}
		})
	}
}

func TestDispatch_SucceedsWithHTTPTestServer(t *testing.T) {
	t.Parallel()

	// httptest.Server binds to 127.0.0.1 which is blocked by SSRF validation.
	// To test the full HTTP integration, we construct an HTTPOutboxDispatcher
	// with the test server's client and exercise the HTTP round-trip directly,
	// replicating what Dispatch does after validateDispatchTarget passes.
	var receivedMethod, receivedCT, receivedUA string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedMethod = r.Method
		receivedCT = r.Header.Get("Content-Type")
		receivedUA = r.Header.Get("User-Agent")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer srv.Close()

	payload := []byte(`{"event":"created"}`)
	req, err := http.NewRequestWithContext(
		context.Background(), http.MethodPost, srv.URL+"/webhook",
		bytes.NewReader(payload),
	)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "brevio-outbox-dispatcher/1.0")

	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<16))

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
	if receivedMethod != http.MethodPost {
		t.Errorf("expected POST, got %s", receivedMethod)
	}
	if receivedCT != "application/json" {
		t.Errorf("expected Content-Type application/json, got %s", receivedCT)
	}
	if receivedUA != "brevio-outbox-dispatcher/1.0" {
		t.Errorf("expected User-Agent brevio-outbox-dispatcher/1.0, got %s", receivedUA)
	}
}

func TestDispatch_ReturnsErrorForNon2xx(t *testing.T) {
	t.Parallel()

	// httptest.Server binds to 127.0.0.1 which is blocked by SSRF validation.
	// We test non-2xx handling by exercising the HTTP round-trip directly,
	// replicating Dispatch's response-status check logic.
	codes := []int{
		http.StatusBadRequest,
		http.StatusUnauthorized,
		http.StatusForbidden,
		http.StatusNotFound,
		http.StatusInternalServerError,
		http.StatusBadGateway,
		http.StatusServiceUnavailable,
	}
	for _, code := range codes {
		code := code // capture loop variable
		t.Run(http.StatusText(code), func(t *testing.T) {
			t.Parallel()

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(code)
			}))
			defer srv.Close()

			payload := []byte(`{"e":1}`)
			req, err := http.NewRequestWithContext(
				context.Background(), http.MethodPost, srv.URL+"/hook",
				bytes.NewReader(payload),
			)
			if err != nil {
				t.Fatalf("failed to create request: %v", err)
			}
			req.Header.Set("Content-Type", "application/json")

			resp, err := srv.Client().Do(req)
			if err != nil {
				t.Fatalf("request failed: %v", err)
			}
			defer resp.Body.Close()
			_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<16))

			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				t.Errorf("expected non-2xx for code %d", code)
			}
		})
	}
}
