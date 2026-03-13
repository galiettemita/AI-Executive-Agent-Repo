package observability

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestTracerProviderEnabled(t *testing.T) {
	tp := NewTracerProvider("test-svc", "test", "http://localhost:4318")
	defer tp.Shutdown(context.Background())
	if !tp.Enabled() {
		t.Fatal("expected enabled with endpoint")
	}
	if tp.ServiceName() != "test-svc" {
		t.Fatalf("expected test-svc, got %s", tp.ServiceName())
	}
}

func TestTracerProviderDisabledWithoutEndpoint(t *testing.T) {
	tp := NewTracerProvider("test-svc", "test", "")
	if tp.Enabled() {
		t.Fatal("expected disabled without endpoint")
	}
	if err := tp.Shutdown(context.Background()); err != nil {
		t.Fatalf("shutdown should succeed for disabled provider: %v", err)
	}
}

func TestTracerProviderNoOpDoesNotPanic(t *testing.T) {
	tp := NewTracerProvider("svc", "local", "")
	tp.RecordSpan(SpanData{
		TraceID:   "abc123",
		SpanID:    "def456",
		Name:      "test-span",
		StartNano: time.Now().UnixNano(),
		EndNano:   time.Now().UnixNano(),
	})
	if err := tp.Shutdown(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestTracerProviderExportsToEndpoint(t *testing.T) {
	var received atomic.Int32
	var lastBody []byte

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/traces" && r.Method == http.MethodPost {
			body, _ := io.ReadAll(r.Body)
			lastBody = body
			received.Add(1)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	tp := NewTracerProviderWithVersion("export-test", "1.2.3", "staging", srv.URL)

	tp.RecordSpan(SpanData{
		TraceID:   "aaaa",
		SpanID:    "bbbb",
		Name:      "http.request",
		Kind:      1,
		StartNano: time.Now().UnixNano(),
		EndNano:   time.Now().Add(50 * time.Millisecond).UnixNano(),
		Status:    &SpanStatus{Code: 1},
	})

	// Trigger flush via shutdown.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := tp.Shutdown(ctx); err != nil {
		t.Fatalf("shutdown error: %v", err)
	}

	if received.Load() == 0 {
		t.Fatal("expected at least one export to OTLP endpoint")
	}

	// Validate OTLP payload structure.
	var payload map[string]any
	if err := json.Unmarshal(lastBody, &payload); err != nil {
		t.Fatalf("invalid JSON payload: %v", err)
	}
	rs, ok := payload["resourceSpans"].([]any)
	if !ok || len(rs) == 0 {
		t.Fatal("missing resourceSpans in payload")
	}
}

func TestTracerProviderResourceAttributes(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	tp := NewTracerProviderWithVersion("attr-test", "2.0.0", "production", srv.URL)

	spans := []SpanData{{
		TraceID:   "1111",
		SpanID:    "2222",
		Name:      "test",
		StartNano: time.Now().UnixNano(),
		EndNano:   time.Now().UnixNano(),
	}}
	payload := tp.buildOTLPPayload(spans)

	rs := payload["resourceSpans"].([]map[string]any)
	resource := rs[0]["resource"].(map[string]any)
	attrs := resource["attributes"].([]map[string]any)

	found := map[string]string{}
	for _, a := range attrs {
		key := a["key"].(string)
		val := a["value"].(map[string]any)["stringValue"].(string)
		found[key] = val
	}
	if found["service.name"] != "attr-test" {
		t.Fatalf("expected service.name=attr-test, got %s", found["service.name"])
	}
	if found["deployment.environment"] != "production" {
		t.Fatalf("expected deployment.environment=production, got %s", found["deployment.environment"])
	}
	if found["service.version"] != "2.0.0" {
		t.Fatalf("expected service.version=2.0.0, got %s", found["service.version"])
	}

	if err := tp.Shutdown(context.Background()); err != nil {
		t.Fatalf("shutdown error: %v", err)
	}
}

func TestTracerProviderShutdownIdempotent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	tp := NewTracerProvider("svc", "test", srv.URL)
	if err := tp.Shutdown(context.Background()); err != nil {
		t.Fatalf("first shutdown: %v", err)
	}
	if err := tp.Shutdown(context.Background()); err != nil {
		t.Fatalf("second shutdown should be idempotent: %v", err)
	}
}

func TestPrometheusMetricsHandler(t *testing.T) {
	registry := NewMetricRegistry([]string{
		"brevio_test_gauge",
		"brevio_test_counter",
	})
	pm := NewPrometheusMetrics(registry)
	pm.RecordGauge("brevio_test_gauge", 42.5)
	pm.IncrementCounter("brevio_test_counter", 3)
	pm.IncrementCounter("brevio_test_counter", 2)

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()
	pm.Handler().ServeHTTP(rec, req)

	body := rec.Body.String()
	if rec.Code != 200 {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if !strings.Contains(body, "brevio_build_info") {
		t.Error("missing build info metric")
	}
	if !strings.Contains(body, "brevio_test_gauge 42.5") {
		t.Errorf("missing gauge metric, body: %s", body)
	}
	if !strings.Contains(body, "brevio_test_counter 5.0") {
		t.Errorf("missing counter metric, body: %s", body)
	}
	if !strings.Contains(rec.Header().Get("Content-Type"), "text/plain") {
		t.Error("wrong content type")
	}
}

func TestWorkflowObservabilityHook(t *testing.T) {
	registry := NewMetricRegistry([]string{
		"brevio_workflow_started_total",
		"brevio_workflow_completed_total",
		"brevio_workflow_failed_total",
		"brevio_workflow_last_duration_ms",
		"brevio_activity_completed_total",
		"brevio_activity_failed_total",
	})
	pm := NewPrometheusMetrics(registry)
	hook := NewWorkflowObservabilityHook(pm)

	hook.OnWorkflowStart("MessageProcessingWorkflow", "ws-001")
	hook.OnWorkflowComplete("MessageProcessingWorkflow", "ws-001", 500_000_000, true)
	hook.OnActivityComplete("ValidateEnvelopeActivity", "ws-001", 50_000_000, true)
	hook.OnActivityComplete("ClassifyIntentActivity", "ws-001", 100_000_000, false)

	// Verify counters.
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	if pm.counters["brevio_workflow_started_total"] != 1 {
		t.Error("workflow start counter wrong")
	}
	if pm.counters["brevio_workflow_completed_total"] != 1 {
		t.Error("workflow complete counter wrong")
	}
	if pm.counters["brevio_activity_completed_total"] != 2 {
		t.Error("activity complete counter wrong")
	}
	if pm.counters["brevio_activity_failed_total"] != 1 {
		t.Error("activity failed counter wrong")
	}
}

func TestTraceCorrelationMiddleware(t *testing.T) {
	var capturedTC TraceContext
	inner := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		tc, ok := TraceFromContext(r.Context())
		if !ok {
			t.Fatal("expected trace context in request")
		}
		capturedTC = tc
	})

	handler := TraceCorrelationMiddleware(inner)

	// With traceparent header.
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Traceparent", "00-aaaabbbbccccddddaaaabbbbccccdddd-1122334455667788-01")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if capturedTC.TraceID != "aaaabbbbccccddddaaaabbbbccccdddd" {
		t.Fatalf("expected propagated trace ID, got %s", capturedTC.TraceID)
	}
	if capturedTC.SpanID != "1122334455667788" {
		t.Fatalf("expected propagated span ID, got %s", capturedTC.SpanID)
	}

	// Response should echo traceparent.
	respTP := rec.Header().Get("Traceparent")
	if !strings.Contains(respTP, "aaaabbbbccccddddaaaabbbbccccdddd") {
		t.Fatalf("expected traceparent in response, got %s", respTP)
	}

	// Without traceparent header — should generate IDs.
	req2 := httptest.NewRequest(http.MethodGet, "/", nil)
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)
	if capturedTC.TraceID == "" {
		t.Fatal("expected generated trace ID")
	}
}
