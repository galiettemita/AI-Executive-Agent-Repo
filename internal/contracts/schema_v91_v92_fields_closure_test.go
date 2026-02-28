package contracts

import "testing"

func TestV91V92SchemaFieldClosure(t *testing.T) {
	t.Parallel()

	cases := []struct {
		file       string
		properties []string
	}{
		{file: "goal_item.v1.json", properties: []string{"goal_id", "workspace_id", "title", "horizon", "status", "priority", "target_date"}},
		{file: "goal_progress_update.v1.json", properties: []string{"goal_id", "workspace_id", "progress_pct", "note", "evidence_refs", "updated_at"}},
		{file: "mission_control_layout.v1.json", properties: []string{"workspace_id", "refresh_cadence_minutes", "widgets"}},
		{file: "trust_score_report.v1.json", properties: []string{"workspace_id", "trust_score", "success_count_30d", "failure_count_30d", "override_count_30d", "evaluated_at"}},
		{file: "promotion_proposal.v1.json", properties: []string{"workspace_id", "proposal_id", "current_autonomy_level", "proposed_autonomy_level", "reason", "eligible"}},
		{file: "feedback_submission.v1.json", properties: []string{"workspace_id", "feedback_type", "disposition", "message", "ingress_turn_id"}},
		{file: "lesson_proposal.v1.json", properties: []string{"lesson_id", "workspace_id", "title", "summary", "status"}},
		{file: "daily_capture_output.v1.json", properties: []string{"workspace_id", "capture_date", "summary", "wins", "blockers", "next_actions"}},
		{file: "capability_recommendation.v1.json", properties: []string{"recommendation_id", "workspace_id", "capability_key", "confidence", "reason", "status"}},
		{file: "code_context_export_request.v1.json", properties: []string{"workspace_id", "repository_id", "format", "scope", "include_dependencies"}},
		{file: "debt_resolution_task.v1.json", properties: []string{"task_id", "debt_item_id", "title", "severity", "status", "owner_user_id"}},
		{file: "discovery_followup.v1.json", properties: []string{"followup_id", "workspace_id", "question", "trigger", "status"}},
		{file: "morning_briefing.v1.json", properties: []string{"workspace_id", "briefing_date", "headline", "priorities", "risks", "agenda"}},
		{file: "context_budget_config.v1.json", properties: []string{"workspace_id", "tier", "max_context_tokens", "reserved_response_tokens", "max_rag_tokens"}},
		{file: "context_allocation_report.v1.json", properties: []string{"ingress_turn_id", "total_budget_tokens", "allocated_prompt_tokens", "allocated_rag_tokens", "allocated_history_tokens"}},
		{file: "rag_collection_config.v1.json", properties: []string{"collection_id", "workspace_id", "name", "embedding_model", "chunk_size", "bm25_enabled"}},
		{file: "rag_search_request.v1.json", properties: []string{"workspace_id", "collection_id", "query", "top_k", "include_provenance"}},
		{file: "rag_search_response.v1.json", properties: []string{"retrieval_id", "query_rewrite", "results"}},
		{file: "session_context.v1.json", properties: []string{"session_id", "conversation_id", "workspace_id", "active_intent", "entities"}},
		{file: "temporal_expression.v1.json", properties: []string{"expression", "timezone", "reference_ts"}},
		{file: "scheduling_conflict_report.v1.json", properties: []string{"has_conflict", "resolution_hint", "conflicts"}},
		{file: "guardrail_event.v1.json", properties: []string{"event_id", "rule_key", "event_type", "severity", "blocked", "reason"}},
		{file: "tool_health_report.v1.json", properties: []string{"workspace_id", "tool_key", "score", "status", "latency_ms", "error_rate"}},
		{file: "feature_flag_evaluation.v1.json", properties: []string{"flag_key", "workspace_id", "enabled", "variant", "reason"}},
		{file: "error_message.v1.json", properties: []string{"error_code", "user_message", "retryable", "next_action"}},
		{file: "compliance_evidence_manifest.v1.json", properties: []string{"framework", "evidence_id", "artifact_hash", "artifact_uri", "collected_at"}},
		{file: "dsr_request.v1.json", properties: []string{"request_id", "workspace_id", "request_type", "status", "deadline_at", "subject_user_id"}},
		{file: "admin_kpi_report.v1.json", properties: []string{"report_period", "generated_at", "kpis"}},
		{file: "admin_alert.v1.json", properties: []string{"alert_id", "rule_key", "severity", "message", "fired_at"}},
		{file: "memory_conflict_report.v1.json", properties: []string{"conflict_id", "workspace_id", "entity_key", "resolution_strategy", "requires_manual_review"}},
		{file: "model_tier_override_request.v1.json", properties: []string{"workspace_id", "target_tier", "reason", "expires_at"}},
	}

	for _, tc := range cases {
		doc := loadSchemaDocument(t, tc.file)
		props := getObject(t, doc, "properties")
		assertHasProperties(t, props, tc.properties...)
		assertRequiredIncludes(t, doc, tc.properties...)
	}

	trustScoreReport := loadSchemaDocument(t, "trust_score_report.v1.json")
	trustScore := getObject(t, getObject(t, trustScoreReport, "properties"), "trust_score")
	assertNumberEquals(t, trustScore["minimum"], -1)
	assertNumberEquals(t, trustScore["maximum"], 1)

	ragSearchRequest := loadSchemaDocument(t, "rag_search_request.v1.json")
	topK := getObject(t, getObject(t, ragSearchRequest, "properties"), "top_k")
	assertNumberEquals(t, topK["minimum"], 1)
	assertNumberEquals(t, topK["maximum"], 50)

	toolHealthReport := loadSchemaDocument(t, "tool_health_report.v1.json")
	toolKey := getObject(t, getObject(t, toolHealthReport, "properties"), "tool_key")
	assertStringEquals(t, toolKey["pattern"], "^[a-z0-9_]+\\.[a-z0-9_]+$")

	complianceEvidenceManifest := loadSchemaDocument(t, "compliance_evidence_manifest.v1.json")
	artifactHash := getObject(t, getObject(t, complianceEvidenceManifest, "properties"), "artifact_hash")
	assertStringEquals(t, artifactHash["pattern"], "^[a-f0-9]{64}$")

	adminAlert := loadSchemaDocument(t, "admin_alert.v1.json")
	adminAlertMessage := getObject(t, getObject(t, adminAlert, "properties"), "message")
	assertNumberEquals(t, adminAlertMessage["maxLength"], 500)

	temporalExpression := loadSchemaDocument(t, "temporal_expression.v1.json")
	expression := getObject(t, getObject(t, temporalExpression, "properties"), "expression")
	assertNumberEquals(t, expression["maxLength"], 500)
}
