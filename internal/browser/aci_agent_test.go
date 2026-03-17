package browser

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// replayTransport returns pre-configured JSON responses in sequence.
type replayTransport struct {
	bodies []string
	idx    int
}

func (r *replayTransport) RoundTrip(_ *http.Request) (*http.Response, error) {
	i := r.idx
	if i >= len(r.bodies) {
		i = len(r.bodies) - 1
	}
	r.idx++
	rec := httptest.NewRecorder()
	rec.WriteHeader(http.StatusOK)
	rec.Header().Set("Content-Type", "application/json")
	_, _ = rec.WriteString(r.bodies[i])
	return rec.Result(), nil
}

func anthropicWrap(actionJSON string) string {
	escaped := strings.ReplaceAll(actionJSON, `\`, `\\`)
	escaped = strings.ReplaceAll(escaped, `"`, `\"`)
	return `{"content":[{"type":"text","text":"` + escaped + `"}]}`
}

func respDone() string {
	return anthropicWrap(`{"type":"done","reason":"objective complete","done":true,"failed":false,"x":0,"y":0,"amount":0}`)
}

func respClick() string {
	return anthropicWrap(`{"type":"click","x":100,"y":200,"reason":"clicking button","done":false,"failed":false,"amount":0}`)
}

func respFailed() string {
	return anthropicWrap(`{"type":"failed","failed":true,"failure":"element not found","reason":"target absent","done":false,"x":0,"y":0,"amount":0}`)
}

// minPNG is a 1x1 transparent PNG, base64-encoded.
var minPNG = base64.StdEncoding.EncodeToString([]byte{
	0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a,
	0x00, 0x00, 0x00, 0x0d, 0x49, 0x48, 0x44, 0x52,
	0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
	0x08, 0x02, 0x00, 0x00, 0x00, 0x90, 0x77, 0x53,
	0xde, 0x00, 0x00, 0x00, 0x0c, 0x49, 0x44, 0x41,
	0x54, 0x08, 0xd7, 0x63, 0xf8, 0xcf, 0xc0, 0x00,
	0x00, 0x00, 0x02, 0x00, 0x01, 0xe2, 0x21, 0xbc,
	0x33, 0x00, 0x00, 0x00, 0x00, 0x49, 0x45, 0x4e,
	0x44, 0xae, 0x42, 0x60, 0x82,
})

func mockBrowserMCP(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/browser/screenshot":
			// Match existing client.go Screenshot parser: {"result":{"data_base64":"...","width":...,"height":...,"format":"png"}}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"result":{"data_base64":"` + minPNG + `","width":1280,"height":720,"format":"png"}}`))
		case "/v1/browser/click", "/v1/browser/type", "/v1/browser/key", "/v1/browser/scroll":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"ok":true}`))
		default:
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"error":"not found"}`))
		}
	}))
}

func newTestClient(serverURL string) *Client {
	return &Client{baseURL: serverURL, httpClient: http.DefaultClient}
}

func newTestAgent(t *testing.T, browserSrv *httptest.Server, transport http.RoundTripper) *ACIAgent {
	t.Helper()
	agent := NewACIAgent(newTestClient(browserSrv.URL))
	agent.httpClient = &http.Client{Transport: transport}
	agent.apiKey = "test-key"
	return agent
}

func TestACIAgent_DoneOnFirstStep(t *testing.T) {
	srv := mockBrowserMCP(t)
	defer srv.Close()
	rt := &replayTransport{bodies: []string{respDone()}}
	agent := newTestAgent(t, srv, rt)

	result, err := agent.ExecuteTask(context.Background(), "s1", "reach homepage", 10)
	if err != nil {
		t.Fatalf("want nil error, got %v", err)
	}
	if !result.Success {
		t.Fatalf("want success=true")
	}
	if result.StepsTaken != 1 {
		t.Fatalf("want StepsTaken=1, got %d", result.StepsTaken)
	}
	if rt.idx != 1 {
		t.Fatalf("want 1 Anthropic call, got %d", rt.idx)
	}
}

func TestACIAgent_ClickThenDone(t *testing.T) {
	srv := mockBrowserMCP(t)
	defer srv.Close()
	rt := &replayTransport{bodies: []string{respClick(), respDone()}}
	agent := newTestAgent(t, srv, rt)

	result, err := agent.ExecuteTask(context.Background(), "s2", "click submit then finish", 10)
	if err != nil {
		t.Fatalf("want nil error, got %v", err)
	}
	if !result.Success {
		t.Fatalf("want success=true")
	}
	if result.StepsTaken != 2 {
		t.Fatalf("want StepsTaken=2, got %d", result.StepsTaken)
	}
	if rt.idx != 2 {
		t.Fatalf("want 2 Anthropic calls, got %d", rt.idx)
	}
}

func TestACIAgent_FailedAction(t *testing.T) {
	srv := mockBrowserMCP(t)
	defer srv.Close()
	rt := &replayTransport{bodies: []string{respFailed()}}
	agent := newTestAgent(t, srv, rt)

	result, err := agent.ExecuteTask(context.Background(), "s3", "impossible task", 10)
	if err == nil {
		t.Fatalf("want non-nil error for failed task")
	}
	if result.Success {
		t.Fatalf("want success=false")
	}
	const wantReason = "element not found"
	if result.FailureReason != wantReason {
		t.Fatalf("want FailureReason=%q, got %q", wantReason, result.FailureReason)
	}
}

func TestACIAgent_MaxStepsExceeded(t *testing.T) {
	srv := mockBrowserMCP(t)
	defer srv.Close()
	rt := &replayTransport{bodies: []string{respClick()}}
	agent := newTestAgent(t, srv, rt)

	result, err := agent.ExecuteTask(context.Background(), "s4", "never completes", 3)
	if err == nil {
		t.Fatalf("want non-nil error for maxSteps exceeded")
	}
	if result.Success {
		t.Fatalf("want success=false")
	}
	if result.StepsTaken != 3 {
		t.Fatalf("want StepsTaken=3, got %d", result.StepsTaken)
	}
}
