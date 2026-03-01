package contracts

import (
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

type openapiDocument struct {
	Paths map[string]map[string]any `yaml:"paths"`
}

func TestOpenAPIV9EndpointParityClosure(t *testing.T) {
	t.Parallel()

	doc := loadOpenAPIDoc(t)
	if len(doc.Paths) < 95 {
		t.Fatalf("openapi path count mismatch: got=%d want>=95", len(doc.Paths))
	}

	required := []string{
		"GET /v1/gateway/webhook/whatsapp",
		"POST /v1/gateway/webhook/whatsapp",
		"POST /v1/gateway/webhook/imessage",
		"POST /v1/webhooks/whatsapp",
		"POST /v1/webhooks/imessage",
		"POST /v1/gateway/outbound/send",
		"POST /v1/gateway/inject/tool_call",
		"GET /v1/user/activity-ledger",
		"GET /v1/user/trust-receipts/{id}/evidence",
		"POST /v1/capabilities/resolve",
		"POST /v1/provision/start",
		"GET /v1/provision/status/{id}",
		"GET /v1/provision/callback",
		"GET /v1/catalog/search",
		"GET /v1/workspaces/{id}/provisioning/policy",
		"PUT /v1/workspaces/{id}/provisioning/policy",
		"PUT /v1/workspaces/{id}/provisioning/budget",
		"GET /v1/admin/forensics/replay/{turn_id}",
		"PUT /v1/admin/server-catalog/{id}/artifacts",
		"GET /v1/admin/llm/replay/{hash}",
		"GET /v1/admin/review-tasks",
		"POST /v1/admin/review-tasks/{id}/decide",
		"POST /v1/brain/turn",
		"POST /v1/control/plan/evaluate",
		"POST /v1/hands/tool/execute",
		"POST /v1/canvas/push",
		"GET /v1/canvas/ws",
		"GET /healthz/ready",
		"GET /healthz/live",
		"GET /v1/goals",
		"POST /v1/goals",
		"GET /v1/goals/{id}",
		"PUT /v1/goals/{id}",
		"DELETE /v1/goals/{id}",
		"GET /v1/goals/{id}/milestones",
		"POST /v1/goals/{id}/milestones",
		"GET /v1/goals/{id}/progress",
		"GET /v1/mission-control/config",
		"PUT /v1/mission-control/config",
		"GET /v1/mission-control/widgets",
		"PUT /v1/mission-control/widgets",
		"GET /v1/mission-control/snapshot",
		"GET /v1/autonomy/trust-scores",
		"GET /v1/autonomy/promotions",
		"POST /v1/autonomy/promotions/{id}/decide",
		"GET /v1/learning/config",
		"PUT /v1/learning/config",
		"POST /v1/learning/feedback",
		"GET /v1/learning/lessons",
		"POST /v1/learning/lessons/{id}/confirm",
		"POST /v1/learning/lessons/{id}/retire",
		"GET /v1/captures/daily",
		"GET /v1/captures/daily/{date}",
		"GET /v1/codebase/dependencies",
		"GET /v1/codebase/patterns",
		"GET /v1/codebase/debt",
		"PUT /v1/codebase/debt/{id}",
		"GET /v1/codebase/debt/{id}/tasks",
		"POST /v1/codebase/debt/{id}/tasks",
		"GET /v1/codebase/debt/{id}/tasks/{task_id}",
		"PUT /v1/codebase/debt/{id}/tasks/{task_id}",
		"GET /v1/codebase/templates",
		"POST /v1/codebase/templates",
		"POST /v1/codebase/context-export",
		"GET /v1/codebase/context-export/{id}",
		"GET /v1/capabilities/recommendations",
		"POST /v1/capabilities/recommendations/{id}/decide",
		"GET /v1/self-modification/policy",
		"PUT /v1/self-modification/policy",
		"POST /v1/admin/trust-scores/recalculate",
		"POST /v1/admin/learning/lessons/bulk-retire",
		"GET /v1/context/budget",
		"PUT /v1/context/budget",
		"GET /v1/context/allocations",
		"GET /v1/rag/collections",
		"POST /v1/rag/collections",
		"GET /v1/rag/collections/{id}",
		"PUT /v1/rag/collections/{id}",
		"DELETE /v1/rag/collections/{id}",
		"POST /v1/rag/collections/{id}/ingest",
		"POST /v1/rag/search",
		"GET /v1/rag/retrievals/{turn_id}",
		"GET /v1/rag/eval/scores",
		"GET /v1/sessions/active",
		"GET /v1/sessions/{id}",
		"GET /v1/sessions/{id}/entities",
		"GET /v1/temporal/config",
		"PUT /v1/temporal/config",
		"GET /v1/temporal/constraints",
		"POST /v1/temporal/constraints",
		"PUT /v1/temporal/constraints/{id}",
		"DELETE /v1/temporal/constraints/{id}",
		"POST /v1/temporal/resolve",
		"POST /v1/temporal/conflicts",
		"POST /v1/temporal/travel-time",
		"GET /v1/guardrails/config",
		"PUT /v1/guardrails/config",
		"GET /v1/guardrails/rule-sets",
		"POST /v1/guardrails/rule-sets",
		"PUT /v1/guardrails/rule-sets/{id}",
		"DELETE /v1/guardrails/rule-sets/{id}",
		"GET /v1/guardrails/events",
		"GET /v1/tools/health",
		"GET /v1/tools/health/{tool_key}",
		"POST /v1/tools/quarantine/{tool_key}/override",
		"GET /v1/tools/quarantine/rules",
		"POST /v1/tools/quarantine/rules",
		"GET /v1/flags",
		"POST /v1/flags",
		"GET /v1/flags/{key}",
		"PUT /v1/flags/{key}",
		"DELETE /v1/flags/{key}",
		"POST /v1/flags/{key}/evaluate",
		"GET /v1/flags/{key}/rules",
		"POST /v1/flags/{key}/rules",
		"GET /v1/streaming/config",
		"PUT /v1/streaming/config",
		"GET /v1/errors/taxonomy",
		"GET /v1/errors/templates",
		"POST /v1/errors/templates",
		"GET /v1/compliance/frameworks",
		"POST /v1/compliance/frameworks",
		"GET /v1/compliance/evidence",
		"GET /v1/compliance/dsr",
		"POST /v1/compliance/dsr",
		"GET /v1/compliance/dsr/{id}",
		"PUT /v1/compliance/dsr/{id}",
		"GET /v1/cache/policies",
		"POST /v1/cache/policies",
		"GET /v1/cache/stats",
		"POST /v1/cache/invalidate",
		"GET /v1/model-tiers/policies",
		"POST /v1/model-tiers/policies",
		"GET /v1/model-tiers/overrides",
		"GET /v1/event-schemas",
		"GET /v1/event-schemas/{type}/versions",
		"POST /v1/event-schemas/{type}/versions",
		"POST /v1/event-schemas/{type}/validate",
		"GET /v1/admin/users",
		"GET /v1/admin/users/{id}",
		"PUT /v1/admin/users/{id}",
		"GET /v1/admin/users/{id}/sessions",
		"GET /v1/admin/operations/dashboard",
		"GET /v1/admin/operations/workflows",
		"GET /v1/admin/operations/queues",
		"GET /v1/admin/costs/summary",
		"GET /v1/admin/costs/anomalies",
		"GET /v1/admin/costs/budgets",
		"PUT /v1/admin/costs/budgets",
		"GET /v1/admin/alerts/rules",
		"POST /v1/admin/alerts/rules",
		"PUT /v1/admin/alerts/rules/{id}",
		"DELETE /v1/admin/alerts/rules/{id}",
		"GET /v1/admin/alerts/channels",
		"POST /v1/admin/alerts/channels",
		"GET /v1/admin/kpi/report",
	}

	requiredSet := make(map[string]struct{}, len(required))
	for _, endpoint := range required {
		parts := strings.SplitN(endpoint, " ", 2)
		if len(parts) != 2 {
			t.Fatalf("invalid required endpoint format: %q", endpoint)
		}
		requiredSet[endpoint] = struct{}{}
	}

	actualSet := map[string]struct{}{}
	for path, operations := range doc.Paths {
		for method := range operations {
			methodUpper := strings.ToUpper(method)
			switch methodUpper {
			case "GET", "POST", "PUT", "DELETE", "PATCH", "OPTIONS", "HEAD", "TRACE":
				actualSet[methodUpper+" "+path] = struct{}{}
			}
		}
	}

	missing := make([]string, 0)
	for endpoint := range requiredSet {
		if _, ok := actualSet[endpoint]; !ok {
			missing = append(missing, endpoint)
		}
	}
	if len(missing) != 0 {
		sort.Strings(missing)
		t.Fatalf("openapi operation set missing required endpoints: %v", missing)
	}

	for endpoint := range requiredSet {
		parts := strings.SplitN(endpoint, " ", 2)
		method := strings.ToLower(parts[0])
		path := parts[1]

		ops, ok := doc.Paths[path]
		if !ok {
			t.Fatalf("missing required openapi path: %s", path)
		}
		if _, ok := ops[method]; !ok {
			t.Fatalf("missing required openapi method: %s %s", strings.ToUpper(method), path)
		}
	}
}

func TestOpenAPIV9OperationIDsArePresentAndUnique(t *testing.T) {
	t.Parallel()

	doc := loadOpenAPIDoc(t)
	seen := map[string]string{}

	for path, operations := range doc.Paths {
		for method, raw := range operations {
			op, ok := raw.(map[string]any)
			if !ok {
				t.Fatalf("openapi operation payload is not an object: %s %s", strings.ToUpper(method), path)
			}

			operationID, ok := op["operationId"].(string)
			if !ok || strings.TrimSpace(operationID) == "" {
				t.Fatalf("missing operationId for %s %s", strings.ToUpper(method), path)
			}

			signature := strings.ToUpper(method) + " " + path
			if prior, exists := seen[operationID]; exists {
				t.Fatalf("duplicate operationId %q for %s and %s", operationID, prior, signature)
			}
			seen[operationID] = signature
		}
	}
}

func TestOpenAPIV9SchemaPointersClosure(t *testing.T) {
	t.Parallel()

	doc := loadOpenAPIDoc(t)

	for path, operations := range doc.Paths {
		for method, raw := range operations {
			methodLower := strings.ToLower(method)
			op, ok := raw.(map[string]any)
			if !ok {
				t.Fatalf("openapi operation payload is not an object: %s %s", strings.ToUpper(methodLower), path)
			}

			if methodLower == "post" || methodLower == "put" || methodLower == "patch" {
				requestBody, ok := op["requestBody"].(map[string]any)
				if !ok {
					t.Fatalf("missing requestBody for %s %s", strings.ToUpper(methodLower), path)
				}
				content, ok := requestBody["content"].(map[string]any)
				if !ok {
					t.Fatalf("missing requestBody.content for %s %s", strings.ToUpper(methodLower), path)
				}
				mediaType, ok := content["application/json"].(map[string]any)
				if !ok {
					t.Fatalf("missing requestBody application/json for %s %s", strings.ToUpper(methodLower), path)
				}
				schema, ok := mediaType["schema"].(map[string]any)
				if !ok {
					t.Fatalf("missing requestBody schema for %s %s", strings.ToUpper(methodLower), path)
				}
				ref, _ := schema["$ref"].(string)
				if strings.TrimSpace(ref) == "" {
					t.Fatalf("missing requestBody schema ref for %s %s", strings.ToUpper(methodLower), path)
				}
			}

			// WebSocket upgrade path is intentionally 101-only.
			if path == "/v1/canvas/ws" && methodLower == "get" {
				continue
			}

			responses, ok := op["responses"].(map[string]any)
			if !ok {
				t.Fatalf("missing responses for %s %s", strings.ToUpper(methodLower), path)
			}

			has2xx := false
			has2xxWithSchema := false
			for statusCode, rawResponse := range responses {
				if !strings.HasPrefix(statusCode, "2") {
					continue
				}
				has2xx = true

				response, ok := rawResponse.(map[string]any)
				if !ok {
					continue
				}
				content, ok := response["content"].(map[string]any)
				if !ok {
					continue
				}
				mediaType, ok := content["application/json"].(map[string]any)
				if !ok {
					continue
				}
				schema, ok := mediaType["schema"].(map[string]any)
				if !ok {
					continue
				}
				ref, _ := schema["$ref"].(string)
				if strings.TrimSpace(ref) != "" {
					has2xxWithSchema = true
				}
			}

			if !has2xx {
				t.Fatalf("missing 2xx response for %s %s", strings.ToUpper(methodLower), path)
			}
			if !has2xxWithSchema {
				t.Fatalf("missing 2xx response schema ref for %s %s", strings.ToUpper(methodLower), path)
			}
		}
	}
}

func TestOpenAPIV9ComponentSchemaCatalogClosure(t *testing.T) {
	t.Parallel()

	root := repositoryRoot(t)
	path := filepath.Join(root, "api", "openapi", "v9.yaml")
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read openapi file: %v", err)
	}

	var doc map[string]any
	if err := yaml.Unmarshal(body, &doc); err != nil {
		t.Fatalf("parse openapi yaml: %v", err)
	}

	components, ok := doc["components"].(map[string]any)
	if !ok {
		t.Fatal("openapi components missing")
	}
	schemas, ok := components["schemas"].(map[string]any)
	if !ok || len(schemas) == 0 {
		t.Fatal("openapi components.schemas missing")
	}
	expectedComponentSchemaKeys := []string{
		"Error",
		"action_proposal_v1_json",
		"admin_alert_v1_json",
		"admin_kpi_report_v1_json",
		"activity_ledger_response_v1_json",
		"capability_extractor_output_v1_json",
		"capability_recommendation_v1_json",
		"capability_resolve_request_v1_json",
		"capability_resolve_response_v1_json",
		"capability_resolver_contract_v1_json",
		"canvas_push_request_v1_json",
		"catalog_search_response_v1_json",
		"code_context_export_request_v1_json",
		"compliance_evidence_manifest_v1_json",
		"context_allocation_report_v1_json",
		"context_budget_config_v1_json",
		"daily_capture_output_v1_json",
		"debt_resolution_task_v1_json",
		"discovery_followup_v1_json",
		"dsr_request_v1_json",
		"error_message_v1_json",
		"error_v9_json",
		"feature_flag_evaluation_v1_json",
		"feedback_submission_v1_json",
		"forensic_replay_response_v1_json",
		"generic_request_v1",
		"generic_response_v1",
		"goal_item_v1_json",
		"goal_progress_update_v1_json",
		"guardrail_event_v1_json",
		"imessage_webhook_payload_v1_json",
		"lesson_proposal_v1_json",
		"llm_request_v1_json",
		"memory_conflict_report_v1_json",
		"mission_control_layout_v1_json",
		"model_tier_override_request_v1_json",
		"morning_briefing_v1_json",
		"outbound_send_request_v1_json",
		"plan_evaluate_request_v1_json",
		"plan_evaluate_response_v1_json",
		"promotion_proposal_v1_json",
		"provision_start_request_v1_json",
		"provision_start_response_v1_json",
		"provision_status_response_v1_json",
		"provisioning_approval_message_v1_json",
		"provisioning_budget_request_v1_json",
		"provisioning_budget_response_v1_json",
		"provisioning_policy_v1_json",
		"provisioning_rank_explainer_v1_json",
		"provisioning_security_justification_v1_json",
		"provisioning_status_message_v1_json",
		"rag_collection_config_v1_json",
		"rag_search_request_v1_json",
		"rag_search_response_v1_json",
		"review_task_decision_v1_json",
		"review_tasks_list_response_v1_json",
		"scheduling_conflict_report_v1_json",
		"server_artifact_manifest_v1_json",
		"session_context_v1_json",
		"temporal_expression_v1_json",
		"tool_execution_response_v1_json",
		"tool_call_v9_json",
		"tool_health_report_v1_json",
		"trust_receipt_evidence_response_v1_json",
		"trust_score_report_v1_json",
		"whatsapp_webhook_payload_v1_json",
		"brain_turn_request_v1_json",
		"brain_turn_response_v1_json",
	}

	actualComponentSchemaKeys := make([]string, 0, len(schemas))
	for key := range schemas {
		actualComponentSchemaKeys = append(actualComponentSchemaKeys, key)
	}
	assertStringSetEquals(t, actualComponentSchemaKeys, expectedComponentSchemaKeys, "openapi components.schemas keys")

	requiredSchemaFiles := []string{
		"tool_call.v9.json",
		"error.v9.json",
		"capability_resolver_contract.v1.json",
		"capability_extractor_output.v1.json",
		"capability_resolve_request.v1.json",
		"capability_resolve_response.v1.json",
		"provisioning_policy.v1.json",
		"provision_start_request.v1.json",
		"server_artifact_manifest.v1.json",
		"llm_request.v1.json",
		"provisioning_approval_message.v1.json",
		"provisioning_status_message.v1.json",
		"provisioning_security_justification.v1.json",
		"provisioning_rank_explainer.v1.json",
		"action_proposal.v1.json",
		"goal_item.v1.json",
		"goal_progress_update.v1.json",
		"mission_control_layout.v1.json",
		"trust_score_report.v1.json",
		"promotion_proposal.v1.json",
		"feedback_submission.v1.json",
		"lesson_proposal.v1.json",
		"daily_capture_output.v1.json",
		"capability_recommendation.v1.json",
		"code_context_export_request.v1.json",
		"debt_resolution_task.v1.json",
		"discovery_followup.v1.json",
		"morning_briefing.v1.json",
		"context_budget_config.v1.json",
		"context_allocation_report.v1.json",
		"rag_collection_config.v1.json",
		"rag_search_request.v1.json",
		"rag_search_response.v1.json",
		"session_context.v1.json",
		"temporal_expression.v1.json",
		"scheduling_conflict_report.v1.json",
		"guardrail_event.v1.json",
		"tool_health_report.v1.json",
		"feature_flag_evaluation.v1.json",
		"error_message.v1.json",
		"compliance_evidence_manifest.v1.json",
		"dsr_request.v1.json",
		"admin_kpi_report.v1.json",
		"admin_alert.v1.json",
		"memory_conflict_report.v1.json",
		"model_tier_override_request.v1.json",
		"whatsapp_webhook_payload.v1.json",
		"imessage_webhook_payload.v1.json",
		"activity_ledger_response.v1.json",
		"trust_receipt_evidence_response.v1.json",
		"provision_start_response.v1.json",
		"provision_status_response.v1.json",
		"catalog_search_response.v1.json",
		"provisioning_budget_request.v1.json",
		"provisioning_budget_response.v1.json",
		"forensic_replay_response.v1.json",
		"review_tasks_list_response.v1.json",
		"review_task_decision.v1.json",
		"brain_turn_request.v1.json",
		"brain_turn_response.v1.json",
		"plan_evaluate_request.v1.json",
		"plan_evaluate_response.v1.json",
		"tool_execution_response.v1.json",
		"outbound_send_request.v1.json",
		"canvas_push_request.v1.json",
	}

	for _, required := range requiredSchemaFiles {
		found := false
		for _, raw := range schemas {
			entry, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			ref, _ := entry["$ref"].(string)
			if strings.HasSuffix(ref, "/"+required) || strings.HasSuffix(ref, required) {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("openapi components.schemas missing reference to %s", required)
		}
	}
}

func TestOpenAPIV9SecurityBindingsClosure(t *testing.T) {
	t.Parallel()

	root := repositoryRoot(t)
	path := filepath.Join(root, "api", "openapi", "v9.yaml")
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read openapi file: %v", err)
	}

	var doc map[string]any
	if err := yaml.Unmarshal(body, &doc); err != nil {
		t.Fatalf("parse openapi yaml: %v", err)
	}

	components, ok := doc["components"].(map[string]any)
	if !ok {
		t.Fatal("openapi components missing")
	}
	securitySchemes, ok := components["securitySchemes"].(map[string]any)
	if !ok {
		t.Fatal("openapi components.securitySchemes missing")
	}
	for _, scheme := range []string{"AdminJWT", "UserJWT", "mTLS"} {
		if _, ok := securitySchemes[scheme]; !ok {
			t.Fatalf("missing security scheme %s", scheme)
		}
	}

	paths, ok := doc["paths"].(map[string]any)
	if !ok {
		t.Fatal("openapi paths missing")
	}

	required := []struct {
		Method string
		Path   string
		Scheme string
	}{
		{Method: "POST", Path: "/v1/rag/search", Scheme: "mTLS"},
		{Method: "GET", Path: "/v1/sessions/active", Scheme: "mTLS"},
		{Method: "POST", Path: "/v1/temporal/resolve", Scheme: "mTLS"},
		{Method: "POST", Path: "/v1/temporal/conflicts", Scheme: "mTLS"},
		{Method: "POST", Path: "/v1/temporal/travel-time", Scheme: "mTLS"},
		{Method: "POST", Path: "/v1/flags/{key}/evaluate", Scheme: "mTLS"},
		{Method: "POST", Path: "/v1/event-schemas/{type}/validate", Scheme: "mTLS"},
		{Method: "POST", Path: "/v1/brain/turn", Scheme: "mTLS"},
		{Method: "POST", Path: "/v1/control/plan/evaluate", Scheme: "mTLS"},
		{Method: "POST", Path: "/v1/hands/tool/execute", Scheme: "mTLS"},
		{Method: "POST", Path: "/v1/canvas/push", Scheme: "mTLS"},
		{Method: "GET", Path: "/v1/user/activity-ledger", Scheme: "UserJWT"},
		{Method: "GET", Path: "/v1/user/trust-receipts/{id}/evidence", Scheme: "UserJWT"},
		{Method: "GET", Path: "/v1/workspaces/{id}/provisioning/policy", Scheme: "UserJWT"},
		{Method: "PUT", Path: "/v1/workspaces/{id}/provisioning/policy", Scheme: "UserJWT"},
		{Method: "PUT", Path: "/v1/workspaces/{id}/provisioning/budget", Scheme: "UserJWT"},
		{Method: "GET", Path: "/v1/compliance/dsr", Scheme: "UserJWT"},
		{Method: "POST", Path: "/v1/compliance/dsr", Scheme: "UserJWT"},
		{Method: "POST", Path: "/v1/admin/trust-scores/recalculate", Scheme: "AdminJWT"},
		{Method: "POST", Path: "/v1/admin/learning/lessons/bulk-retire", Scheme: "AdminJWT"},
		{Method: "GET", Path: "/v1/admin/forensics/replay/{turn_id}", Scheme: "AdminJWT"},
		{Method: "PUT", Path: "/v1/admin/server-catalog/{id}/artifacts", Scheme: "AdminJWT"},
		{Method: "GET", Path: "/v1/admin/llm/replay/{hash}", Scheme: "AdminJWT"},
		{Method: "GET", Path: "/v1/admin/review-tasks", Scheme: "AdminJWT"},
		{Method: "POST", Path: "/v1/admin/review-tasks/{id}/decide", Scheme: "AdminJWT"},
		{Method: "GET", Path: "/v1/guardrails/events", Scheme: "AdminJWT"},
		{Method: "GET", Path: "/v1/errors/taxonomy", Scheme: "AdminJWT"},
		{Method: "POST", Path: "/v1/cache/invalidate", Scheme: "AdminJWT"},
		{Method: "GET", Path: "/v1/event-schemas", Scheme: "AdminJWT"},
		{Method: "GET", Path: "/v1/admin/kpi/report", Scheme: "AdminJWT"},
	}

	for _, item := range required {
		pathValue, ok := paths[item.Path].(map[string]any)
		if !ok {
			t.Fatalf("missing required path for security binding: %s", item.Path)
		}
		op, ok := pathValue[strings.ToLower(item.Method)].(map[string]any)
		if !ok {
			t.Fatalf("missing required method for security binding: %s %s", item.Method, item.Path)
		}
		if !operationHasSecurityScheme(op, item.Scheme) {
			t.Fatalf("missing %s binding for %s %s", item.Scheme, item.Method, item.Path)
		}
	}
}

func TestOpenAPIV9EndpointSchemaSpecializationClosure(t *testing.T) {
	t.Parallel()

	doc := loadOpenAPIDoc(t)

	required := []struct {
		Method      string
		Path        string
		RequestRef  string
		ResponseRef string
	}{
		{
			Method:      "POST",
			Path:        "/v1/gateway/inject/tool_call",
			RequestRef:  "#/components/schemas/tool_call_v9_json",
			ResponseRef: "#/components/schemas/generic_response_v1",
		},
		{
			Method:      "POST",
			Path:        "/v1/webhooks/whatsapp",
			RequestRef:  "#/components/schemas/whatsapp_webhook_payload_v1_json",
			ResponseRef: "#/components/schemas/generic_response_v1",
		},
		{
			Method:      "POST",
			Path:        "/v1/webhooks/imessage",
			RequestRef:  "#/components/schemas/imessage_webhook_payload_v1_json",
			ResponseRef: "#/components/schemas/generic_response_v1",
		},
		{
			Method:      "GET",
			Path:        "/v1/user/activity-ledger",
			ResponseRef: "#/components/schemas/activity_ledger_response_v1_json",
		},
		{
			Method:      "GET",
			Path:        "/v1/user/trust-receipts/{id}/evidence",
			ResponseRef: "#/components/schemas/trust_receipt_evidence_response_v1_json",
		},
		{
			Method:      "POST",
			Path:        "/v1/capabilities/resolve",
			RequestRef:  "#/components/schemas/capability_resolve_request_v1_json",
			ResponseRef: "#/components/schemas/capability_resolve_response_v1_json",
		},
		{
			Method:      "POST",
			Path:        "/v1/provision/start",
			RequestRef:  "#/components/schemas/provision_start_request_v1_json",
			ResponseRef: "#/components/schemas/provision_start_response_v1_json",
		},
		{
			Method:      "GET",
			Path:        "/v1/provision/status/{id}",
			ResponseRef: "#/components/schemas/provision_status_response_v1_json",
		},
		{
			Method:      "GET",
			Path:        "/v1/catalog/search",
			ResponseRef: "#/components/schemas/catalog_search_response_v1_json",
		},
		{
			Method:      "GET",
			Path:        "/v1/workspaces/{id}/provisioning/policy",
			ResponseRef: "#/components/schemas/provisioning_policy_v1_json",
		},
		{
			Method:      "PUT",
			Path:        "/v1/workspaces/{id}/provisioning/policy",
			RequestRef:  "#/components/schemas/provisioning_policy_v1_json",
			ResponseRef: "#/components/schemas/provisioning_policy_v1_json",
		},
		{
			Method:      "PUT",
			Path:        "/v1/workspaces/{id}/provisioning/budget",
			RequestRef:  "#/components/schemas/provisioning_budget_request_v1_json",
			ResponseRef: "#/components/schemas/provisioning_budget_response_v1_json",
		},
		{
			Method:      "GET",
			Path:        "/v1/admin/forensics/replay/{turn_id}",
			ResponseRef: "#/components/schemas/forensic_replay_response_v1_json",
		},
		{
			Method:      "PUT",
			Path:        "/v1/admin/server-catalog/{id}/artifacts",
			RequestRef:  "#/components/schemas/server_artifact_manifest_v1_json",
			ResponseRef: "#/components/schemas/generic_response_v1",
		},
		{
			Method:      "GET",
			Path:        "/v1/admin/llm/replay/{hash}",
			ResponseRef: "#/components/schemas/llm_request_v1_json",
		},
		{
			Method:      "GET",
			Path:        "/v1/admin/review-tasks",
			ResponseRef: "#/components/schemas/review_tasks_list_response_v1_json",
		},
		{
			Method:      "POST",
			Path:        "/v1/admin/review-tasks/{id}/decide",
			RequestRef:  "#/components/schemas/review_task_decision_v1_json",
			ResponseRef: "#/components/schemas/generic_response_v1",
		},
		{
			Method:      "POST",
			Path:        "/v1/brain/turn",
			RequestRef:  "#/components/schemas/brain_turn_request_v1_json",
			ResponseRef: "#/components/schemas/brain_turn_response_v1_json",
		},
		{
			Method:      "POST",
			Path:        "/v1/control/plan/evaluate",
			RequestRef:  "#/components/schemas/plan_evaluate_request_v1_json",
			ResponseRef: "#/components/schemas/plan_evaluate_response_v1_json",
		},
		{
			Method:      "POST",
			Path:        "/v1/hands/tool/execute",
			RequestRef:  "#/components/schemas/tool_call_v9_json",
			ResponseRef: "#/components/schemas/tool_execution_response_v1_json",
		},
		{
			Method:      "POST",
			Path:        "/v1/gateway/outbound/send",
			RequestRef:  "#/components/schemas/outbound_send_request_v1_json",
			ResponseRef: "#/components/schemas/generic_response_v1",
		},
		{
			Method:      "POST",
			Path:        "/v1/canvas/push",
			RequestRef:  "#/components/schemas/canvas_push_request_v1_json",
			ResponseRef: "#/components/schemas/generic_response_v1",
		},
		{
			Method:      "GET",
			Path:        "/v1/goals",
			ResponseRef: "#/components/schemas/goal_item_v1_json",
		},
		{
			Method:      "POST",
			Path:        "/v1/goals",
			RequestRef:  "#/components/schemas/goal_item_v1_json",
			ResponseRef: "#/components/schemas/goal_item_v1_json",
		},
		{
			Method:      "GET",
			Path:        "/v1/goals/{id}/progress",
			ResponseRef: "#/components/schemas/goal_progress_update_v1_json",
		},
		{
			Method:      "PUT",
			Path:        "/v1/mission-control/config",
			RequestRef:  "#/components/schemas/mission_control_layout_v1_json",
			ResponseRef: "#/components/schemas/mission_control_layout_v1_json",
		},
		{
			Method:      "GET",
			Path:        "/v1/mission-control/widgets",
			ResponseRef: "#/components/schemas/mission_control_layout_v1_json",
		},
		{
			Method:      "GET",
			Path:        "/v1/autonomy/trust-scores",
			ResponseRef: "#/components/schemas/trust_score_report_v1_json",
		},
		{
			Method:      "POST",
			Path:        "/v1/autonomy/promotions/{id}/decide",
			RequestRef:  "#/components/schemas/promotion_proposal_v1_json",
			ResponseRef: "#/components/schemas/promotion_proposal_v1_json",
		},
		{
			Method:      "POST",
			Path:        "/v1/learning/feedback",
			RequestRef:  "#/components/schemas/feedback_submission_v1_json",
			ResponseRef: "#/components/schemas/feedback_submission_v1_json",
		},
		{
			Method:      "GET",
			Path:        "/v1/learning/lessons",
			ResponseRef: "#/components/schemas/lesson_proposal_v1_json",
		},
		{
			Method:      "GET",
			Path:        "/v1/captures/daily",
			ResponseRef: "#/components/schemas/daily_capture_output_v1_json",
		},
		{
			Method:      "POST",
			Path:        "/v1/codebase/context-export",
			RequestRef:  "#/components/schemas/code_context_export_request_v1_json",
			ResponseRef: "#/components/schemas/code_context_export_request_v1_json",
		},
		{
			Method:      "GET",
			Path:        "/v1/capabilities/recommendations",
			ResponseRef: "#/components/schemas/capability_recommendation_v1_json",
		},
		{
			Method:      "POST",
			Path:        "/v1/capabilities/recommendations/{id}/decide",
			RequestRef:  "#/components/schemas/capability_recommendation_v1_json",
			ResponseRef: "#/components/schemas/capability_recommendation_v1_json",
		},
		{
			Method:      "PUT",
			Path:        "/v1/context/budget",
			RequestRef:  "#/components/schemas/context_budget_config_v1_json",
			ResponseRef: "#/components/schemas/context_budget_config_v1_json",
		},
		{
			Method:      "GET",
			Path:        "/v1/context/allocations",
			ResponseRef: "#/components/schemas/context_allocation_report_v1_json",
		},
		{
			Method:      "GET",
			Path:        "/v1/rag/collections",
			ResponseRef: "#/components/schemas/rag_collection_config_v1_json",
		},
		{
			Method:      "POST",
			Path:        "/v1/rag/collections",
			RequestRef:  "#/components/schemas/rag_collection_config_v1_json",
			ResponseRef: "#/components/schemas/rag_collection_config_v1_json",
		},
		{
			Method:      "POST",
			Path:        "/v1/rag/search",
			RequestRef:  "#/components/schemas/rag_search_request_v1_json",
			ResponseRef: "#/components/schemas/rag_search_response_v1_json",
		},
		{
			Method:      "GET",
			Path:        "/v1/rag/retrievals/{turn_id}",
			ResponseRef: "#/components/schemas/rag_search_response_v1_json",
		},
		{
			Method:      "GET",
			Path:        "/v1/sessions/active",
			ResponseRef: "#/components/schemas/session_context_v1_json",
		},
		{
			Method:      "GET",
			Path:        "/v1/tools/health/{tool_key}",
			ResponseRef: "#/components/schemas/tool_health_report_v1_json",
		},
		{
			Method:      "POST",
			Path:        "/v1/temporal/conflicts",
			RequestRef:  "#/components/schemas/temporal_expression_v1_json",
			ResponseRef: "#/components/schemas/scheduling_conflict_report_v1_json",
		},
		{
			Method:      "GET",
			Path:        "/v1/guardrails/events",
			ResponseRef: "#/components/schemas/guardrail_event_v1_json",
		},
		{
			Method:      "POST",
			Path:        "/v1/flags/{key}/evaluate",
			RequestRef:  "#/components/schemas/feature_flag_evaluation_v1_json",
			ResponseRef: "#/components/schemas/feature_flag_evaluation_v1_json",
		},
		{
			Method:      "GET",
			Path:        "/v1/model-tiers/overrides",
			ResponseRef: "#/components/schemas/model_tier_override_request_v1_json",
		},
		{
			Method:      "POST",
			Path:        "/v1/admin/alerts/rules",
			RequestRef:  "#/components/schemas/admin_alert_v1_json",
			ResponseRef: "#/components/schemas/admin_alert_v1_json",
		},
		{
			Method:      "POST",
			Path:        "/v1/errors/templates",
			RequestRef:  "#/components/schemas/error_message_v1_json",
			ResponseRef: "#/components/schemas/error_message_v1_json",
		},
		{
			Method:      "POST",
			Path:        "/v1/compliance/dsr",
			RequestRef:  "#/components/schemas/dsr_request_v1_json",
			ResponseRef: "#/components/schemas/dsr_request_v1_json",
		},
		{
			Method:      "PUT",
			Path:        "/v1/compliance/dsr/{id}",
			RequestRef:  "#/components/schemas/dsr_request_v1_json",
			ResponseRef: "#/components/schemas/dsr_request_v1_json",
		},
		{
			Method:      "GET",
			Path:        "/v1/admin/kpi/report",
			ResponseRef: "#/components/schemas/admin_kpi_report_v1_json",
		},
	}

	for _, item := range required {
		ops, ok := doc.Paths[item.Path]
		if !ok {
			t.Fatalf("missing path for schema specialization: %s", item.Path)
		}
		opRaw, ok := ops[strings.ToLower(item.Method)]
		if !ok {
			t.Fatalf("missing method for schema specialization: %s %s", item.Method, item.Path)
		}
		op, ok := opRaw.(map[string]any)
		if !ok {
			t.Fatalf("operation payload is not an object: %s %s", item.Method, item.Path)
		}

		if item.RequestRef != "" {
			requestRef := requestBodySchemaRef(op)
			if requestRef != item.RequestRef {
				t.Fatalf("unexpected request schema ref for %s %s: got=%s want=%s", item.Method, item.Path, requestRef, item.RequestRef)
			}
		}
		if item.ResponseRef != "" {
			if !has2xxResponseSchemaRef(op, item.ResponseRef) {
				t.Fatalf("missing expected 2xx response schema ref for %s %s: %s", item.Method, item.Path, item.ResponseRef)
			}
		}
	}
}

func operationHasSecurityScheme(operation map[string]any, scheme string) bool {
	securityRaw, ok := operation["security"].([]any)
	if !ok {
		return false
	}
	for _, requirementRaw := range securityRaw {
		requirement, ok := requirementRaw.(map[string]any)
		if !ok {
			continue
		}
		if _, exists := requirement[scheme]; exists {
			return true
		}
	}
	return false
}

func requestBodySchemaRef(operation map[string]any) string {
	requestBody, ok := operation["requestBody"].(map[string]any)
	if !ok {
		return ""
	}
	content, ok := requestBody["content"].(map[string]any)
	if !ok {
		return ""
	}
	mediaType, ok := content["application/json"].(map[string]any)
	if !ok {
		return ""
	}
	schema, ok := mediaType["schema"].(map[string]any)
	if !ok {
		return ""
	}
	ref, _ := schema["$ref"].(string)
	return ref
}

func has2xxResponseSchemaRef(operation map[string]any, expectedRef string) bool {
	responses, ok := operation["responses"].(map[string]any)
	if !ok {
		return false
	}
	for statusCode, rawResponse := range responses {
		if !strings.HasPrefix(statusCode, "2") {
			continue
		}
		response, ok := rawResponse.(map[string]any)
		if !ok {
			continue
		}
		content, ok := response["content"].(map[string]any)
		if !ok {
			continue
		}
		mediaType, ok := content["application/json"].(map[string]any)
		if !ok {
			continue
		}
		schema, ok := mediaType["schema"].(map[string]any)
		if !ok {
			continue
		}
		ref, _ := schema["$ref"].(string)
		if ref == expectedRef {
			return true
		}
	}
	return false
}

func assertStringSetEquals(t *testing.T, actual []string, expected []string, label string) {
	t.Helper()

	actualSet := make(map[string]struct{}, len(actual))
	for _, item := range actual {
		actualSet[item] = struct{}{}
	}
	expectedSet := make(map[string]struct{}, len(expected))
	for _, item := range expected {
		expectedSet[item] = struct{}{}
	}

	missing := make([]string, 0)
	for item := range expectedSet {
		if _, ok := actualSet[item]; !ok {
			missing = append(missing, item)
		}
	}
	extra := make([]string, 0)
	for item := range actualSet {
		if _, ok := expectedSet[item]; !ok {
			extra = append(extra, item)
		}
	}
	if len(missing) == 0 && len(extra) == 0 {
		return
	}
	sort.Strings(missing)
	sort.Strings(extra)
	t.Fatalf("%s mismatch: missing=%v extra=%v", label, missing, extra)
}

func loadOpenAPIDoc(t *testing.T) openapiDocument {
	t.Helper()

	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("unable to resolve current file path")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(currentFile), "..", ".."))
	path := filepath.Join(root, "api", "openapi", "v9.yaml")

	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read openapi file: %v", err)
	}

	var doc openapiDocument
	if err := yaml.Unmarshal(body, &doc); err != nil {
		t.Fatalf("parse openapi yaml: %v", err)
	}
	if len(doc.Paths) == 0 {
		t.Fatal("openapi paths are empty")
	}
	return doc
}
