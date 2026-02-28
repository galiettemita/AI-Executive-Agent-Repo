package observability

import (
	"encoding/json"
	"path/filepath"
	"testing"
)

func TestStructuredLogEntryRequiredFields(t *testing.T) {
	t.Parallel()

	entry := NewLogEntry("gateway", "staging", "ws_1", "user_1", "turn_1", "trace_1", "span_1", "BREVIO.ingress.received.v1", "info", map[string]any{"channel": "whatsapp"})
	if err := entry.Validate(); err != nil {
		t.Fatalf("expected valid structured log entry, got %v", err)
	}
	blob, err := entry.JSON()
	if err != nil {
		t.Fatalf("marshal log json: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(blob, &decoded); err != nil {
		t.Fatalf("unmarshal log json: %v", err)
	}
	for _, key := range []string{"ts", "service", "env", "workspace_id", "user_id", "ingress_turn_id", "trace_id", "span_id", "event", "severity", "attrs"} {
		if _, ok := decoded[key]; !ok {
			t.Fatalf("missing required log key %s in %v", key, decoded)
		}
	}
}

func TestMetricRegistryCanonicalValidation(t *testing.T) {
	t.Parallel()

	metrics, err := LoadCanonicalMetricNames(filepath.Join("..", "..", "spec", "metrics", "canonical_metrics_v92.txt"))
	if err != nil {
		t.Fatalf("load canonical metrics: %v", err)
	}
	if len(metrics) == 0 {
		t.Fatal("expected canonical metrics list to be non-empty")
	}

	registry := NewMetricRegistry(metrics)
	if err := registry.Record(metrics[0], 1.5); err != nil {
		t.Fatalf("record canonical metric: %v", err)
	}
	if _, ok := registry.Value(metrics[0]); !ok {
		t.Fatalf("expected metric value for %s", metrics[0])
	}
	if err := registry.Record("BREVIO_unknown_metric", 1); err == nil {
		t.Fatal("expected unknown metric registration error")
	}
}
