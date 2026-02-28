package contracts

import (
	"fmt"
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

	assertPolicyRuleBinding(t, v91PolicyPath, "self_modification_gate_deny", "deny", "SELF_MODIFICATION_DENIED")
	assertPolicyRuleBinding(t, v91PolicyPath, "self_modification_approval_require", "require_approval", "REQUIRE_APPROVAL")
	assertPolicyRuleBinding(t, v91PolicyPath, "self_modification_audit_allow", "allow", "ALLOW_WITH_AUDIT")
	assertPolicyRuleBinding(t, v91PolicyPath, "autonomy_promotion_cap_deny", "deny", "PROMOTION_EXCEEDS_SYSTEM_CAP")
	assertPolicyRuleBinding(t, v91PolicyPath, "goal_creation_rate_limit_deny", "deny", "GOAL_RATE_LIMIT")
	assertPolicyRuleBinding(t, v91PolicyPath, "learning_lesson_cap_deny", "deny", "LESSON_CAP_REACHED")
	assertPolicyRuleBinding(t, v91PolicyPath, "code_context_export_rate_deny", "deny", "EXPORT_RATE_LIMIT")
	assertPolicyRuleBinding(t, v91PolicyPath, "daily_capture_uniqueness_skip", "skip", "DAILY_CAPTURE_UNIQUENESS")

	assertPolicyRuleBinding(t, v92PolicyPath, "context_budget_gate_deny", "deny", "CONTEXT_BUDGET_EXCEEDED")
	assertPolicyRuleBinding(t, v92PolicyPath, "rag_token_budget_gate_deny", "deny", "RAG_BUDGET_EXCEEDED")
	assertPolicyRuleBinding(t, v92PolicyPath, "session_expiry_gate_deny", "deny", "SESSION_EXPIRED")
	assertPolicyRuleBinding(t, v92PolicyPath, "temporal_constraint_violation_deny", "deny", "TEMPORAL_CONSTRAINT_VIOLATION")
	assertPolicyRuleBinding(t, v92PolicyPath, "guardrail_block_override_deny", "deny", "GUARDRAIL_BLOCK_ACTIVE")
	assertPolicyRuleBinding(t, v92PolicyPath, "tool_quarantine_gate_deny", "deny", "TOOL_QUARANTINED")
	assertPolicyRuleBinding(t, v92PolicyPath, "feature_flag_gate_deny", "deny", "FEATURE_DISABLED")
	assertPolicyRuleBinding(t, v92PolicyPath, "model_tier_cap_deny", "deny", "MODEL_TIER_EXCEEDED")
	assertPolicyRuleBinding(t, v92PolicyPath, "react_step_limit_terminate", "terminate", "MAX_STEPS_REACHED")
	assertPolicyRuleBinding(t, v92PolicyPath, "pii_encryption_gate_deny", "deny", "PII_ENCRYPTION_REQUIRED")
	assertPolicyRuleBinding(t, v92PolicyPath, "mcp_sandbox_enforcement_deny", "deny", "SANDBOX_VIOLATION")
	assertPolicyRuleBinding(t, v92PolicyPath, "dsr_sla_warning_escalate", "escalate", "DSR_SLA_AT_RISK")
	assertPolicyRuleBinding(t, v92PolicyPath, "event_schema_validation_deny", "deny", "EVENT_SCHEMA_INVALID")
	assertPolicyRuleBinding(t, v92PolicyPath, "cache_write_size_limit_deny", "deny", "CACHE_ENTRY_TOO_LARGE")
	assertPolicyRuleBinding(t, v92PolicyPath, "conflict_resolution_manual_pause", "pause", "CONFLICT_REQUIRES_MANUAL_REVIEW")
	assertPolicyRuleBinding(t, v92PolicyPath, "streaming_first_byte_sla_warn", "warn", "FIRST_BYTE_SLA_BREACH")
	assertPolicyRuleBinding(t, v92PolicyPath, "admin_action_audit_allow", "allow", "ADMIN_ACTION_AUDIT")
	assertPolicyRuleBinding(t, v92PolicyPath, "compliance_evidence_integrity_deny", "deny", "EVIDENCE_HASH_MISSING")
}

func TestCanonicalEventsAndMetricsClosure(t *testing.T) {
	t.Parallel()

	root := repositoryRoot(t)

	assertLineSetEquals(
		t,
		readLineSet(t, filepath.Join(root, "spec", "events", "canonical_events_v9.txt")),
		[]string{
			"BREVIO.ingress.received.v1",
			"BREVIO.ingress.duplicate_dropped.v1",
			"BREVIO.brain.plan.proposed.v1",
			"BREVIO.control.gate.decision.v1",
			"BREVIO.control.consent.challenge_issued.v1",
			"BREVIO.control.consent.approved.v1",
			"BREVIO.workflow.created.v1",
			"BREVIO.workflow.step.started.v1",
			"BREVIO.workflow.step.completed.v1",
			"BREVIO.workflow.step.failed.v1",
			"BREVIO.hands.tool.simulated.v1",
			"BREVIO.hands.tool.committed.v1",
			"BREVIO.trust.receipt.created.v1",
			"BREVIO.trust.evidence.attached.v1",
			"BREVIO.trust.synthesis_evidence.created.v1",
			"BREVIO.security.ssrf.blocked.v1",
			"BREVIO.security.webhook.signature_invalid.v1",
			"BREVIO.security.webhook.replay_blocked.v1",
			"BREVIO.provision.request.created.v1",
			"BREVIO.provision.server.active.v1",
			"BREVIO.provision.server.failed.v1",
			"BREVIO.provision.policy_decision.v1",
			"BREVIO.provision.step_started.v1",
			"BREVIO.provision.step_succeeded.v1",
			"BREVIO.provision.step_failed.v1",
			"BREVIO.provision.compensation_started.v1",
			"BREVIO.provision.compensation_completed.v1",
			"BREVIO.provision.server_quarantined.v1",
			"BREVIO.mcp.drift.quarantined.v1",
		},
		"v9 canonical events",
	)

	assertLineSetEquals(
		t,
		readLineSet(t, filepath.Join(root, "spec", "events", "canonical_events_v91.txt")),
		[]string{
			"BREVIO.goal.created.v1",
			"BREVIO.goal.progress_updated.v1",
			"BREVIO.goal.milestone_completed.v1",
			"BREVIO.goal.stalled.v1",
			"BREVIO.goal.completed.v1",
			"BREVIO.trust.score_computed.v1",
			"BREVIO.trust.promotion_proposed.v1",
			"BREVIO.trust.promotion_decided.v1",
			"BREVIO.trust.autonomy_upgraded.v1",
			"BREVIO.learning.feedback_received.v1",
			"BREVIO.learning.lesson_proposed.v1",
			"BREVIO.learning.lesson_confirmed.v1",
			"BREVIO.learning.lesson_applied.v1",
			"BREVIO.learning.lesson_retired.v1",
			"BREVIO.capture.daily_completed.v1",
			"BREVIO.capture.morning_briefing_sent.v1",
			"BREVIO.mission_control.refreshed.v1",
			"BREVIO.capability.recommendation_created.v1",
			"BREVIO.capability.recommendation_adopted.v1",
			"BREVIO.self_modification.denied.v1",
			"BREVIO.self_modification.executed.v1",
			"BREVIO.codebase.dependency_detected.v1",
			"BREVIO.codebase.pattern_detected.v1",
			"BREVIO.codebase.debt_identified.v1",
			"BREVIO.codebase.debt_task_created.v1",
			"BREVIO.codebase.debt_task_completed.v1",
			"BREVIO.codebase.template_created.v1",
			"BREVIO.codebase.template_instantiated.v1",
			"BREVIO.codebase.context_exported.v1",
			"BREVIO.discovery.followup_generated.v1",
			"BREVIO.discovery.followup_answered.v1",
		},
		"v9.1 canonical events",
	)

	assertLineSetEquals(
		t,
		readLineSet(t, filepath.Join(root, "spec", "events", "canonical_events_v92.txt")),
		[]string{
			"BREVIO.context.budget_allocated.v1",
			"BREVIO.context.expansion_triggered.v1",
			"BREVIO.context.overflow.v1",
			"BREVIO.rag.retrieval.completed.v1",
			"BREVIO.rag.ingestion.completed.v1",
			"BREVIO.rag.eval.completed.v1",
			"BREVIO.session.created.v1",
			"BREVIO.session.expired.v1",
			"BREVIO.session.intent_continued.v1",
			"BREVIO.session.intent_corrected.v1",
			"BREVIO.session.entity_resolved.v1",
			"BREVIO.temporal.resolved.v1",
			"BREVIO.temporal.conflict_detected.v1",
			"BREVIO.guardrail.triggered.v1",
			"BREVIO.guardrail.blocked.v1",
			"BREVIO.tool_health.score_computed.v1",
			"BREVIO.tool_health.quarantined.v1",
			"BREVIO.tool_health.recovered.v1",
			"BREVIO.tool_health.degraded.v1",
			"BREVIO.feature_flag.created.v1",
			"BREVIO.feature_flag.toggled.v1",
			"BREVIO.memory.conflict_detected.v1",
			"BREVIO.memory.conflict_resolved.v1",
			"BREVIO.streaming.ack_sent.v1",
			"BREVIO.streaming.first_byte.v1",
			"BREVIO.error.template_served.v1",
			"BREVIO.cache.invalidated.v1",
			"BREVIO.model_tier.override.v1",
			"BREVIO.react.early_exit.v1",
			"BREVIO.compliance.evidence_collected.v1",
			"BREVIO.compliance.dsr_received.v1",
			"BREVIO.compliance.dsr_completed.v1",
			"BREVIO.event_schema.registered.v1",
			"BREVIO.event_schema.deprecated.v1",
			"BREVIO.admin.alert_fired.v1",
			"BREVIO.pii.encrypted.v1",
			"BREVIO.mcp.sandbox_violation.v1",
		},
		"v9.2 canonical events",
	)

	assertLineSetEquals(
		t,
		readLineSet(t, filepath.Join(root, "spec", "metrics", "canonical_metrics_v92.txt")),
		[]string{
			"BREVIO_context_budget_utilization_pct",
			"BREVIO_context_expansion_triggered_total",
			"BREVIO_context_overflow_total",
			"BREVIO_rag_retrieval_latency_ms",
			"BREVIO_rag_rerank_latency_ms",
			"BREVIO_rag_faithfulness_score",
			"BREVIO_rag_relevance_score",
			"BREVIO_rag_chunks_total",
			"BREVIO_rag_ingestion_total",
			"BREVIO_session_active_gauge",
			"BREVIO_session_duration_ms",
			"BREVIO_session_turns_per_session",
			"BREVIO_session_coreference_resolution_total",
			"BREVIO_temporal_resolution_total",
			"BREVIO_temporal_conflict_detected_total",
			"BREVIO_guardrail_trigger_total",
			"BREVIO_guardrail_latency_ms",
			"BREVIO_guardrail_block_total",
			"BREVIO_tool_health_score",
			"BREVIO_tool_quarantine_total",
			"BREVIO_tool_recovery_total",
			"BREVIO_feature_flag_evaluation_total",
			"BREVIO_memory_conflict_total",
			"BREVIO_streaming_first_byte_ms",
			"BREVIO_streaming_ack_sent_total",
			"BREVIO_error_template_served_total",
			"BREVIO_cache_hit_rate",
			"BREVIO_cache_latency_ms",
			"BREVIO_model_tier_usage_total",
			"BREVIO_model_tier_override_total",
			"BREVIO_react_early_exit_total",
			"BREVIO_react_steps_per_run",
			"BREVIO_compliance_evidence_collected_total",
			"BREVIO_dsr_processing_latency_ms",
			"BREVIO_dsr_sla_breach_total",
			"BREVIO_event_schema_validation_total",
			"BREVIO_admin_api_call_total",
			"BREVIO_pii_encryption_total",
			"BREVIO_mcp_sandbox_violation_total",
		},
		"v9.2 canonical metrics",
	)
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

func assertPolicyRuleBinding(t *testing.T, path, rule, result, reason string) {
	t.Helper()

	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file %s: %v", path, err)
	}
	expected := fmt.Sprintf(`%s := {"result": "%s", "reason": "%s"} if`, rule, result, reason)
	if !strings.Contains(string(body), expected) {
		t.Fatalf("missing exact policy rule binding in %s: %s", path, expected)
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

func assertLineSetEquals(t *testing.T, actual []string, expected []string, label string) {
	t.Helper()

	sort.Strings(actual)
	sort.Strings(expected)

	if len(actual) != len(expected) {
		t.Fatalf("%s count mismatch: got=%d want=%d", label, len(actual), len(expected))
	}

	for idx := range expected {
		if actual[idx] != expected[idx] {
			t.Fatalf("%s mismatch at index %d: got=%q want=%q", label, idx, actual[idx], expected[idx])
		}
	}
}
