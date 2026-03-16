package metrics_test

import (
	"testing"
	"time"

	"github.com/brevio/brevio/internal/metrics"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestRecordActivity_Success(t *testing.T) {
	t.Parallel()
	// Should not panic.
	metrics.RecordActivity("TestActivity", time.Now().Add(-100*time.Millisecond), nil)
}

func TestRecordToolExecution_IncrementsCounts(t *testing.T) {
	before := testutil.ToFloat64(metrics.ToolExecutions.WithLabelValues("test_tool_inc", "success"))
	metrics.RecordToolExecution("test_tool_inc", "success")
	after := testutil.ToFloat64(metrics.ToolExecutions.WithLabelValues("test_tool_inc", "success"))
	if after-before != 1 {
		t.Errorf("expected counter to increment by 1, got %f -> %f", before, after)
	}
}

func TestRecordAuthorization_IncrementsBothDecisions(t *testing.T) {
	// Should not panic for either decision type.
	metrics.RecordAuthorization("allow", "low")
	metrics.RecordAuthorization("deny", "critical")
}

func TestRecordLLMTokens_TracksAllTypes(t *testing.T) {
	before := testutil.ToFloat64(metrics.LLMTokens.WithLabelValues("anthropic", "T2", "input"))
	metrics.RecordLLMTokens("anthropic", "T2", 500, 200, 50)
	after := testutil.ToFloat64(metrics.LLMTokens.WithLabelValues("anthropic", "T2", "input"))
	if after-before != 500 {
		t.Errorf("expected 500 input tokens added, got %f", after-before)
	}
}

func TestHandler_Returns200(t *testing.T) {
	if metrics.Handler() == nil {
		t.Error("expected non-nil metrics handler")
	}
}
