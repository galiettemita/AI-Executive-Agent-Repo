package observability

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestTracerProviderEnabled(t *testing.T) {
	tp := NewTracerProvider("test-svc", "test", "http://localhost:4318")
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
