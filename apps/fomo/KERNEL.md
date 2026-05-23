# Brevio FOMO — Kernel Completeness Gate (Phase 2F)

This file is the **load-bearing artifact** the founder ticks off to approve
Phase 3. If every checklist row below is `[x]` and the integration harness
test passes, the minimal MCP OS kernel is complete and Phase 3 (the FOMO
workflow) may begin. Not before.

> **Founder gate:** Phase 3 may begin only when (a) every box in the
> [Completeness checklist](#completeness-checklist) is checked, (b)
> `pnpm test` is green, and (c) the founder signs off in the Phase 2F PR.

---

## Completeness checklist

### Substrate present
- [x] Tool Registry — 6 v0.1 tools declared, `surface` (external/internal) load-bearing
- [x] Permission Gate — fail-closed; honest semantics (`declared` → `not_implemented` regardless of surface)
- [x] Kill Switches — env-driven, strict opt-in, all defaults safe
- [x] Egress Policy — three views, payload-shaped fields never leak
- [x] Alert State Machine — 12 states, exhaustive transition graph, terminal accounting
- [x] Feedback Events — 11 event kinds, append-only, detail redacted via safe-logger
- [x] Memory Signals — 6 signal kinds, upsert identity (user_id, kind, scope_key), provenance + confidence mandatory
- [x] Model Router — `classification` only, validator-function based, fail-closed
- [x] Cost Tracking — per-call records (model, prompt_version, tokens, latency, USD, schema_valid)
- [x] Mock Model Backend — deterministic prompt→response map, no network (asserted by tripwire test)
- [x] Eval Harness — pure-function binary precision/recall (no real fixtures yet)
- [x] Audit Log — high-level lifecycle event log + kernel-touch events (Phase 2F.1); detail redacted via safe-logger; **exercised end-to-end by the integration harness** (25 entries per scenario run, asserted)
- [x] Safe Logger — sensitive-key redaction + token-shape regex
- [x] OAuth substrate — token-crypto (AES-256-GCM at-rest), oauth-state (HMAC+PKCE), exchange, token-store, provider registry (Google only)
- [x] Session HMAC — signed-session tokens + session-middleware
- [x] Tool Invocations — per-dispatch call log; privacy invariant tested (no payload columns)
- [x] Alert State Transitions store — persists state-machine moves; in-memory + Postgres
- [x] Drizzle/Neon persistence skeleton — 9 substrate tables, env-gated store factory, production fail-closed without `DATABASE_URL`
- [x] Postgres-backed stores — 7 stores, end-to-end verified against PGlite (Phase 2E.1)

### Honest semantics
- [x] Every v0.1 tool ships `executor_status: 'declared'` (verified by `tool-registry.test.ts` and by the gate scenario)
- [x] Permission Gate denies `not_implemented` for ALL declared tools regardless of surface (Phase 2C.1 amendment)
- [x] No tool was flipped to `'implemented'` during Phase 2
- [x] No HTTP route was added beyond `GET /health`
- [x] No Gmail / SendBlue / Slack / model adapter exists yet (Phase 3)

### Integration
- [x] [`apps/fomo/src/kernel/integration-harness.ts`](src/kernel/integration-harness.ts) exercises every kernel piece in one in-process scenario
- [x] [`apps/fomo/src/kernel/integration-harness.test.ts`](src/kernel/integration-harness.test.ts) asserts the substrate cooperates — fails loudly on any regression
- [x] `KernelScenarioReport` is deeply frozen — callers cannot mutate it
- [x] Two scenario runs are independent — each constructs its own in-memory stores
- [x] **Audit log participates in the integrated path** (Phase 2F.1) — harness writes a sanitized audit entry at every kernel touch: gate decisions, tool invocations, state transitions, feedback writes, memory upserts, and model routes. Gate test asserts entries > 0, per-action breakdown, and forbidden-content leak canary (no raw email body / headers / attachment filenames / prompt text / reply text in any audit entry).

### CI + safety
- [x] `pnpm build` green
- [x] `pnpm test` green (default: in-memory stores only)
- [x] `pnpm test` green with `BREVIO_RUN_PG_TESTS=true` (Phase 2E.1 gated suite)
- [x] `pnpm lint` green (zero violations across all .ts files)
- [x] No live network calls in CI (asserted by `model-backends/mock.test.ts` tripwire)
- [x] No live DB connection in CI

### Founder sign-off
- [ ] Founder has reviewed this file and confirmed Phase 3 may begin

---

## Substrate inventory

Each piece lists: source file, primary test file, test count, current callers,
and what Phase 3 must wire to use it.

| Piece | Source | Tests | Cases | Current callers | Phase 3 wires |
|---|---|---|---|---|---|
| Tool Registry | [src/core/tool-registry.ts](src/core/tool-registry.ts) | [tool-registry.test.ts](src/core/tool-registry.test.ts) | ~19 | Permission Gate, Integration Harness | Dispatch table reads from here |
| Permission Gate | [src/core/policy-gate.ts](src/core/policy-gate.ts) | [policy-gate.test.ts](src/core/policy-gate.test.ts) | ~25 | Integration Harness | Workflow steps consult before each tool invocation |
| Kill Switches | [src/core/kill-switches.ts](src/core/kill-switches.ts) | [kill-switches.test.ts](src/core/kill-switches.test.ts) | ~26 | Permission Gate, Integration Harness | Gmail polling + send paths read |
| Egress Policy | [src/core/egress-policy.ts](src/core/egress-policy.ts) | [egress-policy.test.ts](src/core/egress-policy.test.ts) | ~25 | Integration Harness | Ranker prompt + Slack card + reply parser go through |
| Alert State Machine | [src/core/state-machine.ts](src/core/state-machine.ts) | [state-machine.test.ts](src/core/state-machine.test.ts) | ~100 (parameterized) | Integration Harness | Every alert workflow step writes a transition |
| Alert State Transitions store | [src/core/alert-state-transitions.ts](src/core/alert-state-transitions.ts) | [alert-state-transitions.test.ts](src/core/alert-state-transitions.test.ts) | ~12 | Integration Harness | State-machine writes here on every transition |
| Feedback Events | [src/memory/feedback-events.ts](src/memory/feedback-events.ts) | [feedback-events.test.ts](src/memory/feedback-events.test.ts) | ~22 | Integration Harness | Founder review + reply parser write here |
| Memory Signals | [src/memory/memory-signals.ts](src/memory/memory-signals.ts) | [memory-signals.test.ts](src/memory/memory-signals.test.ts) | ~20 | Integration Harness | Background derivation jobs + user confirmation flows upsert here |
| Model Router | [src/core/model-router.ts](src/core/model-router.ts) | [model-router.test.ts](src/core/model-router.test.ts) | ~15 | Integration Harness | Ranker + reply parser call `.route('classification', …)` |
| Cost Tracking | [src/core/cost-tracking.ts](src/core/cost-tracking.ts) | [cost-tracking.test.ts](src/core/cost-tracking.test.ts) | ~15 | Model Router, Integration Harness | Auto-written by Model Router on every backend call |
| Mock Model Backend | [src/core/model-backends/mock.ts](src/core/model-backends/mock.ts) | [model-backends/mock.test.ts](src/core/model-backends/mock.test.ts) | ~10 | Integration Harness, Eval Harness | Tests + dev. Real OpenAI / Anthropic backends are Phase 3 deps |
| Eval Harness | [src/eval/harness.ts](src/eval/harness.ts) | [eval/harness.test.ts](src/eval/harness.test.ts) | ~11 | (developer tool) | Bake-off against real fixtures in Phase 3 |
| Tool Invocations | [src/core/tool-invocations.ts](src/core/tool-invocations.ts) | [tool-invocations.test.ts](src/core/tool-invocations.test.ts) | ~17 | Integration Harness | Dispatch layer writes per-call after gate decision |
| Audit Log | [src/core/audit.ts](src/core/audit.ts) | [audit.test.ts](src/core/audit.test.ts) | ~4 | Integration Harness (Phase 2F.1) — writes 25 audit entries per scenario | Phase 3 lifecycle callers (consent grants, OAuth connects, session events) layer on top of the kernel-touch events already wired |
| Safe Logger | [src/core/safe-logger.ts](src/core/safe-logger.ts) | [safe-logger.test.ts](src/core/safe-logger.test.ts) | ~6 | Audit, Feedback, Memory, Tool Invocations, Postgres stores | All log lines + redacted store details |
| Token Crypto | [src/security/token-crypto.ts](src/security/token-crypto.ts) | [token-crypto.test.ts](src/security/token-crypto.test.ts) | ~9 | Token Store | OAuth token at-rest encryption |
| Session HMAC | [src/security/session.ts](src/security/session.ts) | [session.test.ts](src/security/session.test.ts) | ~13 | (no v0.1 caller yet) | Founder/admin session auth in Phase 3 |
| Session middleware | [src/security/session-middleware.ts](src/security/session-middleware.ts) | (covered by session tests) | — | (no v0.1 caller yet) | Wrapping admin routes in Phase 3 |
| OAuth state | [src/security/oauth/state.ts](src/security/oauth/state.ts) | [oauth/state.test.ts](src/security/oauth/state.test.ts) | ~11 | (no v0.1 caller yet) | Google OAuth start route in Phase 3 |
| OAuth exchange | [src/security/oauth/exchange.ts](src/security/oauth/exchange.ts) | [oauth/exchange.test.ts](src/security/oauth/exchange.test.ts) | ~8 | (no v0.1 caller yet) | Google OAuth callback route in Phase 3 |
| Token Store | [src/security/oauth/token-store.ts](src/security/oauth/token-store.ts) | [oauth/token-store.test.ts](src/security/oauth/token-store.test.ts) | ~6 | Store factory, Integration Harness | OAuth callback writes here |
| Provider registry | [src/security/oauth/providers/index.ts](src/security/oauth/providers/index.ts) | (covered by exchange tests) | — | OAuth exchange | Google config (v0.1) |
| Drizzle schema | [src/db/schema.ts](src/db/schema.ts) | (covered by gated PG tests) | — | All Postgres stores | Schema migrations applied to Neon in Phase 3 deploy |
| Drizzle client | [src/db/client.ts](src/db/client.ts) | [db/client.test.ts](src/db/client.test.ts) | ~12 | Store factory | `DATABASE_URL` env var |
| Store factory | [src/db/store-factory.ts](src/db/store-factory.ts) | [db/store-factory.test.ts](src/db/store-factory.test.ts) | ~7 | Integration Harness | Phase 3 callers receive `SubstrateStores` from here |
| Postgres stores | [src/db/stores/*-postgres.ts](src/db/stores/) | [stores/gated-pg.test.ts](src/db/stores/gated-pg.test.ts) | ~19 (gated) | Store factory (when `DATABASE_URL` set) | Real Neon deploy uses these |

---

## Audit participation matrix (Phase 2F.1)

The integration harness writes a sanitized audit entry at every kernel
touch. The gate test asserts both the count and that no payload content
leaks into any entry.

| Kernel touch | Audit action | Entries per scenario | Sanitized detail (no payload) |
|---|---|---|---|
| Gate decision | `policy.decided` | 7 | `tool_id`, `code`, `allowed` |
| Tool invocation write | `tool.invoked` | 7 | `tool_id`, `invocation_id`, `policy_decision`, `status` |
| Alert state transition | `state.transitioned` | 6 | `alert_id`, `from_state`, `to_state` |
| Feedback write | `feedback.written` | 2 | `kind`, `alert_id`, `sender_present` (boolean, not the email) |
| Memory upsert | `memory.upserted` | 2 | `kind`, `scope_present` (boolean, not the scope_key), `source` |
| Model route | `model.routed` | 1 | `capability`, `model_name`, `prompt_version`, `schema_valid` (NOT prompt text or output text) |
| **Total** | — | **25** | |

Forbidden in any audit entry — asserted by the harness's leak-canary
test (the harness intentionally passes recognizable canary strings as
the prompt, reply text, and email body so any leak surfaces here):

- raw email body (`body_plain`)
- HTML email body (`body_html`)
- raw headers (any header name or value)
- attachment filenames
- prompt text passed to the model router
- full user reply text
- known leak canary strings injected by the harness for the test

The new audit action types (`policy.decided`, `tool.invoked`,
`state.transitioned`, `feedback.written`, `memory.upserted`,
`model.routed`) are legitimate Phase 3 audit categories. Phase 3 callers
should continue writing them when the real dispatch wiring lands.

## v0.1 Tool Registry status

All six tools are `executor_status: 'declared'` and remain so until Phase 3
wires their respective dispatch.

| Tool ID | Surface | Risk tier | Executor status | Phase 3 wiring |
|---|---|---|---|---|
| `gmail.read` | external | read | **declared** | Gmail watch worker reads via this dispatch |
| `sendblue.send_user_message` | external | send | **declared** | SendBlue HTTP adapter + send path |
| `slack.founder_review` | internal | send | **declared** | Slack Block-Kit card poster |
| `audit.write` | internal | internal | **declared** | Dispatch wires to `InMemoryAuditStore` (Phase 3) or `PostgresAuditStore` |
| `feedback.write` | internal | internal | **declared** | Dispatch wires to `FeedbackStore` |
| `memory_signal.write` | internal | internal | **declared** | Dispatch wires to `MemorySignalStore` |

Until Phase 3 adds the dispatch table and flips status to `implemented`,
the Permission Gate denies every tool with code `not_implemented`. This is
the honest invariant the integration harness test asserts.

---

## What Phase 3 must NOT do without a Phase 2-style amendment

- Add a workflow table (`alerts`, `message_events`, `rank_results`,
  `gmail_cursors`, `replies`, `sender_importance`, `suppressions`,
  `user_preferences`) without an accompanying caller landing in the same PR
- Flip a tool's `executor_status` to `'implemented'` without a real dispatch
  binding the tool id to a real executor
- Add a model backend without paired cost-tracking + schema validation
- Add a new HTTP route without a Permission Gate consultation upstream of any
  dispatch
- Take a runtime dep on a service the Phase 2 map deferred (pgvector, Redis,
  R2) without an active caller

---

## Run the gate

```bash
# Full kernel test surface (default, in-memory stores only)
pnpm build
pnpm test
pnpm lint

# Plus end-to-end Postgres verification against PGlite
BREVIO_RUN_PG_TESTS=true pnpm test
```

When all four are green and every checklist row above is checked, the
substrate is ready. Founder signs the PR; Phase 3 begins.
