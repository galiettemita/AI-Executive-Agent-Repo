package database

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"testing"
)

var (
	createTypeEnumPattern   = regexp.MustCompile(`(?mi)^CREATE TYPE\s+([a-z0-9_]+)\s+AS ENUM`)
	createTableNamePattern  = regexp.MustCompile(`(?mi)^CREATE TABLE\s+([a-z0-9_]+)\s*\(`)
	createTableBlockPattern = regexp.MustCompile(`(?ms)^CREATE TABLE\s+([a-z0-9_]+)\s*\((.*?)\);\s*`)
)

func TestMigration001V9CoreClosure(t *testing.T) {
	t.Parallel()

	sql := readMigrationSQL(t, "001_BREVIO_v9_init.sql")

	if !strings.Contains(sql, "CREATE EXTENSION IF NOT EXISTS pgcrypto;") {
		t.Fatal("migration 001 missing pgcrypto extension")
	}
	if !strings.Contains(sql, "CREATE EXTENSION IF NOT EXISTS vector;") {
		t.Fatal("migration 001 missing vector extension")
	}
	if !strings.Contains(sql, "CREATE OR REPLACE FUNCTION uuid_v7_generate()") {
		t.Fatal("migration 001 missing uuid_v7_generate function")
	}

	gotEnums := parseIdentifiers(sql, createTypeEnumPattern)
	if len(gotEnums) != 27 {
		t.Fatalf("migration 001 enum count mismatch: got=%d want=27 (%v)", len(gotEnums), gotEnums)
	}

	gotTables := parseIdentifiers(sql, createTableNamePattern)
	expectedTables := []string{
		"accounts", "airport_knowledge", "approvals", "audit_log_entries", "auto_commit_proofs",
		"canvas_interactions", "canvas_sessions", "capability_aliases", "capability_defs", "capability_resolution_cache",
		"channel_bindings", "channel_identity_envelopes", "code_repo_profiles", "code_repositories", "connector_health",
		"connector_success_stats", "connector_tools", "connectors", "consents", "content_firewall_logs",
		"delegation_grants", "discovery_answers", "discovery_questions", "discovery_sessions", "discovery_unparsed_lines",
		"document_parse_results", "environment_signals", "execution_gate_decisions", "financial_anomaly_events", "financial_merchant_rules",
		"ha_entity_cache", "incident_notifications", "ingress_turns", "key_versions", "llm_output_replay",
		"mcp_egress_audit_events", "mcp_tool_schema_snapshots", "memory_exclusion_rules", "memory_items", "memory_revisions",
		"memory_write_requests", "model_catalog", "outbox_items", "pairing_invitations", "portability_requests",
		"prompt_versions", "provisioning_declined", "provisioning_events", "provisioning_monthly_budgets", "provisioning_monthly_usage",
		"provisioning_policy_versions", "provisioning_ranker_versions", "provisioning_requests", "provisioning_steps", "rate_limit_events",
		"review_tasks", "role_bindings", "roles", "routing_policies", "runtime_profiles",
		"semantic_verifier_failures", "server_artifacts", "server_capability_bindings", "server_catalog", "specialist_agents",
		"synthesis_evidence_items", "synthesis_evidence_receipts", "system_autonomy_overrides", "tool_capability_bindings", "tool_executions",
		"tool_inventory", "trajectories", "trajectory_tool_calls", "transcription_logs", "trust_receipt_evidence",
		"trust_receipts", "user_channels", "user_connector_settings", "user_domain_autonomy_settings", "user_entity_fingerprints",
		"user_oauth_tokens", "users", "voice_profiles", "workflow_instances", "workflow_steps",
		"workspace_behavior_policies", "workspace_mcp_servers", "workspace_personas", "workspace_profiles", "workspace_server_rules",
		"workspaces",
	}
	assertExactNameSet(t, "migration 001 tables", gotTables, expectedTables)
	assertWorkspaceRLSCoverage(t, sql, "migration 001")
}

func TestMigration002V91SoftIntelligenceClosure(t *testing.T) {
	t.Parallel()

	sql := readMigrationSQL(t, "002_BREVIO_v91_soft_intelligence.sql")

	expectedEnums := []string{
		"capture_trigger", "context_export_format", "debt_category", "debt_severity", "debt_status",
		"debt_task_status", "dependency_type", "exploration_status", "feedback_disposition", "feedback_type",
		"followup_trigger", "goal_horizon", "goal_priority", "goal_status", "lesson_status",
		"pattern_scope", "promotion_status", "self_mod_action", "template_status", "trust_event_type",
		"widget_type",
	}
	gotEnums := parseIdentifiers(sql, createTypeEnumPattern)
	assertExactNameSet(t, "migration 002 enums", gotEnums, expectedEnums)

	expectedTables := []string{
		"autonomy_promotions", "autonomy_trust_scores", "capability_recommendations", "code_context_exports", "cross_repo_patterns",
		"daily_captures", "debt_resolution_tasks", "discovery_adaptive_questions", "discovery_followup_rules", "exploration_history",
		"goal_items", "goal_milestones", "goal_progress_logs", "interaction_feedback", "introspection_templates",
		"learned_lessons", "learning_config", "mission_control_config", "mission_control_widgets", "project_templates",
		"repo_dependencies", "self_modification_policies", "technical_debt_items",
	}
	gotTables := parseIdentifiers(sql, createTableNamePattern)
	assertExactNameSet(t, "migration 002 tables", gotTables, expectedTables)
	assertWorkspaceRLSCoverage(t, sql, "migration 002")
}

func TestMigration003V92ProductionHardeningClosure(t *testing.T) {
	t.Parallel()

	sql := readMigrationSQL(t, "003_BREVIO_v92_production_hardening.sql")

	expectedEnums := []string{
		"cache_policy_status", "cache_scope", "compatibility_level", "compliance_evidence_status", "compliance_framework",
		"context_budget_status", "context_item_type", "crdt_conflict_status", "dsr_request_status", "error_category",
		"error_severity", "event_schema_status", "feature_flag_match_type", "feature_flag_status", "feature_flag_type",
		"guardrail_action", "guardrail_severity", "model_override_status", "model_tier", "pii_policy_status",
		"quarantine_status", "rag_chunk_status", "rag_eval_status", "rag_retrieval_mode", "react_exit_reason",
		"sandbox_profile_status", "session_intent_status", "session_status_v92", "streaming_ack_status", "streaming_mode",
		"temporal_constraint_priority", "temporal_resolution_status", "tool_health_status", "travel_mode",
	}
	gotEnums := parseIdentifiers(sql, createTypeEnumPattern)
	assertExactNameSet(t, "migration 003 enums", gotEnums, expectedEnums)

	expectedTables := []string{
		"admin_alert_channels", "admin_alert_rules", "admin_dashboard_config", "admin_kpi_reports", "admin_saved_views",
		"cache_audit_log", "cache_policies", "compliance_dsr_requests", "compliance_evidence", "compliance_frameworks",
		"constrained_decoding_config", "context_budget_allocations", "context_budget_audit", "context_budgets", "conversation_sessions",
		"error_taxonomy", "error_templates", "event_schema_registry", "event_schema_versions", "feature_flag_evaluations",
		"feature_flag_rules", "feature_flags", "guardrails_config", "guardrails_events", "guardrails_rule_sets",
		"latency_budgets", "mcp_sandbox_profiles", "memory_conflict_log", "memory_vector_clocks", "model_tier_overrides",
		"model_tier_policies", "pii_encryption_policies", "rag_chunks", "rag_collections", "rag_eval_scores",
		"rag_reranker_config", "rag_retrievals", "react_execution_policies", "session_entities", "session_intents",
		"streaming_config", "temporal_constraints", "temporal_reasoning_config", "tool_health_events", "tool_health_scores",
		"tool_quarantine_rules", "travel_time_cache",
	}
	gotTables := parseIdentifiers(sql, createTableNamePattern)
	assertExactNameSet(t, "migration 003 tables", gotTables, expectedTables)

	assertWorkspaceRLSCoverage(t, sql, "migration 003")

	if !strings.Contains(sql, "CREATE INDEX idx_rag_chunks_bm25_tokens_gin ON rag_chunks USING gin (bm25_tokens);") {
		t.Fatal("migration 003 missing GIN index on rag_chunks.bm25_tokens")
	}
	if !strings.Contains(sql, "CREATE INDEX idx_rag_chunks_embedding_hnsw ON rag_chunks USING hnsw (embedding vector_cosine_ops);") {
		t.Fatal("migration 003 missing HNSW index on rag_chunks.embedding")
	}
}

func readMigrationSQL(t *testing.T, fileName string) string {
	t.Helper()

	path := migrationPath(fileName)
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", fileName, err)
	}
	return string(body)
}

func migrationPath(fileName string) string {
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		panic("unable to resolve caller for migration path")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(currentFile), "..", "..", "db", "migrations", fileName))
}

func parseIdentifiers(content string, pattern *regexp.Regexp) []string {
	matches := pattern.FindAllStringSubmatch(content, -1)
	seen := make(map[string]struct{}, len(matches))
	names := make([]string, 0, len(matches))
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		name := strings.ToLower(match[1])
		if _, exists := seen[name]; exists {
			continue
		}
		seen[name] = struct{}{}
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func parseWorkspaceScopedTables(content string) []string {
	matches := createTableBlockPattern.FindAllStringSubmatch(content, -1)
	workspaceTables := make([]string, 0, len(matches))
	for _, match := range matches {
		if len(match) < 3 {
			continue
		}
		tableName := strings.ToLower(match[1])
		tableBody := strings.ToLower(match[2])
		if strings.Contains(tableBody, "workspace_id uuid") {
			workspaceTables = append(workspaceTables, tableName)
		}
	}
	sort.Strings(workspaceTables)
	return workspaceTables
}

func assertWorkspaceRLSCoverage(t *testing.T, sql string, label string) {
	t.Helper()

	scopedTables := parseWorkspaceScopedTables(sql)
	missing := make([]string, 0)
	for _, table := range scopedTables {
		if !strings.Contains(sql, fmt.Sprintf("'%s'", table)) {
			missing = append(missing, table)
		}
	}
	if len(missing) != 0 {
		t.Fatalf("%s missing workspace tables in RLS policy list: %v", label, missing)
	}
}

func assertExactNameSet(t *testing.T, label string, got []string, expected []string) {
	t.Helper()

	gotSet := toSet(got)
	expectedSet := toSet(expected)

	missing := setDifference(expectedSet, gotSet)
	extra := setDifference(gotSet, expectedSet)

	if len(missing) == 0 && len(extra) == 0 {
		return
	}

	t.Fatalf(
		"%s mismatch: got=%d expected=%d missing=%v extra=%v",
		label,
		len(got),
		len(expected),
		missing,
		extra,
	)
}

func toSet(items []string) map[string]struct{} {
	set := make(map[string]struct{}, len(items))
	for _, item := range items {
		set[item] = struct{}{}
	}
	return set
}

func setDifference(left map[string]struct{}, right map[string]struct{}) []string {
	diff := make([]string, 0)
	for key := range left {
		if _, ok := right[key]; !ok {
			diff = append(diff, key)
		}
	}
	sort.Strings(diff)
	return diff
}
