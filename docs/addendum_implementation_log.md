# BREVIO V9.3 Addendum Implementation Log

Date: 2026-03-01

## Completed in this cycle

- Added Phase 0 gap audit output: `docs/addendum_gap_audit.md`.
- Added V9.3 migration `006_BREVIO_v93_addendum_specification_closure.sql`:
  - creates `whatsapp_message_templates`
  - enables RLS + workspace policy
  - adds supporting indexes
- Added 19 addendum schema files and mapped them in OpenAPI:
  - webhook payloads, user ledger/evidence, provisioning status/start, catalog search
  - review-task endpoints, brain/control/hands internal contracts
  - outbound send, canvas push, forensic replay, budgeting requests/responses
- Expanded control-plane mux coverage for addendum-owned endpoints:
  - `/v1/webhooks/*`, `/v1/user/*`, `/v1/provision/*`, `/v1/catalog/search`
  - `/v1/workspaces/{id}/provisioning/*`
  - `/v1/brain/turn`, `/v1/control/plan/evaluate`, `/v1/hands/tool/execute`
  - addendum admin routes for replay/review/artifact paths
- Implemented deterministic jitter helper:
  - `sha256(workspace_id || job_name) mod 51`
  - files: `internal/determinism/jitter.go`, `internal/determinism/jitter_test.go`
- Implemented plan scoring utility + deterministic tie-breaks in workflows.
- Implemented context assembly slot model and attention budget constants.
- Implemented interactive reply parser intent mapping enhancements.
- Added concrete default rate limits, global limits, and plan budgets in control layer.
- Added LLM tier model mapping helpers in `internal/llm`.
- Added addendum canonical events to `spec/events/canonical_events_v9.txt`.
- Expanded SSRF deny CIDR coverage and rebinding-aware checks in executor/sandbox layers.
- Regenerated API docs: `docs/API_REFERENCE.md`.

## Contract and test closures updated

- Updated closure tests for:
  - addendum schema presence
  - OpenAPI endpoint/schema parity and ownership
  - migration order/UUID/runtime verification scripts
  - canonical event registry expectations
- Added canvas owner route for `POST /v1/canvas/push` in `internal/canvas/service.go`.
- Added onboarding compatibility defaults to preserve legacy acceptance fixtures while enforcing new V9.3 question keys.

## Validation

- Targeted suites passed:
  - `./scripts/dev/go_exec.sh test ./internal/context ./internal/control ./internal/gateway ./internal/llm ./internal/security/sandbox ./internal/executor ./internal/onboarding ./internal/workflows ./internal/contracts ./internal/determinism ./internal/canvas -count=1`
- Full repository pass:
  - `./scripts/dev/go_exec.sh test ./... -count=1`

## Additional addendum closures (second pass)

- Control-layer addendum policy helpers added and tested:
  - effective autonomy MIN(global/domain/cap)
  - autonomy upgrade path rules (A0->A4)
  - outbox hold-window overrides (A2/A3/A4 + elevated/critical)
  - write budgets per tier (T1/T2/T3)
  - recipient verification prompt/logic
  - financial approval + professional two-man requirement
  - consent type/scope/proof-channel enumerations
- Temporal/workflow addendum policy helpers added and tested:
  - interactive/provisioning activity retry policies with exact timeout/attempt/non-retryable values
  - common retry defaults
  - drift watchdog cadences (5m/1h/24h)
  - delegation pairing 12-step flow list
- Provisioning ranking addendum formula added and tested:
  - 6-factor scoring
  - default weight set
  - deterministic tie-break by lexical `server_id` within 0.001 score delta
- Connector OAuth registry addendum values added and tested:
  - 6 provider registry definitions
  - per-connector additional scope map
- Retention policy defaults added and tested:
  - RP-001..RP-006 map
  - data-class default mapping
- Canvas/Voice addendum constants added and tested:
  - Canvas protocol message/surface enumerations
  - Voice STT/TTS defaults and thresholds
- Gateway workspace routing helper added and tested:
  - fallback default-workspace auto-binding behavior for inbound messages
- Security chain helpers added and tested:
  - HMAC-SHA256 audit chain computation
  - HMAC-SHA256 auto-commit proof chain computation
