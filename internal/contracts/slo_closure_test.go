package contracts

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSLOCatalogClosure(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)

	assertSLOFileEquals(t, filepath.Join(root, "spec", "slos", "v9_slos.txt"), []string{
		"P95 interactive turn latency T1: <= 2.5s",
		"P95 interactive turn latency T2: <= 6.0s",
		"P95 interactive turn latency T3: <= 20.0s",
		"Gateway availability: >= 99.95%",
		"Control availability: >= 99.9%",
		"Brain availability: >= 99.9%",
		"Provisioning success rate: >= 99.0% reach active within 10min (excl. user approval wait)",
		"Critical drift quarantine response: >= 99% quarantined within 2min",
		"Operator review SLA: 95% decided within 24h; auto-expire at 72h",
		"Error rate (load test): <= 0.5%",
	})

	assertSLOFileEquals(t, filepath.Join(root, "spec", "slos", "v92_slos.txt"), []string{
		"RAG retrieval P95: <= 300ms (dense), <= 500ms (hybrid)",
		"RAG faithfulness score (eval): >= 0.80",
		"RAG relevance score (eval): >= 0.75",
		"Context budget allocation P95: <= 50ms",
		"Session resolution P95: <= 100ms",
		"Temporal expression resolution P95: <= 50ms",
		"Guardrail evaluation P95: <= 200ms",
		"Guardrail false positive rate: <= 2%",
		"Tool health score computation P95: <= 100ms",
		"Feature flag evaluation P95: <= 5ms",
		"Streaming first byte: <= 500ms",
		"CRDT conflict resolution P95: <= 200ms",
		"Admin API P95: <= 500ms",
		"DSR SLA compliance: >= 99% before deadline",
		"Event schema validation P95: <= 10ms",
		"Cache hit rate (prompt_embedding): >= 60%",
		"Cache hit rate (tool_result): >= 40%",
		"Cache hit rate (compiled_context): >= 30%",
	})
}

func assertSLOFileEquals(t *testing.T, path string, expected []string) {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read slo file %s: %v", path, err)
	}

	lines := strings.Split(string(body), "\n")
	actual := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		actual = append(actual, trimmed)
	}
	assertStringSetEquals(t, actual, expected, "slo_catalog:"+path)
}
