package metrics

import (
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// ActivityDuration tracks duration of every Temporal activity execution.
var ActivityDuration = promauto.NewHistogramVec(
	prometheus.HistogramOpts{
		Name:    "brevio_activity_duration_seconds",
		Help:    "Duration of Temporal activity executions in seconds.",
		Buckets: []float64{0.05, 0.1, 0.25, 0.5, 1, 2, 5, 10, 30, 60},
	},
	[]string{"activity", "status"},
)

// WorkflowDuration tracks end-to-end workflow duration from ingest to dispatch.
var WorkflowDuration = promauto.NewHistogramVec(
	prometheus.HistogramOpts{
		Name:    "brevio_workflow_duration_seconds",
		Help:    "End-to-end workflow duration in seconds.",
		Buckets: []float64{1, 3, 5, 8, 10, 15, 30, 60, 120},
	},
	[]string{"workflow"},
)

// ToolExecutions counts total tool execution attempts and outcomes.
var ToolExecutions = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name: "brevio_tool_executions_total",
		Help: "Total number of tool executions.",
	},
	[]string{"tool_key", "status"},
)

// PlanAuthorizations counts authorization decisions per risk level.
var PlanAuthorizations = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name: "brevio_plan_authorizations_total",
		Help: "Total plan authorization decisions by outcome.",
	},
	[]string{"decision", "risk_level"},
)

// LLMTokens tracks cumulative token consumption by provider and tier.
var LLMTokens = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name: "brevio_llm_tokens_total",
		Help: "Total LLM tokens consumed.",
	},
	[]string{"provider", "tier", "token_type"},
)

// InboundMessages counts messages received per channel.
var InboundMessages = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name: "brevio_inbound_messages_total",
		Help: "Total inbound messages processed by channel.",
	},
	[]string{"channel", "status"},
)

// RecordActivity records activity duration into the histogram.
func RecordActivity(activityName string, start time.Time, err error) {
	s := "success"
	if err != nil {
		s = "error"
	}
	ActivityDuration.WithLabelValues(activityName, s).Observe(time.Since(start).Seconds())
}

// RecordToolExecution increments the tool execution counter.
func RecordToolExecution(toolKey, status string) {
	ToolExecutions.WithLabelValues(toolKey, status).Inc()
}

// RecordAuthorization increments the authorization counter.
func RecordAuthorization(decision, riskLevel string) {
	PlanAuthorizations.WithLabelValues(decision, riskLevel).Inc()
}

// RecordLLMTokens increments token consumption counters.
func RecordLLMTokens(provider, tier string, inputTokens, outputTokens, cacheReadTokens int) {
	LLMTokens.WithLabelValues(provider, tier, "input").Add(float64(inputTokens))
	LLMTokens.WithLabelValues(provider, tier, "output").Add(float64(outputTokens))
	if cacheReadTokens > 0 {
		LLMTokens.WithLabelValues(provider, tier, "cache_read").Add(float64(cacheReadTokens))
	}
}

// RecordInboundMessage increments the inbound message counter.
func RecordInboundMessage(channel, status string) {
	InboundMessages.WithLabelValues(channel, status).Inc()
}

// Handler returns the Prometheus metrics HTTP handler for /metrics.
func Handler() http.Handler {
	return promhttp.Handler()
}
