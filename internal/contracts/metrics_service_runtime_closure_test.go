package contracts

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMetricsServiceRuntimeClosure(t *testing.T) {
	t.Parallel()

	root := repositoryRoot(t)
	metricsSource := filepath.Join(root, "deprecated", "brevio-metrics", "src", "index.ts")
	metricsReadme := filepath.Join(root, "deprecated", "brevio-metrics", "README.md")

	assertFileContainsTokens(t, metricsSource, []string{
		"brevio_messages_total",
		"brevio_message_latency_ms",
		"brevio_skill_executions_total",
		"brevio_skill_latency_ms",
		"brevio_llm_tokens_total",
		"brevio_llm_cost_cents",
		"brevio_circuit_breaker_state",
		"brevio_active_sessions",
		"brevio_auth_token_refreshes",
		"brevio_budget_utilization_pct",
		"/metrics",
		"metrics.event.ingested",
		"createMetricsRuntime",
	})

	assertFileContainsTokens(t, metricsReadme, []string{
		"Metrics aggregation service",
		"GET /metrics",
		"POST /api/v1/metrics/events",
		"GET /api/v1/metrics/snapshot",
		"Section 10",
	})

	body, err := os.ReadFile(metricsReadme)
	if err != nil {
		t.Fatalf("read metrics readme: %v", err)
	}
	if strings.Contains(strings.ToLower(string(body)), "scaffold directory") {
		t.Fatalf("metrics README still contains scaffold marker")
	}
}
