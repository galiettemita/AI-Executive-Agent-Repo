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
| A.3 OAuth Providers | partial | `internal/connectors/service.go` | Token envelope and refresh metadata exist; no full provider registry, PKCE/state HMAC flow, callback/replay protection, revocation integrations. |
| A.4 Connector APIs (40+) | partial | `internal/connectors/seeds/connectors.yaml` | Seed inventory exists but differs from launch-set semantics; no complete native/MCP client architecture with uniform contract and retry/circuit behavior per addendum. |
| A.5 LLM Providers | partial | `internal/llm/service.go` | Deterministic params + replay/fallback exist; provider/model/tier mappings, cost/rate accounting, schema re-validation and failover rules are incomplete. |
| A.6 Git HTTPS Remotes | missing | N/A | No dedicated HTTPS clone/ingestion pipeline with size limit, SSRF URL validation, host/token strategy, and on-demand refresh behavior. |
| B Content Firewall L1-L4 | partial | `internal/control/service.go` | Basic pattern firewall exists; full L1-L4 pipeline semantics and quarantine workflow/eventing not implemented. |
| C Load Shedding D0-D5 | partial | `internal/control/service.go` | Tier gate function exists; no trigger-based escalation/recovery scheduler and persistence semantics from addendum. |
| D Semantic Verifiers SV-001..SV-007 | partial | `internal/control/service.go` | Placeholder output checks exist; concrete verifier set and per-tool/domain logic are not fully implemented. |
| E Plan Scoring U(plan) | missing | N/A | No utility-scoring implementation with weighted factors, tie-break logic, and deterministic selection as specified. |
| F Rate Limits (concrete values) | partial | `internal/control/service.go`, `internal/gateway/service.go` | Generic rate limits exist; addendum per-category/per-user limits and action mapping not fully implemented. |
| G Budget Defaults | partial | `internal/control/service.go` | Basic budget cap exists; plan defaults and warning/exhaustion events/threshold logic are incomplete. |
| H Memory Consolidation Rules | partial | `internal/memory/service.go` | Duplicate/stale behavior exists at simplified level; cosine thresholds, contradiction logic, 90-day policy and 10k-item behavior missing. |
| I Discovery Question Sets | partial | `internal/onboarding/service.go` | Stage scaffolding exists, but fixed 28-question set and exact mapping semantics are not aligned with addendum. |
| J Attention Budget | missing | N/A | No per-tier cumulative token/call enforcement across turn reasoning loop. |
| K Deterministic Jitter | missing | N/A | No `sha256(workspace_id||job_name) mod 51` helper wired to scheduled jobs. |
| L Missing Canonical Events | missing | `spec/events/canonical_events_v9.txt` | The eight addendum events are absent from canonical V9 event registry. |
| M Missing Table Additions | missing | `db/migrations/001_BREVIO_v9_init.sql` | `whatsapp_message_templates` table not present. |
| N Config Keys (Secrets + Env) | partial | `docs/DEPLOYMENT.md`, `internal/connectors/aws_key_provider.go` | Some config structure exists; complete enumerated key registry is not codified. |
| O Autonomy A0-A4 Matrix | partial | `internal/control/service.go` | Basic A0-A4 decisions exist; upgrade-path/consent/history/admin requirements incomplete. |
| P Outbox Hold/Undo Windows | partial | `internal/workflows/service.go` | Hold worker exists but no autonomy/risk override durations and outbox delivery-state semantics. |
| Q Temporal Activity Retry Policies | missing | `internal/workflows/service.go` | Workflow stubs exist; no per-activity timeout/retry/non-retryable policy model matching addendum tables. |
| R Context Assembly | missing | `internal/context/service.go` | Budget allocator exists; no 8-slot deterministic assembly/truncation order implementation. |
| S Endpoint ↔ JSON Schema Mapping | missing | `api/openapi/v9.yaml`, `schemas/` | Several required `new` schemas absent and endpoint mapping not aligned with addendum list. |
| T Workspace Routing Algorithm | partial | `internal/gateway/service.go`, `internal/identity/service.go` | Basic channel binding lookup exists; fallback bind/create and unbound-channel handling behavior incomplete. |
| U Recipient Verification | partial | `internal/control/service.go` | Gate hook exists, but full verification conditions (contacts/recent convo/allowlist/domain) are not implemented. |
| V Specialist Agents | partial | `db/migrations/001_BREVIO_v9_init.sql` | Table exists; routing/execution constraints and prompt swap mechanics are missing in runtime. |
| W A2UI Canvas | partial | `internal/canvas/service.go` | WebSocket exists, but protocol message types/surfaces/security/rate-limit semantics incomplete. |
| X Voice Pipeline | partial | `internal/gateway/service.go` | Stub transcript behavior exists; no Whisper/Google STT fallback, confidence handling, or TTS pipeline. |
| Y Deterministic Ranking Formula | partial | `internal/provisioning/service.go` | Ranker exists but not the 6-factor formula/weights/tiebreak and replay key semantics from addendum. |
| Z Drift Watchdog Cadence | missing | `internal/workflows/service.go` | Drift status stub exists; no 5m/1h/24h cadence with quarantine and auto-heal behavior. |
| AA SSRF Deny CIDR List | partial | `internal/security/sandbox/service.go` | Partial block list present; full 14 CIDRs and explicit pre+post resolution behavior incomplete. |
| AB Write Budget `max_writes` | missing | N/A | No tiered max-write gating/splitting behavior implemented. |
| AC Financial Two-Man Rule | missing | N/A | No threshold logic or second-approver enforcement for professional workspaces. |
| AD Retention Policies | partial | `db/migrations/001_BREVIO_v9_init.sql` | Retention field exists but full policy catalog/defaults/expiry actions and eventing are incomplete. |
| AE PII Leakage Detection | missing | `internal/security/pii/service.go` | Encryption exists; cross-user fingerprint leakage detection workflow is absent. |
| AF JWT Spec | partial | `api/openapi/v9.yaml` | JWT schemes listed in OpenAPI; concrete UserJWT/AdminJWT RS256/JWKS issuance+validation implementation not present in runtime package. |
| AG workspace_profiles 13 dimensions | partial | `internal/onboarding/service.go` | Values stored as map dimensions; not persisted as explicit 13 jsonb columns with version semantics in runtime model. |
| AH behavior policies 10 dimensions | partial | `internal/onboarding/service.go` | Similar partial mapping; no explicit 10 jsonb column model behavior in runtime layer. |
| AI routing_policies override | partial | `db/migrations/001_BREVIO_v9_init.sql` | Table exists; addendum-specific override resolution logic not implemented in LLM routing path. |
| AJ tool_inventory vs connector_tools | partial | `db/migrations/001_BREVIO_v9_init.sql` | Both tables exist; separation semantics are not explicitly enforced in planner/executor behavior. |
| AK domain_autonomy_json structure | partial | `internal/identity/service.go` | Defaults align with domains; missing-key fallback enforcement and full update lifecycle need hardening. |
| AL allowed_connector_keys population | partial | `internal/identity/service.go` | Field exists; population/deprovision/admin block lifecycle behavior not fully implemented. |
| AM financial_merchant_rules | partial | `db/migrations/001_BREVIO_v9_init.sql` | Table exists with generic JSON field; concrete rule schema and evaluation order are missing. |
| AN financial_anomaly_events | partial | `db/migrations/001_BREVIO_v9_init.sql` | Table exists with generic JSON field; detection algorithm and next-hour enforcement logic missing. |
| AO Home Assistant/environment | partial | `db/migrations/001_BREVIO_v9_init.sql` | Tables exist; full HA integration and proactive rule behavior not implemented. |
| AP airport_knowledge seed | partial | `db/migrations/001_BREVIO_v9_init.sql` | Table exists; required airport dataset seed and schema fields are not populated. |
| AQ Eval pass thresholds | missing | `evals/` | Dataset scaffolding exists; deploy/governor threshold enforcement values are not codified. |
| AR Audit hash chain | partial | `internal/executor/service.go`, `db/migrations/001_BREVIO_v9_init.sql` | Hash chain exists but not addendum HMAC chain algorithm over defined fields with integrity job. |
| AS Internal mTLS cert mgmt | missing | `helm/`, `terraform/` | mTLS concept present in OpenAPI/infra tokens; no explicit cert-manager/ACM PCA rotation spec implementation. |
| AT Attachment pipeline | partial | `internal/gateway/service.go` | Upload reference exists; no strict MIME/magic/size validation and branch pipelines per addendum. |
| AU Document parse pipeline | partial | `db/migrations/001_BREVIO_v9_init.sql` | Parse result table exists; no format parser/OCR pipeline and context injection mechanics. |
| AV Delegation/pairing flow | partial | `internal/delegation/service.go` | Core invitation/grant present; full 12-step pairing with workspace/channel binding and event semantics incomplete. |
| AW Consent types/scopes/channels | missing | `db/migrations/001_BREVIO_v9_init.sql` | Consent table exists; no explicit type/scope/proof enumerations and version bump handling. |
| AX Memory write gate rules | partial | `internal/memory/service.go` | Policy gates exist; addendum-specific auto-approve/confirm matrix and semantic exclusion handling incomplete. |
| AY Workspace type differences | partial | `db/migrations/002_BREVIO_v91_soft_intelligence.sql` | Workspace type exists; differential behavior enforcement across personal/professional/delegation is incomplete. |
| AZ Eval dataset + grader format | partial | `evals/`, `internal/rag/eval` | Eval scaffolding exists; exact dataset/grader interfaces and required implementations are incomplete. |
| BA Interactive reply parser | partial | `internal/gateway/service.go` | Parsing exists for button/list payloads; canonical intent parser matrix and pending-action resolution behavior missing. |
| BB Executor cache L1/L2/L3 | partial | `internal/executor/service.go` | Three cache maps exist; TTL/scoping/invalidation rules only partially represented. |
| BC Auto-commit proof chain | partial | `db/migrations/001_BREVIO_v9_init.sql` | Table exists; chain algorithm/verification job and autonomy-consent linkage behavior missing. |
| BD channel_identity_envelopes | partial | `db/migrations/001_BREVIO_v9_init.sql`, `internal/gateway/service.go` | Envelope table exists; end-to-end verification flow and failed identity rejection logging are incomplete. |

## Recon Conclusion

The repository already contains broad V9/V9.1/V9.2 scaffolding, but the majority of V9.3 addendum items are currently `partial` or `missing` in behavior-level detail. The immediate implementation priority is:

1. Schema/API/event/config exactness (Sections S, L, N, M)
2. Security-hardening concretes (Sections AA, AR, BD, AE)
3. Deterministic algorithm closures (Sections E, J, K, R, Y, Q)
4. Control/gateway behavior precision (Sections B, C, D, F, G, O, P, BA, AT)
