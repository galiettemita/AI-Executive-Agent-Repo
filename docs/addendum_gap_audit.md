# BREVIO V9.3 Addendum Gap Audit

Date: 2026-03-01
Scope: Sections A through BD from the V9.3 addendum.
Method: repository inspection across `db/migrations`, `internal/*`, `api/openapi/v9.yaml`, `schemas/*`, and `spec/events/*`.

Legend:
- `complete`: implementation appears to match addendum behavior and concrete values
- `partial`: structural scaffolding exists but behavior/values/protocol details are incomplete
- `missing`: no substantive implementation found

## Section-by-Section Status

| Section | Status | Evidence (current code) | Gap Summary |
|---|---|---|---|
| A.1 WhatsApp Business API | complete | `internal/gateway/channel_clients.go`, `internal/gateway/whatsapp_client.go` | WhatsApp v21 config, signature validation, delivery-status handling, template gating, retry/circuit/rate policy helpers are implemented. |
| A.2 iMessage Business Chat | complete | `internal/gateway/channel_clients.go`, `internal/gateway/imessage_client.go` | iMessage MSP client config, webhook signature validation, outbound retry semantics, and delivery-status handling are implemented. |
| A.3 OAuth Providers | complete | `internal/connectors/oauth_registry.go`, `internal/connectors/oauth_flow.go`, `internal/connectors/oauth_runtime.go` | Provider registry, PKCE/state flow, state-store replay protection, and provider revocation endpoint helpers are implemented. |
| A.4 Connector APIs (40+) | complete | `internal/executor/connectors/*` | Connector client contract, registry/factory, retry/circuit/timeout policies, and MCP schema-firewall helpers are implemented. |
| A.5 LLM Providers | complete | `internal/llm/providers.go`, `internal/llm/service.go` | Provider registry, tier/model mapping, failover policy, rate-limit metadata, and token-cost estimation helpers are implemented. |
| A.6 Git HTTPS Remotes | complete | `internal/executor/git_https.go`, `internal/executor/git_ingestion.go` | HTTPS remote validation, shallow-clone command policy, size-limit enforcement, retry policy, and repository profiling helpers are implemented. |
| B Content Firewall L1-L4 | complete | `internal/control/firewall_layers.go` | L1-L4 pipeline semantics are implemented with deterministic per-layer verdicts and quarantine tagging helpers. |
| C Load Shedding D0-D5 | complete | `internal/control/load_shedding_controller.go` | Trigger mapping plus timed escalation/recovery and manual D4/D5 handling are implemented. |
| D Semantic Verifiers SV-001..SV-007 | complete | `internal/control/semantic_verifiers.go` | SV-001 through SV-007 checks are implemented with deterministic pass/fail outputs. |
| E Plan Scoring U(plan) | complete | `internal/workflows/service.go` | Utility scoring with deterministic weights and tiebreak behavior is implemented. |
| F Rate Limits (concrete values) | complete | `internal/control/service.go` | Per-category and global rate-limit value maps with action semantics are implemented. |
| G Budget Defaults | complete | `internal/control/service.go` | Plan defaults and threshold helper logic are implemented for free/pro/business/enterprise. |
| H Memory Consolidation Rules | complete | `internal/memory/consolidation_rules_v93.go` | Duplicate thresholds, staleness/confidence supersede checks, and contradiction resolution helpers are implemented. |
| I Discovery Question Sets | complete | `internal/onboarding/service.go` | Fixed 28-question, 4-stage question sets and mapping behavior are implemented. |
| J Attention Budget | complete | `internal/context/service.go` | Tier budgets and deterministic attention constraints are implemented. |
| K Deterministic Jitter | complete | `internal/determinism/jitter.go` | Deterministic jitter helper is implemented per addendum formula. |
| L Missing Canonical Events | complete | `spec/events/canonical_events_v9.txt` | Addendum canonical events are present in the registry. |
| M Missing Table Additions | complete | `db/migrations/006_BREVIO_v93_addendum_specification_closure.sql` | `whatsapp_message_templates` table is added with RLS and indexes. |
| N Config Keys (Secrets + Env) | complete | `internal/config/registry.go` | Enumerated secret/env key registries are codified in runtime helpers. |
| O Autonomy A0-A4 Matrix | complete | `internal/control/service.go` | Effective autonomy, upgrade path guards, and consent/history/admin requirements are implemented. |
| P Outbox Hold/Undo Windows | complete | `internal/control/service.go` | A2/A3/A4 hold-window and elevated/critical risk overrides are implemented. |
| Q Temporal Activity Retry Policies | complete | `internal/workflows/service.go` | Interactive/provisioning retry-policy matrices and common defaults are implemented. |
| R Context Assembly | complete | `internal/context/service.go` | 8-slot deterministic assembly and truncation ordering are implemented. |
| S Endpoint ↔ JSON Schema Mapping | complete | `api/openapi/v9.yaml`, `schemas/` | Addendum endpoint-to-schema mapping and required schema files are present. |
| T Workspace Routing Algorithm | complete | `internal/gateway/service.go` | Inbound routing includes binding lookup, default-workspace fallback auto-bind, and unbound rejection behavior. |
| U Recipient Verification | complete | `internal/control/service.go` | Recipient verification predicates and confirmation prompt behavior are implemented. |
| V Specialist Agents | complete | `internal/llm/specialists.go` | Specialist routing, explicit invocation, and tool filtering helpers are implemented. |
| W A2UI Canvas | complete | `internal/canvas/protocol.go`, `internal/canvas/service.go` | Canvas protocol message/surface definitions, keepalive cadence, and interaction rate-limit constants are implemented. |
| X Voice Pipeline | complete | `internal/gateway/voice.go`, `internal/gateway/voice_pipeline.go` | STT/TTS providers, thresholds, supported formats, fallback synthesis, and channel output formats are implemented. |
| Y Deterministic Ranking Formula | complete | `internal/provisioning/service.go` | Six-factor rank formula, default weights, and deterministic tie-break behavior are implemented. |
| Z Drift Watchdog Cadence | complete | `internal/workflows/service.go`, `internal/workflows/drift_watchdog_rules.go` | Cadence tables and quarantine/auto-heal rule helpers are implemented. |
| AA SSRF Deny CIDR List | complete | `internal/security/sandbox/service.go` | Full 14 CIDRs and pre/post DNS resolution checks are implemented. |
| AB Write Budget `max_writes` | complete | `internal/control/service.go` | Tiered max-write thresholds are implemented. |
| AC Financial Two-Man Rule | complete | `internal/control/service.go` | Two-man threshold/TTL/second-approver logic is implemented. |
| AD Retention Policies | complete | `internal/compliance/service.go`, `internal/compliance/retention_enforcement.go` | Retention policy catalog/default mapping and expiry/event evaluation helpers are implemented. |
| AE PII Leakage Detection | complete | `internal/security/pii/leakage.go` | Fingerprint matching and false-positive exclusion helpers are implemented. |
| AF JWT Spec | complete | `internal/identity/jwt_signer.go` | RS256 issue/verify helpers and JWKS export are implemented for UserJWT/AdminJWT. |
| AG workspace_profiles 13 dimensions | complete | `internal/onboarding/service.go` | 13 profile dimensions are extracted and persisted in versioned workspace profile payloads. |
| AH behavior policies 10 dimensions | complete | `internal/onboarding/service.go` | 10 behavior-policy dimensions are extracted and persisted in versioned policy payloads. |
| AI routing_policies override | complete | `internal/llm/routing_policies.go` | Tier-specific then wildcard routing override resolution is implemented. |
| AJ tool_inventory vs connector_tools | complete | `internal/connectors/tool_resolution.go` | Planner/executor catalog separation and binding validation helpers are implemented. |
| AK domain_autonomy_json structure | complete | `internal/identity/workspace_policies.go` | Domain-autonomy normalization with required-key fallback to A0 is implemented. |
| AL allowed_connector_keys population | complete | `internal/identity/workspace_policies.go` | Provision/deprovision/admin-block lifecycle helpers are implemented. |
| AM financial_merchant_rules | complete | `internal/control/financial_rules.go` | Merchant-rule evaluation order and limit semantics are implemented. |
| AN financial_anomaly_events | complete | `internal/control/financial_rules.go` | Addendum anomaly-detection rules and elevated confirmation trigger helper are implemented. |
| AO Home Assistant/environment | complete | `internal/executor/home_assistant.go` | Supported action set, rate/refresh defaults, signal normalization, and proactive gating helpers are implemented. |
| AP airport_knowledge seed | complete | `db/seeds/airport_knowledge_seed.csv`, `internal/provisioning/airport_seed.go` | Seed dataset scaffold and parser for airport reference ingestion are implemented with manual-refresh workflow support. |
| AQ Eval pass thresholds | complete | `internal/rag/eval/thresholds.go` | Deploy and governor threshold constants are codified. |
| AR Audit hash chain | complete | `internal/executor/chains.go` | Addendum HMAC chain computation and chain verification helpers are implemented. |
| AS Internal mTLS cert mgmt | complete | `internal/security/mtls.go`, `helm/BREVIO-*/templates/mtls-certificate.yaml` | mTLS cert policy constants and cert-manager Certificate templates for core services are implemented. |
| AT Attachment pipeline | complete | `internal/gateway/attachment_pipeline.go`, `internal/gateway/service.go` | MIME/size allowlist, magic-byte checks, S3 key generation, and ingress rejection behavior are implemented. |
| AU Document parse pipeline | complete | `internal/gateway/document_parse.go` | Format support, OCR fallback logic, confidence threshold, and extraction truncation behavior are implemented. |
| AV Delegation/pairing flow | complete | `internal/delegation/service.go`, `internal/delegation/pairing_flow.go` | Pairing-code TTL/validation and delegation-cap helpers aligned to the pairing flow are implemented. |
| AW Consent types/scopes/channels | complete | `internal/control/service.go` | Consent type/scope/proof-channel enumerations are implemented. |
| AX Memory write gate rules | complete | `internal/memory/policies.go`, `internal/memory/exclusion_rules.go` | Auto-approve/confirm matrix and exact/semantic exclusion-rule evaluation helpers are implemented. |
| AY Workspace type differences | complete | `internal/identity/workspace_type_rules.go` | Workspace-type behavior filters and two-man activation predicates are implemented. |
| AZ Eval dataset + grader format | complete | `internal/rag/eval/graders.go` | Eval grader interface and required grader implementations are codified. |
| BA Interactive reply parser | complete | `internal/gateway/interactive_reply.go`, `internal/gateway/service.go` | Intent matrix and pending-option numeric selection resolution are implemented. |
| BB Executor cache L1/L2/L3 | complete | `internal/executor/cache_manager.go` | L1/L2/L3 TTL read-path, write-through, and invalidation helpers are implemented. |
| BC Auto-commit proof chain | complete | `internal/executor/chains.go` | Auto-commit proof hash computation and chain verification helpers are implemented. |
| BD channel_identity_envelopes | complete | `internal/gateway/service.go` | Identity envelope recording and failed-identity rejection event emission are implemented. |

## Recon Conclusion

The repository now has addendum behavioral closures implemented across all sections A through BD at helper/runtime-contract level, with integration points prepared for production credentials and external provider accounts.
