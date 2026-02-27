package contracts

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

func TestPolicyRuleClosure(t *testing.T) {
	t.Parallel()

	root := repositoryRoot(t)

	v91PolicyPath := filepath.Join(root, "policies", "v91_addendum.rego")
	v92PolicyPath := filepath.Join(root, "policies", "v92_addendum.rego")

	assertFileContainsAll(t, v91PolicyPath, []string{
		"self_modification_gate_deny",
		"self_modification_approval_require",
		"self_modification_audit_allow",
		"autonomy_promotion_cap_deny",
		"goal_creation_rate_limit_deny",
		"learning_lesson_cap_deny",
		"code_context_export_rate_deny",
		"daily_capture_uniqueness_skip",
	})

	assertFileContainsAll(t, v92PolicyPath, []string{
		"context_budget_gate_deny",
		"rag_token_budget_gate_deny",
		"session_expiry_gate_deny",
		"temporal_constraint_violation_deny",
		"guardrail_block_override_deny",
		"tool_quarantine_gate_deny",
		"feature_flag_gate_deny",
		"model_tier_cap_deny",
		"react_step_limit_terminate",
		"pii_encryption_gate_deny",
		"mcp_sandbox_enforcement_deny",
		"dsr_sla_warning_escalate",
		"event_schema_validation_deny",
		"cache_write_size_limit_deny",
		"conflict_resolution_manual_pause",
		"streaming_first_byte_sla_warn",
		"admin_action_audit_allow",
		"compliance_evidence_integrity_deny",
	})
}

func TestCanonicalEventsAndMetricsClosure(t *testing.T) {
	t.Parallel()

	root := repositoryRoot(t)

	v9Events := readLineSet(t, filepath.Join(root, "spec", "events", "canonical_events_v9.txt"))
	if len(v9Events) != 29 {
		t.Fatalf("v9 canonical event count mismatch: got=%d want=29", len(v9Events))
	}
	if !containsLine(v9Events, "BREVIO.mcp.drift.quarantined.v1") {
		t.Fatal("v9 canonical events missing BREVIO.mcp.drift.quarantined.v1")
	}

	v91Events := readLineSet(t, filepath.Join(root, "spec", "events", "canonical_events_v91.txt"))
	if len(v91Events) != 31 {
		t.Fatalf("v9.1 canonical event count mismatch: got=%d want=31", len(v91Events))
	}
	if !containsLine(v91Events, "BREVIO.discovery.followup_answered.v1") {
		t.Fatal("v9.1 canonical events missing BREVIO.discovery.followup_answered.v1")
	}

	v92Events := readLineSet(t, filepath.Join(root, "spec", "events", "canonical_events_v92.txt"))
	if len(v92Events) != 37 {
		t.Fatalf("v9.2 canonical event count mismatch: got=%d want=37", len(v92Events))
	}
	if !containsLine(v92Events, "BREVIO.mcp.sandbox_violation.v1") {
		t.Fatal("v9.2 canonical events missing BREVIO.mcp.sandbox_violation.v1")
	}

	v92Metrics := readLineSet(t, filepath.Join(root, "spec", "metrics", "canonical_metrics_v92.txt"))
	if len(v92Metrics) != 39 {
		t.Fatalf("v9.2 metric count mismatch: got=%d want=39", len(v92Metrics))
	}
	if !containsLine(v92Metrics, "BREVIO_streaming_first_byte_ms") {
		t.Fatal("v9.2 metrics missing BREVIO_streaming_first_byte_ms")
	}
}

func assertFileContainsAll(t *testing.T, path string, required []string) {
	t.Helper()

	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file %s: %v", path, err)
	}
	content := string(body)
	for _, token := range required {
		if !strings.Contains(content, token) {
			t.Fatalf("missing token %q in %s", token, path)
		}
	}
}

func readLineSet(t *testing.T, path string) []string {
	t.Helper()

	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}

	seen := map[string]struct{}{}
	for _, line := range strings.Split(string(body), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		seen[trimmed] = struct{}{}
	}

	out := make([]string, 0, len(seen))
	for item := range seen {
		out = append(out, item)
	}
	sort.Strings(out)
	return out
}

func containsLine(items []string, needle string) bool {
	for _, item := range items {
		if item == needle {
			return true
		}
	}
	return false
}
