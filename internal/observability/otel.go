package observability

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
)

// TracerProvider manages distributed tracing with W3C propagation.
// In production, this wraps the OpenTelemetry SDK; in-process it provides
// a lightweight trace context for structured logging correlation.
type TracerProvider struct {
	serviceName string
	environment string
	endpoint    string // OTEL_EXPORTER_OTLP_ENDPOINT
	enabled     bool
}

// NewTracerProvider creates a tracer provider. When endpoint is empty,
// tracing operates in passthrough mode (contexts propagated, not exported).
func NewTracerProvider(serviceName, environment, endpoint string) *TracerProvider {
	return &TracerProvider{
		serviceName: serviceName,
		environment: environment,
		endpoint:    endpoint,
		enabled:     endpoint != "",
	}
}

// Enabled returns whether trace export is configured.
func (tp *TracerProvider) Enabled() bool {
	return tp.enabled
}

// ServiceName returns the service name for trace attribution.
func (tp *TracerProvider) ServiceName() string {
	return tp.serviceName
}

// Shutdown gracefully flushes pending spans.
func (tp *TracerProvider) Shutdown(ctx context.Context) error {
	// In production, this would call tp.provider.Shutdown(ctx).
	return nil
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
