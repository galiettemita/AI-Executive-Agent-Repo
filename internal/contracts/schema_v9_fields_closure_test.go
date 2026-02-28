package contracts

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestV9SchemaFieldClosure(t *testing.T) {
	t.Parallel()

	toolCall := loadSchemaDocument(t, "tool_call.v9.json")
	toolCallProps := getObject(t, toolCall, "properties")
	assertHasProperties(t, toolCallProps, "tool_key", "idempotency_key", "arguments", "requested_risk", "workspace_id", "ingress_turn_id")
	assertRequiredIncludes(t, toolCall, "tool_key", "idempotency_key", "arguments", "requested_risk", "workspace_id", "ingress_turn_id")
	toolKey := getObject(t, toolCallProps, "tool_key")
	assertStringEquals(t, toolKey["pattern"], "^[a-z0-9_]+\\.[a-z0-9_]+$")
	idempotencyKey := getObject(t, toolCallProps, "idempotency_key")
	assertNumberEquals(t, idempotencyKey["minLength"], 16)
	assertNumberEquals(t, idempotencyKey["maxLength"], 128)

	errorSchema := loadSchemaDocument(t, "error.v9.json")
	errorProps := getObject(t, errorSchema, "properties")
	assertHasProperties(t, errorProps, "error_code", "message", "retryable", "retry_after_ms", "user_message")
	assertRequiredIncludes(t, errorSchema, "error_code", "message", "retryable", "retry_after_ms", "user_message")
	assertStringEquals(t, getObject(t, errorProps, "retryable")["type"], "boolean")

	resolverContract := loadSchemaDocument(t, "capability_resolver_contract.v1.json")
	resolverContractProps := getObject(t, resolverContract, "properties")
	assertHasProperties(t, resolverContractProps, "normalized_query", "tokens", "matched_rules")
	assertRequiredIncludes(t, resolverContract, "normalized_query", "tokens", "matched_rules")
	tokens := getObject(t, resolverContractProps, "tokens")
	assertNumberEquals(t, tokens["maxItems"], 256)

	extractorOutput := loadSchemaDocument(t, "capability_extractor_output.v1.json")
	extractorProps := getObject(t, extractorOutput, "properties")
	assertHasProperties(t, extractorProps, "candidates")
	candidates := getObject(t, extractorProps, "candidates")
	assertNumberEquals(t, candidates["maxItems"], 8)
	candidateItem := getObject(t, candidates, "items")
	candidateItemProps := getObject(t, candidateItem, "properties")
	assertHasProperties(t, candidateItemProps, "capability_key", "confidence", "evidence")
	confidence := getObject(t, candidateItemProps, "confidence")
	assertNumberEquals(t, confidence["minimum"], 0)
	assertNumberEquals(t, confidence["maximum"], 1)
	evidence := getObject(t, candidateItemProps, "evidence")
	assertNumberEquals(t, evidence["maxLength"], 500)

	resolveRequest := loadSchemaDocument(t, "capability_resolve_request.v1.json")
	resolveRequestProps := getObject(t, resolveRequest, "properties")
	queryText := getObject(t, resolveRequestProps, "query_text")
	assertNumberEquals(t, queryText["minLength"], 1)
	assertNumberEquals(t, queryText["maxLength"], 2000)
	allowLLMFallback := getObject(t, resolveRequestProps, "allow_llm_fallback")
	assertStringEquals(t, allowLLMFallback["type"], "boolean")
	maxCandidates := getObject(t, resolveRequestProps, "max_candidates")
	assertNumberEquals(t, maxCandidates["minimum"], 1)
	assertNumberEquals(t, maxCandidates["maximum"], 20)

	resolveResponse := loadSchemaDocument(t, "capability_resolve_response.v1.json")
	resolveResponseProps := getObject(t, resolveResponse, "properties")
	assertHasProperties(t, resolveResponseProps, "normalized_query_hash", "capabilities", "recommended_servers")
	assertRequiredIncludes(t, resolveResponse, "normalized_query_hash", "capabilities", "recommended_servers")

	provisioningPolicy := loadSchemaDocument(t, "provisioning_policy.v1.json")
	provisioningPolicyProps := getObject(t, provisioningPolicy, "properties")
	assertHasProperties(t, provisioningPolicyProps, "max_allowed_risk_level", "require_operator_review_at_or_above", "allowed_server_ids", "denied_server_ids", "oauth_owner_approval_required", "mcp_deploy_owner_approval_required")
	assertRequiredIncludes(t, provisioningPolicy, "max_allowed_risk_level", "require_operator_review_at_or_above", "allowed_server_ids", "denied_server_ids", "oauth_owner_approval_required", "mcp_deploy_owner_approval_required")

	provisionStartRequest := loadSchemaDocument(t, "provision_start_request.v1.json")
	provisionStartProps := getObject(t, provisionStartRequest, "properties")
	requestedCapabilityKeys := getObject(t, provisionStartProps, "requested_capability_keys")
	assertNumberEquals(t, requestedCapabilityKeys["maxItems"], 50)
	trigger := getObject(t, provisionStartProps, "trigger")
	assertEnumEquals(t, trigger["enum"], "capability_gap", "user_request", "operator_action")

	serverArtifactManifest := loadSchemaDocument(t, "server_artifact_manifest.v1.json")
	serverArtifactProps := getObject(t, serverArtifactManifest, "properties")
	assertHasProperties(t, serverArtifactProps, "image_digest", "sbom_s3_uri", "vuln_scan_summary_json", "signature_bundle_json")
	imageDigest := getObject(t, serverArtifactProps, "image_digest")
	assertStringEquals(t, imageDigest["pattern"], "^sha256:[a-f0-9]{64}$")

	llmRequest := loadSchemaDocument(t, "llm_request.v1.json")
	llmProps := getObject(t, llmRequest, "properties")
	assertHasProperties(t, llmProps, "model_id", "provider_id", "seed_int", "temperature", "top_p", "max_output_tokens", "prompt_text", "response_schema_id")
	temperature := getObject(t, llmProps, "temperature")
	assertNumberEquals(t, temperature["const"], 0)
	topP := getObject(t, llmProps, "top_p")
	assertNumberEquals(t, topP["const"], 1)

	provisioningApprovalMessage := loadSchemaDocument(t, "provisioning_approval_message.v1.json")
	provisioningApprovalProps := getObject(t, provisioningApprovalMessage, "properties")
	assertNumberEquals(t, getObject(t, provisioningApprovalProps, "title")["maxLength"], 80)
	assertNumberEquals(t, getObject(t, provisioningApprovalProps, "body")["maxLength"], 700)
	assertNumberEquals(t, getObject(t, provisioningApprovalProps, "cta_approve")["maxLength"], 40)
	assertNumberEquals(t, getObject(t, provisioningApprovalProps, "cta_deny")["maxLength"], 40)

	provisioningStatusMessage := loadSchemaDocument(t, "provisioning_status_message.v1.json")
	provisioningStatusProps := getObject(t, provisioningStatusMessage, "properties")
	assertNumberEquals(t, getObject(t, provisioningStatusProps, "message")["maxLength"], 600)
	assertStringEquals(t, getObject(t, provisioningStatusProps, "show_auth_link")["type"], "boolean")

	provisioningSecurityJustification := loadSchemaDocument(t, "provisioning_security_justification.v1.json")
	provisioningSecurityProps := getObject(t, provisioningSecurityJustification, "properties")
	assertNumberEquals(t, getObject(t, provisioningSecurityProps, "summary")["maxLength"], 700)

	provisioningRankExplainer := loadSchemaDocument(t, "provisioning_rank_explainer.v1.json")
	provisioningRankProps := getObject(t, provisioningRankExplainer, "properties")
	assertNumberEquals(t, getObject(t, provisioningRankProps, "explanation")["maxLength"], 500)

	actionProposal := loadSchemaDocument(t, "action_proposal.v1.json")
	actionProposalProps := getObject(t, actionProposal, "properties")
	assertHasProperties(t, actionProposalProps, "intent", "actions", "risk", "requires_approval")
	actions := getObject(t, actionProposalProps, "actions")
	actionItems := getObject(t, actions, "items")
	actionItemProps := getObject(t, actionItems, "properties")
	assertHasProperties(t, actionItemProps, "tool", "operation", "params", "idempotency_key")
	actionTool := getObject(t, actionItemProps, "tool")
	assertStringEquals(t, actionTool["pattern"], "^[a-z0-9_]+\\.[a-z0-9_]+$")
	actionIdempotencyKey := getObject(t, actionItemProps, "idempotency_key")
	assertNumberEquals(t, actionIdempotencyKey["minLength"], 16)
	assertNumberEquals(t, actionIdempotencyKey["maxLength"], 128)
	risk := getObject(t, actionProposalProps, "risk")
	riskProps := getObject(t, risk, "properties")
	assertHasProperties(t, riskProps, "impact", "rollback_plan")
}

func loadSchemaDocument(t *testing.T, schemaFile string) map[string]any {
	t.Helper()

	path := filepath.Join(repositoryRoot(t), "schemas", schemaFile)
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read schema %s: %v", schemaFile, err)
	}

	var doc map[string]any
	if err := json.Unmarshal(body, &doc); err != nil {
		t.Fatalf("parse schema %s: %v", schemaFile, err)
	}
	if doc["additionalProperties"] != false {
		t.Fatalf("schema %s must set additionalProperties=false", schemaFile)
	}
	return doc
}

func getObject(t *testing.T, object map[string]any, key string) map[string]any {
	t.Helper()

	raw, ok := object[key]
	if !ok {
		t.Fatalf("missing object key %q", key)
	}
	typed, ok := raw.(map[string]any)
	if !ok {
		t.Fatalf("key %q must be an object", key)
	}
	return typed
}

func assertHasProperties(t *testing.T, properties map[string]any, keys ...string) {
	t.Helper()

	for _, key := range keys {
		if _, ok := properties[key]; !ok {
			t.Fatalf("missing property %q", key)
		}
	}
}

func assertRequiredIncludes(t *testing.T, doc map[string]any, keys ...string) {
	t.Helper()

	requiredRaw, ok := doc["required"].([]any)
	if !ok {
		t.Fatal("schema required list missing")
	}
	required := map[string]struct{}{}
	for _, item := range requiredRaw {
		asString, ok := item.(string)
		if !ok {
			t.Fatalf("schema required entry must be string: %T", item)
		}
		required[asString] = struct{}{}
	}
	for _, key := range keys {
		if _, ok := required[key]; !ok {
			t.Fatalf("required list missing key %q", key)
		}
	}
}

func assertStringEquals(t *testing.T, raw any, expected string) {
	t.Helper()

	asString, ok := raw.(string)
	if !ok {
		t.Fatalf("expected string value %q, got %T", expected, raw)
	}
	if asString != expected {
		t.Fatalf("unexpected string value: got=%q want=%q", asString, expected)
	}
}

func assertNumberEquals(t *testing.T, raw any, expected float64) {
	t.Helper()

	asNumber, ok := raw.(float64)
	if !ok {
		t.Fatalf("expected numeric value %v, got %T", expected, raw)
	}
	if asNumber != expected {
		t.Fatalf("unexpected numeric value: got=%v want=%v", asNumber, expected)
	}
}

func assertEnumEquals(t *testing.T, raw any, expected ...string) {
	t.Helper()

	enumValues, ok := raw.([]any)
	if !ok {
		t.Fatalf("expected enum array, got %T", raw)
	}
	if len(enumValues) != len(expected) {
		t.Fatalf("unexpected enum length: got=%d want=%d", len(enumValues), len(expected))
	}
	for idx, want := range expected {
		got, ok := enumValues[idx].(string)
		if !ok {
			t.Fatalf("enum index %d must be string, got %T", idx, enumValues[idx])
		}
		if got != want {
			t.Fatalf("unexpected enum value at index %d: got=%q want=%q", idx, got, want)
		}
	}
}
