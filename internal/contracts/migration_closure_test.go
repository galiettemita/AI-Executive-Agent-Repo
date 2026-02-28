package contracts

import (
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

func TestMigrationSchemaStrictClosure(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)

	v9Path := filepath.Join(root, "db", "migrations", "001_BREVIO_v9_init.sql")
	v91Path := filepath.Join(root, "db", "migrations", "002_BREVIO_v91_soft_intelligence.sql")
	v92Path := filepath.Join(root, "db", "migrations", "003_BREVIO_v92_production_hardening.sql")

	assertMigrationClosure(t, migrationExpectation{
		path:           v9Path,
		expectedEnums:  v9Enums,
		expectedTables: v9Tables,
	})
	assertMigrationClosure(t, migrationExpectation{
		path:           v91Path,
		expectedEnums:  v91Enums,
		expectedTables: v91Tables,
	})
	assertMigrationClosure(t, migrationExpectation{
		path:           v92Path,
		expectedEnums:  v92Enums,
		expectedTables: v92Tables,
	})
}

type migrationExpectation struct {
	path           string
	expectedEnums  []string
	expectedTables []string
}

func assertMigrationClosure(t *testing.T, exp migrationExpectation) {
	t.Helper()
	body := readFileString(t, exp.path)
	enumNames := extractCreateNames(t, body, `(?mi)^CREATE TYPE\s+([a-z0-9_]+)\s+AS ENUM`)
	tableNames := extractCreateNames(t, body, `(?mi)^CREATE TABLE\s+([a-z0-9_]+)\s*\(`)

	assertStringSliceSetEquals(t, enumNames, exp.expectedEnums, filepath.Base(exp.path)+"_enum_set")
	assertStringSliceSetEquals(t, tableNames, exp.expectedTables, filepath.Base(exp.path)+"_table_set")
	assertEnumNaming(t, enumNames, exp.path)
	assertMigrationOrdering(t, body, exp.path)
	assertWorkspaceTableRLSCoverage(t, body, exp.path)
}

func assertEnumNaming(t *testing.T, enumNames []string, path string) {
	t.Helper()
	re := regexp.MustCompile(`^[a-z0-9_]+$`)
	for _, enumName := range enumNames {
		if !re.MatchString(enumName) {
			t.Fatalf("enum %q in %s is not snake_case", enumName, path)
		}
	}
}

func assertMigrationOrdering(t *testing.T, body string, path string) {
	t.Helper()
	firstCreateType := firstMatchIndex(body, `(?mi)^CREATE TYPE\s+`)
	lastCreateType := lastMatchIndex(body, `(?mi)^CREATE TYPE\s+`)
	firstCreateTable := firstMatchIndex(body, `(?mi)^CREATE TABLE\s+`)
	lastCreateTable := lastMatchIndex(body, `(?mi)^CREATE TABLE\s+`)
	firstRLS := firstMatchIndex(body, `(?mi)ENABLE ROW LEVEL SECURITY`)
	firstCreateIndex := firstMatchIndex(body, `(?mi)^CREATE (UNIQUE\s+)?INDEX\s+`)

	if firstCreateType < 0 || firstCreateTable < 0 || firstRLS < 0 || firstCreateIndex < 0 {
		t.Fatalf("migration %s missing required ordered sections (type/table/rls/index)", path)
	}
	if lastCreateType > firstCreateTable {
		t.Fatalf("migration %s violates order: CREATE TYPE appears after CREATE TABLE", path)
	}
	if firstRLS < lastCreateTable {
		t.Fatalf("migration %s violates order: RLS block appears before all CREATE TABLE statements complete", path)
	}
	if firstCreateIndex < firstRLS {
		t.Fatalf("migration %s violates order: CREATE INDEX appears before RLS block", path)
	}
	if firstMatchIndex(body[firstRLS:], `(?mi)^CREATE TABLE\s+`) >= 0 {
		t.Fatalf("migration %s violates order: CREATE TABLE appears after RLS block", path)
	}
	if firstMatchIndex(body[firstCreateTable:], `(?mi)^CREATE TYPE\s+`) >= 0 {
		t.Fatalf("migration %s violates order: CREATE TYPE appears after CREATE TABLE block begins", path)
	}
}

func assertWorkspaceTableRLSCoverage(t *testing.T, body string, path string) {
	t.Helper()
	if !strings.Contains(body, "current_setting(''app.workspace_id'')::uuid") {
		t.Fatalf("migration %s missing app.workspace_id RLS policy expression", path)
	}

	workspaceScopedTables := extractWorkspaceScopedTableNames(t, body)
	declaredRlsTables := extractRlsWorkspaceTableList(t, body, path)
	assertStringSliceSetEquals(
		t,
		declaredRlsTables,
		workspaceScopedTables,
		filepath.Base(path)+"_workspace_rls_table_set",
	)
}

func extractCreateNames(t *testing.T, body, pattern string) []string {
	t.Helper()
	re := regexp.MustCompile(pattern)
	matches := re.FindAllStringSubmatch(body, -1)
	out := make([]string, 0, len(matches))
	for _, match := range matches {
		if len(match) < 2 {
			t.Fatalf("invalid regex capture for pattern %q", pattern)
		}
		out = append(out, strings.ToLower(strings.TrimSpace(match[1])))
	}
	sort.Strings(out)
	return out
}

func extractWorkspaceScopedTableNames(t *testing.T, body string) []string {
	t.Helper()
	re := regexp.MustCompile(`(?mis)^CREATE TABLE\s+([a-z0-9_]+)\s*\((.*?)\);`)
	matches := re.FindAllStringSubmatch(body, -1)
	out := make([]string, 0)
	for _, match := range matches {
		if len(match) < 3 {
			t.Fatalf("invalid CREATE TABLE block capture")
		}
		tableName := strings.ToLower(strings.TrimSpace(match[1]))
		block := strings.ToLower(match[2])
		if strings.Contains(block, "workspace_id uuid") {
			out = append(out, tableName)
		}
	}
	sort.Strings(out)
	return out
}

func extractRlsWorkspaceTableList(t *testing.T, body, path string) []string {
	t.Helper()
	arrayRe := regexp.MustCompile(`(?mis)workspace_tables\s+text\[\]\s*:=\s*ARRAY\[(.*?)\];`)
	arrayMatch := arrayRe.FindStringSubmatch(body)
	if len(arrayMatch) < 2 {
		t.Fatalf("migration %s missing workspace_tables ARRAY declaration", path)
	}
	itemRe := regexp.MustCompile(`'([a-z0-9_]+)'`)
	itemMatches := itemRe.FindAllStringSubmatch(arrayMatch[1], -1)
	if len(itemMatches) == 0 {
		t.Fatalf("migration %s workspace_tables ARRAY has no entries", path)
	}

	seen := map[string]struct{}{}
	out := make([]string, 0, len(itemMatches))
	for _, itemMatch := range itemMatches {
		if len(itemMatch) < 2 {
			continue
		}
		tableName := strings.ToLower(strings.TrimSpace(itemMatch[1]))
		if _, exists := seen[tableName]; exists {
			t.Fatalf("migration %s workspace_tables ARRAY has duplicate table %q", path, tableName)
		}
		seen[tableName] = struct{}{}
		out = append(out, tableName)
	}
	sort.Strings(out)
	return out
}

func firstMatchIndex(body, pattern string) int {
	re := regexp.MustCompile(pattern)
	match := re.FindStringIndex(body)
	if match == nil {
		return -1
	}
	return match[0]
}

func lastMatchIndex(body, pattern string) int {
	re := regexp.MustCompile(pattern)
	matches := re.FindAllStringIndex(body, -1)
	if len(matches) == 0 {
		return -1
	}
	return matches[len(matches)-1][0]
}

var v9Enums = []string{
	"account_status",
	"autonomy_level",
	"channel_type",
	"connector_status",
	"consent_status",
	"content_trust",
	"data_class",
	"delegation_status",
	"gate_decision",
	"incident_status",
	"ingress_status",
	"memory_status",
	"memory_type",
	"pairing_status",
	"plan_tier",
	"portability_status",
	"provisioning_status",
	"provisioning_step_status",
	"review_status",
	"risk_level",
	"role_key",
	"sensitivity_label",
	"tool_execution_phase",
	"user_status",
	"workflow_status",
	"workflow_step_status",
	"workspace_status",
}

var v91Enums = []string{
	"capture_trigger",
	"context_export_format",
	"debt_category",
	"debt_severity",
	"debt_status",
	"debt_task_status",
	"dependency_type",
	"exploration_status",
	"feedback_disposition",
	"feedback_type",
	"followup_trigger",
	"goal_horizon",
	"goal_priority",
	"goal_status",
	"lesson_status",
	"pattern_scope",
	"promotion_status",
	"self_mod_action",
	"template_status",
	"trust_event_type",
	"widget_type",
}

var v92Enums = []string{
	"cache_policy_status",
	"cache_scope",
	"compatibility_level",
	"compliance_evidence_status",
	"compliance_framework",
	"context_budget_status",
	"context_item_type",
	"crdt_conflict_status",
	"dsr_request_status",
	"error_category",
	"error_severity",
	"event_schema_status",
	"feature_flag_match_type",
	"feature_flag_status",
	"feature_flag_type",
	"guardrail_action",
	"guardrail_severity",
	"model_override_status",
	"model_tier",
	"pii_policy_status",
	"quarantine_status",
	"rag_chunk_status",
	"rag_eval_status",
	"rag_retrieval_mode",
	"react_exit_reason",
	"sandbox_profile_status",
	"session_intent_status",
	"session_status_v92",
	"streaming_ack_status",
	"streaming_mode",
	"temporal_constraint_priority",
	"temporal_resolution_status",
	"tool_health_status",
	"travel_mode",
}

var v9Tables = []string{
	"accounts",
	"approvals",
	"audit_log_entries",
	"auto_commit_proofs",
	"airport_knowledge",
	"capability_aliases",
	"capability_defs",
	"capability_resolution_cache",
	"canvas_interactions",
	"canvas_sessions",
	"channel_bindings",
	"channel_identity_envelopes",
	"code_repo_profiles",
	"code_repositories",
	"connector_health",
	"connector_success_stats",
	"connector_tools",
	"connectors",
	"consents",
	"content_firewall_logs",
	"delegation_grants",
	"discovery_answers",
	"discovery_questions",
	"discovery_sessions",
	"discovery_unparsed_lines",
	"document_parse_results",
	"environment_signals",
	"execution_gate_decisions",
	"financial_anomaly_events",
	"financial_merchant_rules",
	"ha_entity_cache",
	"incident_notifications",
	"ingress_turns",
	"key_versions",
	"llm_output_replay",
	"mcp_egress_audit_events",
	"mcp_tool_schema_snapshots",
	"memory_exclusion_rules",
	"memory_items",
	"memory_revisions",
	"memory_write_requests",
	"model_catalog",
	"outbox_items",
	"pairing_invitations",
	"portability_requests",
	"prompt_versions",
	"provisioning_declined",
	"provisioning_events",
	"provisioning_monthly_budgets",
	"provisioning_monthly_usage",
	"provisioning_policy_versions",
	"provisioning_ranker_versions",
	"provisioning_requests",
	"provisioning_steps",
	"rate_limit_events",
	"review_tasks",
	"role_bindings",
	"roles",
	"routing_policies",
	"runtime_profiles",
	"semantic_verifier_failures",
	"server_artifacts",
	"server_capability_bindings",
	"server_catalog",
	"specialist_agents",
	"synthesis_evidence_items",
	"synthesis_evidence_receipts",
	"system_autonomy_overrides",
	"tool_capability_bindings",
	"tool_executions",
	"tool_inventory",
	"trajectories",
	"trajectory_tool_calls",
	"transcription_logs",
	"trust_receipt_evidence",
	"trust_receipts",
	"user_channels",
	"user_connector_settings",
	"user_domain_autonomy_settings",
	"user_entity_fingerprints",
	"user_oauth_tokens",
	"users",
	"voice_profiles",
	"workflow_instances",
	"workflow_steps",
	"workspace_behavior_policies",
	"workspace_mcp_servers",
	"workspace_personas",
	"workspace_profiles",
	"workspace_server_rules",
	"workspaces",
}

var v91Tables = []string{
	"autonomy_promotions",
	"autonomy_trust_scores",
	"capability_recommendations",
	"code_context_exports",
	"cross_repo_patterns",
	"daily_captures",
	"debt_resolution_tasks",
	"discovery_adaptive_questions",
	"discovery_followup_rules",
	"exploration_history",
	"goal_items",
	"goal_milestones",
	"goal_progress_logs",
	"interaction_feedback",
	"introspection_templates",
	"learned_lessons",
	"learning_config",
	"mission_control_config",
	"mission_control_widgets",
	"project_templates",
	"repo_dependencies",
	"self_modification_policies",
	"technical_debt_items",
}

var v92Tables = []string{
	"admin_alert_channels",
	"admin_alert_rules",
	"admin_dashboard_config",
	"admin_kpi_reports",
	"admin_saved_views",
	"cache_audit_log",
	"cache_policies",
	"compliance_dsr_requests",
	"compliance_evidence",
	"compliance_frameworks",
	"constrained_decoding_config",
	"context_budget_allocations",
	"context_budget_audit",
	"context_budgets",
	"conversation_sessions",
	"error_taxonomy",
	"error_templates",
	"event_schema_registry",
	"event_schema_versions",
	"feature_flag_evaluations",
	"feature_flag_rules",
	"feature_flags",
	"guardrails_config",
	"guardrails_events",
	"guardrails_rule_sets",
	"latency_budgets",
	"mcp_sandbox_profiles",
	"memory_conflict_log",
	"memory_vector_clocks",
	"model_tier_overrides",
	"model_tier_policies",
	"pii_encryption_policies",
	"rag_chunks",
	"rag_collections",
	"rag_eval_scores",
	"rag_reranker_config",
	"rag_retrievals",
	"react_execution_policies",
	"session_entities",
	"session_intents",
	"streaming_config",
	"temporal_constraints",
	"temporal_reasoning_config",
	"tool_health_events",
	"tool_health_scores",
	"tool_quarantine_rules",
	"travel_time_cache",
}
