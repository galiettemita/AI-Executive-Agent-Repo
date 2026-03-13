package observability

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
)

// traceContextKey is the context key for trace correlation.
type traceContextKey struct{}

// TraceContext holds trace correlation IDs for structured logging.
type TraceContext struct {
	TraceID  string
	SpanID   string
	ParentID string
}

// TracerProvider manages distributed tracing with W3C propagation.
// When OTEL_EXPORTER_OTLP_ENDPOINT is configured, traces are exported
// to the configured collector via OTLP/HTTP JSON. Otherwise, operates in
// passthrough mode with context propagation for structured logging correlation.
type TracerProvider struct {
	serviceName    string
	serviceVersion string
	environment    string
	endpoint       string // OTEL_EXPORTER_OTLP_ENDPOINT
	enabled        bool

	mu       sync.Mutex
	spans    []SpanData
	client   *http.Client
	stopCh   chan struct{}
	doneCh   chan struct{}
}

// SpanData represents a completed span for export.
type SpanData struct {
	TraceID    string         `json:"traceId"`
	SpanID     string         `json:"spanId"`
	ParentID   string         `json:"parentSpanId,omitempty"`
	Name       string         `json:"name"`
	Kind       int            `json:"kind"`
	StartNano  int64          `json:"startTimeUnixNano,string"`
	EndNano    int64          `json:"endTimeUnixNano,string"`
	Attributes map[string]any `json:"attributes,omitempty"`
	Status     *SpanStatus    `json:"status,omitempty"`
}

// SpanStatus represents the status of a span.
type SpanStatus struct {
	Code    int    `json:"code"`
	Message string `json:"message,omitempty"`
}

// NewTracerProvider creates a tracer provider. When endpoint is empty,
// tracing operates in passthrough mode (contexts propagated, not exported).
func NewTracerProvider(serviceName, environment, endpoint string) *TracerProvider {
	return NewTracerProviderWithVersion(serviceName, "", environment, endpoint)
}

// NewTracerProviderWithVersion creates a tracer provider with an explicit service version.
func NewTracerProviderWithVersion(serviceName, serviceVersion, environment, endpoint string) *TracerProvider {
	tp := &TracerProvider{
		serviceName:    serviceName,
		serviceVersion: serviceVersion,
		environment:    environment,
		endpoint:       strings.TrimRight(endpoint, "/"),
		enabled:        endpoint != "",
		spans:          make([]SpanData, 0, 64),
		stopCh:         make(chan struct{}),
		doneCh:         make(chan struct{}),
	}
	if tp.enabled {
		tp.client = &http.Client{Timeout: 10 * time.Second}
		go tp.exportLoop()
	} else {
		close(tp.doneCh)
	}
	return tp
}

// exportLoop periodically flushes buffered spans to the OTLP endpoint.
func (tp *TracerProvider) exportLoop() {
	defer close(tp.doneCh)
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			tp.flush(context.Background())
		case <-tp.stopCh:
			tp.flush(context.Background())
			return
		}
	}
}

// flush exports all buffered spans to the OTLP endpoint.
func (tp *TracerProvider) flush(ctx context.Context) {
	tp.mu.Lock()
	if len(tp.spans) == 0 {
		tp.mu.Unlock()
		return
	}
	batch := tp.spans
	tp.spans = make([]SpanData, 0, 64)
	tp.mu.Unlock()

	payload := tp.buildOTLPPayload(batch)
	body, err := json.Marshal(payload)
	if err != nil {
		return
	}

	url := tp.endpoint + "/v1/traces"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := tp.client.Do(req)
	if err != nil {
		return
	}
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<16))
	resp.Body.Close()
}

// buildOTLPPayload constructs the OTLP/HTTP JSON trace export payload.
func (tp *TracerProvider) buildOTLPPayload(spans []SpanData) map[string]any {
	attrs := []map[string]any{
		{"key": "service.name", "value": map[string]any{"stringValue": tp.serviceName}},
		{"key": "deployment.environment", "value": map[string]any{"stringValue": tp.environment}},
	}
	if tp.serviceVersion != "" {
		attrs = append(attrs, map[string]any{
			"key": "service.version", "value": map[string]any{"stringValue": tp.serviceVersion},
		})
	}

	otlpSpans := make([]map[string]any, 0, len(spans))
	for _, s := range spans {
		span := map[string]any{
			"traceId":            s.TraceID,
			"spanId":             s.SpanID,
			"name":               s.Name,
			"kind":               s.Kind,
			"startTimeUnixNano":  fmt.Sprintf("%d", s.StartNano),
			"endTimeUnixNano":    fmt.Sprintf("%d", s.EndNano),
		}
		if s.ParentID != "" {
			span["parentSpanId"] = s.ParentID
		}
		if s.Status != nil {
			span["status"] = map[string]any{"code": s.Status.Code, "message": s.Status.Message}
		}
		otlpSpans = append(otlpSpans, span)
	}

	return map[string]any{
		"resourceSpans": []map[string]any{
			{
				"resource": map[string]any{"attributes": attrs},
				"scopeSpans": []map[string]any{
					{
						"scope": map[string]any{"name": "brevio", "version": "1.0.0"},
						"spans": otlpSpans,
					},
				},
			},
		},
	}
}

// RecordSpan adds a completed span to the export buffer.
func (tp *TracerProvider) RecordSpan(span SpanData) {
	if !tp.enabled {
		return
	}
	tp.mu.Lock()
	tp.spans = append(tp.spans, span)
	tp.mu.Unlock()
}

// Enabled returns whether trace export is configured.
func (tp *TracerProvider) Enabled() bool {
	return tp.enabled
}

// ServiceName returns the service name for trace attribution.
func (tp *TracerProvider) ServiceName() string {
	return tp.serviceName
}

// Endpoint returns the configured OTLP endpoint.
func (tp *TracerProvider) Endpoint() string {
	return tp.endpoint
}

// Shutdown gracefully flushes pending spans and stops the export loop.
func (tp *TracerProvider) Shutdown(ctx context.Context) error {
	if !tp.enabled {
		return nil
	}
	select {
	case <-tp.stopCh:
		return nil // already stopped
	default:
		close(tp.stopCh)
	}
	select {
	case <-tp.doneCh:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// ContextWithTrace injects a TraceContext into the Go context for log correlation.
func ContextWithTrace(ctx context.Context, tc TraceContext) context.Context {
	return context.WithValue(ctx, traceContextKey{}, tc)
}

// TraceFromContext extracts a TraceContext from the Go context.
func TraceFromContext(ctx context.Context) (TraceContext, bool) {
	tc, ok := ctx.Value(traceContextKey{}).(TraceContext)
	return tc, ok
}

// TraceCorrelationMiddleware injects trace IDs from W3C traceparent header
// into the request context for structured log correlation.
func TraceCorrelationMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tp := r.Header.Get("Traceparent")
		tc := TraceContext{}
		if parts := strings.Split(tp, "-"); len(parts) >= 4 {
			tc.TraceID = parts[1]
			tc.SpanID = parts[2]
		}
		if tc.TraceID == "" {
			tc.TraceID = fmt.Sprintf("%016x%016x", time.Now().UnixNano(), time.Now().UnixNano()^0x5DEECE66D)
			tc.SpanID = fmt.Sprintf("%016x", time.Now().UnixNano())
		}
		ctx := ContextWithTrace(r.Context(), tc)
		w.Header().Set("Traceparent", fmt.Sprintf("00-%s-%s-01", tc.TraceID, tc.SpanID))
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// PrometheusMetrics provides a /metrics-compatible handler that exports
// all registered metrics in Prometheus exposition format.
type PrometheusMetrics struct {
	mu       sync.RWMutex
	registry *MetricRegistry
	gauges   map[string]float64
	counters map[string]float64
	info     map[string]string
}

// NewPrometheusMetrics creates a metrics exporter bound to a MetricRegistry.
func NewPrometheusMetrics(registry *MetricRegistry) *PrometheusMetrics {
	return &PrometheusMetrics{
		registry: registry,
		gauges:   make(map[string]float64),
		counters: make(map[string]float64),
		info: map[string]string{
			"version":     "1.0.0",
			"environment": "production",
		},
	}
}

// RecordGauge records a gauge metric value.
func (pm *PrometheusMetrics) RecordGauge(name string, value float64) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.gauges[name] = value
	if pm.registry != nil {
		_ = pm.registry.Record(name, value)
	}
}

// IncrementCounter increments a counter metric.
func (pm *PrometheusMetrics) IncrementCounter(name string, delta float64) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.counters[name] += delta
}

// Handler returns an http.Handler serving Prometheus exposition format.
func (pm *PrometheusMetrics) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pm.mu.RLock()
		defer pm.mu.RUnlock()

		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")

		// Write info metric.
		fmt.Fprintf(w, "# HELP brevio_build_info Build information.\n")
		fmt.Fprintf(w, "# TYPE brevio_build_info gauge\n")
		var labels []string
		for k, v := range pm.info {
			labels = append(labels, fmt.Sprintf(`%s="%s"`, k, v))
		}
		sort.Strings(labels)
		fmt.Fprintf(w, "brevio_build_info{%s} 1\n\n", strings.Join(labels, ","))

		// Write gauges.
		var gaugeKeys []string
		for k := range pm.gauges {
			gaugeKeys = append(gaugeKeys, k)
		}
		sort.Strings(gaugeKeys)
		for _, k := range gaugeKeys {
			fmt.Fprintf(w, "# TYPE %s gauge\n", k)
			fmt.Fprintf(w, "%s %f\n", k, pm.gauges[k])
		}

		// Write counters.
		var counterKeys []string
		for k := range pm.counters {
			counterKeys = append(counterKeys, k)
		}
		sort.Strings(counterKeys)
		for _, k := range counterKeys {
			fmt.Fprintf(w, "# TYPE %s counter\n", k)
			fmt.Fprintf(w, "%s %f\n", k, pm.counters[k])
		}
	})
}

// WorkflowObservabilityHook provides observability hooks for Temporal workflows.
type WorkflowObservabilityHook struct {
	metrics *PrometheusMetrics
}

// NewWorkflowObservabilityHook creates a hook for workflow metrics.
func NewWorkflowObservabilityHook(metrics *PrometheusMetrics) *WorkflowObservabilityHook {
	return &WorkflowObservabilityHook{metrics: metrics}
}

// OnWorkflowStart records workflow start metrics.
func (h *WorkflowObservabilityHook) OnWorkflowStart(workflowType, workspaceID string) {
	h.metrics.IncrementCounter("brevio_workflow_started_total", 1)
}

// OnWorkflowComplete records workflow completion metrics.
func (h *WorkflowObservabilityHook) OnWorkflowComplete(workflowType, workspaceID string, duration time.Duration, success bool) {
	h.metrics.IncrementCounter("brevio_workflow_completed_total", 1)
	h.metrics.RecordGauge("brevio_workflow_last_duration_ms", float64(duration.Milliseconds()))
	if !success {
		h.metrics.IncrementCounter("brevio_workflow_failed_total", 1)
	}
}

// OnActivityComplete records activity completion metrics.
func (h *WorkflowObservabilityHook) OnActivityComplete(activityType, workspaceID string, duration time.Duration, success bool) {
	h.metrics.IncrementCounter("brevio_activity_completed_total", 1)
	if !success {
		h.metrics.IncrementCounter("brevio_activity_failed_total", 1)
	}
}
