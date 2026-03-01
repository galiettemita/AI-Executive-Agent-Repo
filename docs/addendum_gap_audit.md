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
| A.1 WhatsApp Business API | partial | `internal/gateway/service.go` | Webhook/path scaffolding exists; no Meta v21.0 client, status callbacks mapping, template/re-engagement handling, or spec token bucket/circuit semantics. |
| A.2 iMessage Business Chat | partial | `internal/gateway/service.go` | iMessage ingress path exists; no MSP mTLS relay client contract, signature policy, delivery callback semantics, or outbound protocol implementation. |
| A.3 OAuth Providers | partial | `internal/connectors/oauth_registry.go`, `internal/connectors/oauth_flow.go` | Provider registry and PKCE/state HMAC helpers are implemented; external callback/revocation execution wiring remains partial in runtime service paths. |
| A.4 Connector APIs (40+) | partial | `internal/connectors/seeds/connectors.yaml` | Seed inventory exists but differs from launch-set semantics; no complete native/MCP client architecture with uniform contract and retry/circuit behavior per addendum. |
| A.5 LLM Providers | partial | `internal/llm/service.go` | Deterministic params + replay/fallback exist; provider/model/tier mappings, cost/rate accounting, schema re-validation and failover rules are incomplete. |
| A.6 Git HTTPS Remotes | partial | `internal/executor/git_https.go` | HTTPS-only host/size/retry policy helpers are implemented; full clone/ingestion execution pipeline remains partial. |
| B Content Firewall L1-L4 | complete | `internal/control/firewall_layers.go` | L1-L4 pipeline semantics are implemented with deterministic per-layer verdicts and quarantine tagging helpers. |
| C Load Shedding D0-D5 | complete | `internal/control/load_shedding_controller.go` | Trigger mapping plus timed escalation/recovery and manual D4/D5 handling are implemented. |
| D Semantic Verifiers SV-001..SV-007 | complete | `internal/control/semantic_verifiers.go` | SV-001 through SV-007 checks are implemented with deterministic pass/fail outputs. |
| E Plan Scoring U(plan) | complete | `internal/workflows/service.go` | Utility scoring with deterministic weights and tiebreak behavior is implemented. |
| F Rate Limits (concrete values) | complete | `internal/control/service.go` | Per-category and global rate-limit value maps with action semantics are implemented. |
| G Budget Defaults | complete | `internal/control/service.go` | Plan defaults and threshold helper logic are implemented for free/pro/business/enterprise. |
| H Memory Consolidation Rules | complete | `internal/memory/consolidation_rules_v93.go` | Duplicate thresholds, staleness/confidence supersede checks, and contradiction resolution helpers are implemented. |
| I Discovery Question Sets | partial | `internal/onboarding/service.go` | Stage scaffolding exists, but fixed 28-question set and exact mapping semantics are not aligned with addendum. |
| J Attention Budget | complete | `internal/context/service.go` | Tier budgets and deterministic attention constraints are implemented. |
| K Deterministic Jitter | complete | `internal/determinism/jitter.go` | Deterministic jitter helper is implemented per addendum formula. |
| L Missing Canonical Events | complete | `spec/events/canonical_events_v9.txt` | Addendum canonical events are present in the registry. |
| M Missing Table Additions | complete | `db/migrations/006_BREVIO_v93_addendum_specification_closure.sql` | `whatsapp_message_templates` table is added with RLS and indexes. |
| N Config Keys (Secrets + Env) | complete | `internal/config/registry.go` | Enumerated secret/env key registries are codified in runtime helpers. |
| O Autonomy A0-A4 Matrix | partial | `internal/control/service.go` | Basic A0-A4 decisions exist; upgrade-path/consent/history/admin requirements incomplete. |
| P Outbox Hold/Undo Windows | partial | `internal/workflows/service.go` | Hold worker exists but no autonomy/risk override durations and outbox delivery-state semantics. |
| Q Temporal Activity Retry Policies | complete | `internal/workflows/service.go` | Interactive/provisioning retry-policy matrices and common defaults are implemented. |
| R Context Assembly | complete | `internal/context/service.go` | 8-slot deterministic assembly and truncation ordering are implemented. |
| S Endpoint ↔ JSON Schema Mapping | complete | `api/openapi/v9.yaml`, `schemas/` | Addendum endpoint-to-schema mapping and required schema files are present. |
| T Workspace Routing Algorithm | complete | `internal/gateway/service.go` | Inbound routing includes binding lookup, default-workspace fallback auto-bind, and unbound rejection behavior. |
| U Recipient Verification | complete | `internal/control/service.go` | Recipient verification predicates and confirmation prompt behavior are implemented. |
| V Specialist Agents | complete | `internal/llm/specialists.go` | Specialist routing, explicit invocation, and tool filtering helpers are implemented. |
| W A2UI Canvas | partial | `internal/canvas/service.go` | WebSocket exists, but protocol message types/surfaces/security/rate-limit semantics incomplete. |
| X Voice Pipeline | partial | `internal/gateway/service.go` | Stub transcript behavior exists; no Whisper/Google STT fallback, confidence handling, or TTS pipeline. |
| Y Deterministic Ranking Formula | partial | `internal/provisioning/service.go` | Ranker exists but not the 6-factor formula/weights/tiebreak and replay key semantics from addendum. |
| Z Drift Watchdog Cadence | complete | `internal/workflows/service.go`, `internal/workflows/drift_watchdog_rules.go` | Cadence tables and quarantine/auto-heal rule helpers are implemented. |
| AA SSRF Deny CIDR List | partial | `internal/security/sandbox/service.go` | Partial block list present; full 14 CIDRs and explicit pre+post resolution behavior incomplete. |
| AB Write Budget `max_writes` | complete | `internal/control/service.go` | Tiered max-write thresholds are implemented. |
| AC Financial Two-Man Rule | complete | `internal/control/service.go` | Two-man threshold/TTL/second-approver logic is implemented. |
| AD Retention Policies | partial | `db/migrations/001_BREVIO_v9_init.sql` | Retention field exists but full policy catalog/defaults/expiry actions and eventing are incomplete. |
| AE PII Leakage Detection | complete | `internal/security/pii/leakage.go` | Fingerprint matching and false-positive exclusion helpers are implemented. |
| AF JWT Spec | complete | `internal/identity/jwt_signer.go` | RS256 issue/verify helpers and JWKS export are implemented for UserJWT/AdminJWT. |
| AG workspace_profiles 13 dimensions | partial | `internal/onboarding/service.go` | Values stored as map dimensions; not persisted as explicit 13 jsonb columns with version semantics in runtime model. |
| AH behavior policies 10 dimensions | partial | `internal/onboarding/service.go` | Similar partial mapping; no explicit 10 jsonb column model behavior in runtime layer. |
| AI routing_policies override | complete | `internal/llm/routing_policies.go` | Tier-specific then wildcard routing override resolution is implemented. |
| AJ tool_inventory vs connector_tools | complete | `internal/connectors/tool_resolution.go` | Planner/executor catalog separation and binding validation helpers are implemented. |
| AK domain_autonomy_json structure | complete | `internal/identity/workspace_policies.go` | Domain-autonomy normalization with required-key fallback to A0 is implemented. |
| AL allowed_connector_keys population | complete | `internal/identity/workspace_policies.go` | Provision/deprovision/admin-block lifecycle helpers are implemented. |
| AM financial_merchant_rules | complete | `internal/control/financial_rules.go` | Merchant-rule evaluation order and limit semantics are implemented. |
| AN financial_anomaly_events | complete | `internal/control/financial_rules.go` | Addendum anomaly-detection rules and elevated confirmation trigger helper are implemented. |
| AO Home Assistant/environment | complete | `internal/executor/home_assistant.go` | Supported action set, rate/refresh defaults, signal normalization, and proactive gating helpers are implemented. |
| AP airport_knowledge seed | partial | `db/migrations/001_BREVIO_v9_init.sql` | Table exists; required airport dataset seed and schema fields are not populated. |
| AQ Eval pass thresholds | complete | `internal/rag/eval/thresholds.go` | Deploy and governor threshold constants are codified. |
| AR Audit hash chain | partial | `internal/executor/service.go`, `db/migrations/001_BREVIO_v9_init.sql` | Hash chain exists but not addendum HMAC chain algorithm over defined fields with integrity job. |
| AS Internal mTLS cert mgmt | missing | `helm/`, `terraform/` | mTLS concept present in OpenAPI/infra tokens; no explicit cert-manager/ACM PCA rotation spec implementation. |
| AT Attachment pipeline | partial | `internal/gateway/service.go` | Upload reference exists; no strict MIME/magic/size validation and branch pipelines per addendum. |
| AU Document parse pipeline | partial | `db/migrations/001_BREVIO_v9_init.sql` | Parse result table exists; no format parser/OCR pipeline and context injection mechanics. |
| AV Delegation/pairing flow | partial | `internal/delegation/service.go` | Core invitation/grant present; full 12-step pairing with workspace/channel binding and event semantics incomplete. |
| AW Consent types/scopes/channels | complete | `internal/control/service.go` | Consent type/scope/proof-channel enumerations are implemented. |
| AX Memory write gate rules | partial | `internal/memory/service.go` | Policy gates exist; addendum-specific auto-approve/confirm matrix and semantic exclusion handling incomplete. |
| AY Workspace type differences | complete | `internal/identity/workspace_type_rules.go` | Workspace-type behavior filters and two-man activation predicates are implemented. |
| AZ Eval dataset + grader format | complete | `internal/rag/eval/graders.go` | Eval grader interface and required grader implementations are codified. |
| BA Interactive reply parser | complete | `internal/gateway/interactive_reply.go`, `internal/gateway/service.go` | Intent matrix and pending-option numeric selection resolution are implemented. |
| BB Executor cache L1/L2/L3 | partial | `internal/executor/service.go` | Three cache maps exist; TTL/scoping/invalidation rules only partially represented. |
| BC Auto-commit proof chain | partial | `db/migrations/001_BREVIO_v9_init.sql` | Table exists; chain algorithm/verification job and autonomy-consent linkage behavior missing. |
| BD channel_identity_envelopes | complete | `internal/gateway/service.go` | Identity envelope recording and failed-identity rejection event emission are implemented. |

## Recon Conclusion

The repository now has broad behavior-level closure coverage across deterministic algorithms, policy matrices, gateway parsing/routing rules, and security helpers. Remaining work is concentrated in deeper runtime/infrastructure integrations where helper-level closures are present but end-to-end external execution paths are still partial (notably A.1/A.2 outbound channel clients, A.4 connector runtime architecture, A.5 provider cost/rate accounting, AP airport bulk seed ingestion, AS cert-manager/ACM wiring, and full AT/AU binary/media execution pipelines).
