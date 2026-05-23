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
- [x] Audit Log — high-level lifecycle event log + kernel-touch events (Phase 2F.1); detail redacted via safe-logger; **exercised end-to-end by the integration harness** (30 entries per scenario run after Phase 3A; previously 25)
- [x] **Dispatch Table** (Phase 3A) — typed registry binding tool ids to executors, fail-closed on unknown tool / no executor / executor throw
- [x] **Internal Executors** (Phase 3A) — `auditWriteExecutor`, `feedbackWriteExecutor`, `memorySignalUpsertExecutor` wired via `wireInternalExecutors()`
- [x] **Gmail HTTP client** (Phase 3B.1) — read-only scope, injectable FetchLike, 401 → `GmailUnauthorizedError`, retryable 5xx/429 classification, base64url-aware message projection to `RawEmailContext`
- [x] **Gmail cursor store** (Phase 3B.1) — per-user Gmail history_id; in-memory + Postgres; new `gmail_cursors` Drizzle table (migration `0001_gmail_cursors.sql`)
- [x] **OAuth go-live routes** (Phase 3B.1) — `POST /oauth/google/start` (session-authenticated) + `GET /oauth/google/callback` (state-authenticated). PKCE + nonce single-use replay protection; state HMAC binds user_id; defense-in-depth nonce-row vs state user_id check
- [x] Safe Logger — sensitive-key redaction + token-shape regex
- [x] OAuth substrate — token-crypto (AES-256-GCM at-rest), oauth-state (HMAC+PKCE), exchange, token-store, provider registry (Google only)
- [x] Session HMAC — signed-session tokens + session-middleware
- [x] Tool Invocations — per-dispatch call log; privacy invariant tested (no payload columns)
- [x] Alert State Transitions store — persists state-machine moves; in-memory + Postgres
- [x] Drizzle/Neon persistence skeleton — 9 substrate tables, env-gated store factory, production fail-closed without `DATABASE_URL`
- [x] Postgres-backed stores — 7 stores, end-to-end verified against PGlite (Phase 2E.1)

### Honest semantics
- [x] Permission Gate denies `not_implemented` for ALL declared tools regardless of surface (Phase 2C.1 amendment)
- [x] Phase 3A flipped the three internal capabilities to `'implemented'` alongside their dispatch wiring in [`src/dispatch/internal-executors.ts`](src/dispatch/internal-executors.ts)
- [x] Three external tools (`gmail.read`, `sendblue.send_user_message`, `slack.founder_review`) remain `'declared'` until their adapters land (Phase 3B / 3E / 3D respectively per the revised Phase 3 map)
- [x] Phase 3B.1 adds two HTTP routes: `POST /oauth/google/start` (session-authenticated) and `GET /oauth/google/callback` (state-HMAC-authenticated). The Phase 2F invariant "no HTTP route beyond `GET /health`" was scoped to Phase 2; Phase 3 explicitly authorizes workflow HTTP surface per the revised map
- [x] Gmail read-only adapter exists (Phase 3B.1) but `gmail.read` tool stays `'declared'` — no executor wired until Phase 3B.2. The adapter is exercised once by the OAuth callback (to seed the gmail_cursors history_id) and is otherwise dormant
- [x] No SendBlue / Slack / real-model adapter exists yet (Phase 3D / 3E / 3C)
- [x] **Dispatch table cannot bypass the Permission Gate — structural guarantee** (Phase 3A.1). `dispatch.execute()` signature requires an `AuthorizedToolCall`, not a raw `tool_id`. The class has a private constructor; the only factory is `AuthorizedToolCall.fromDecision(decision)` which returns `null` unless `decision.allowed === true && decision.code === 'allowed' && isToolId(decision.tool_id)`. Runtime `instanceof` check in `execute()` rejects forged objects with code `'unauthorized'`
- [x] **`audit.write` dispatch never creates recursive audit logging** — executor calls `store.audit.write` directly; harness's `policy.decided` / `tool.invoked` audits go direct to store, not through dispatch

### Integration
- [x] [`apps/fomo/src/kernel/integration-harness.ts`](src/kernel/integration-harness.ts) exercises every kernel piece in one in-process scenario
- [x] [`apps/fomo/src/kernel/integration-harness.test.ts`](src/kernel/integration-harness.test.ts) asserts the substrate cooperates — fails loudly on any regression
- [x] `KernelScenarioReport` is deeply frozen — callers cannot mutate it
- [x] Two scenario runs are independent — each constructs its own in-memory stores
- [x] **Audit log participates in the integrated path** (Phase 2F.1) — harness writes a sanitized audit entry at every kernel touch. Gate test asserts entries > 0, per-action breakdown, and forbidden-content leak canary (no raw email body / headers / attachment filenames / prompt text / reply text in any audit entry).
- [x] **Dispatch table participates in the integrated path** (Phase 3A) — harness wires `wireInternalExecutors`, then routes the 5 internal invocations as `gate → AuthorizedToolCall.fromDecision → dispatch.execute → store write`. The 4 declared-external + unknown probes return `null` from the factory, so dispatch is structurally unreachable. The runtime `unauthorized` deny path is covered by dedicated tests in [`dispatch/dispatcher.test.ts`](src/dispatch/dispatcher.test.ts).

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
| Audit Log | [src/core/audit.ts](src/core/audit.ts) | [audit.test.ts](src/core/audit.test.ts) | ~4 | Integration Harness — writes 30 audit entries per scenario (Phase 3A added the audit.write executor + 'session.created' entry); dispatched via internal executor for `audit.write` tool | Phase 3 lifecycle callers (consent grants, OAuth connects, session events) write through `dispatch.execute('audit.write', ...)` |
| **Dispatch Table** (NEW Phase 3A, hardened 3A.1) | [src/dispatch/dispatcher.ts](src/dispatch/dispatcher.ts) | [dispatcher.test.ts](src/dispatch/dispatcher.test.ts) | ~22 | Integration Harness; internal executor wireup. `execute()` accepts only `AuthorizedToolCall` (class with private constructor, minted only by `fromDecision(allowedDecision)`). Runtime `instanceof` guard catches forged objects | Phase 3B/3D/3E register external executors here; flip those tools to 'implemented' as their adapters land |
| **Internal Executors** (NEW Phase 3A) | [src/dispatch/internal-executors.ts](src/dispatch/internal-executors.ts) | [internal-executors.test.ts](src/dispatch/internal-executors.test.ts) | ~10 | `wireInternalExecutors` registers all three at dispatch construction | Phase 3 callers (Gmail polling, ranker, Slack review, reply parser) invoke audit/feedback/memory via dispatch |
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
| Postgres stores | [src/db/stores/*-postgres.ts](src/db/stores/) | [stores/gated-pg.test.ts](src/db/stores/gated-pg.test.ts) | ~22 (gated, including gmail_cursors in 3B.1) | Store factory (when `DATABASE_URL` set) | Real Neon deploy uses these |
| **Gmail HTTP client** (NEW Phase 3B.1) | [src/adapters/gmail/client.ts](src/adapters/gmail/client.ts) | [adapters/gmail/client.test.ts](src/adapters/gmail/client.test.ts) | ~15 | OAuth callback (seeds cursor); Phase 3B.2 polling worker | `gmail.read` executor (Phase 3B.2) |
| **Gmail cursor store** (NEW Phase 3B.1) | [src/memory/gmail-cursors.ts](src/memory/gmail-cursors.ts) | [memory/gmail-cursors.test.ts](src/memory/gmail-cursors.test.ts) | ~9 (in-memory) + ~3 (gated PG) | OAuth callback (initializes); Phase 3B.2 polling worker (advances) | Phase 3B.2 reads + advances |
| **OAuth Google routes** (NEW Phase 3B.1) | [src/routes/oauth-google.ts](src/routes/oauth-google.ts) | [routes/oauth-google.test.ts](src/routes/oauth-google.test.ts) | ~10 | `apps/fomo/src/index.ts` (wired when GOOGLE_CLIENT_ID/SECRET/REDIRECT env set) | Phase 3B.2 polling worker reads tokens via TokenStore |

---

## Audit participation matrix (Phase 2F.1 + 3A)

The integration harness writes a sanitized audit entry at every kernel
touch. The gate test asserts both the count and that no payload content
leaks into any entry. Phase 3A added 5 entries: the dispatched internal
invocations produce additional `policy.decided` + `tool.invoked` entries
(now 9 each instead of 7), plus one `session.created` entry written by
the dispatched `audit.write` executor itself.

| Kernel touch | Audit action | Entries per scenario | Sanitized detail (no payload) |
|---|---|---|---|
| Gate decision | `policy.decided` | 9 | `tool_id`, `code`, `allowed` |
| Tool invocation write | `tool.invoked` | 9 | `tool_id`, `invocation_id`, `policy_decision`, `status` |
| Dispatched `audit.write` lifecycle event | `session.created` | 1 | `source` (executor calls `store.audit.write` directly — no recursive audit wrapper) |
| Alert state transition | `state.transitioned` | 6 | `alert_id`, `from_state`, `to_state` |
| Feedback write | `feedback.written` | 2 | `kind`, `alert_id`, `sender_present` (boolean, not the email) |
| Memory upsert | `memory.upserted` | 2 | `kind`, `scope_present` (boolean, not the scope_key), `source` |
| Model route | `model.routed` | 1 | `capability`, `model_name`, `prompt_version`, `schema_valid` (NOT prompt text or output text) |
| **Total** | — | **30** | |

**No-recursive-audit invariant (Phase 3A guardrail):** the dispatched
`audit.write` executor calls `store.audit.write` DIRECTLY. The harness's
`policy.decided` and `tool.invoked` audits around the dispatch call also
go direct to `store.audit.write`, never through `dispatch.execute('audit.write')`.
One `audit.write` dispatch produces exactly three audit entries
(`policy.decided` + `session.created` + `tool.invoked`), never more.

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

## v0.1 Tool Registry status (Phase 3A)

Three internal capabilities flipped to `'implemented'` in Phase 3A. The
three external tools remain `'declared'` and the Permission Gate continues
to deny them with `not_implemented`.

| Tool ID | Surface | Risk tier | Executor status | Wired in | Remaining Phase 3 wiring |
|---|---|---|---|---|---|
| `gmail.read` | external | read | **declared** | — | Adapter + OAuth + cursor store landed in 3B.1; executor wiring + flip lands in **Phase 3B.2** |
| `sendblue.send_user_message` | external | send | **declared** | — | **Phase 3E** SendBlue HTTP adapter + send path |
| `slack.founder_review` | internal | send | **declared** | — | **Phase 3D** Slack Block-Kit card poster |
| `audit.write` | internal | internal | **implemented** | Phase 3A | `auditWriteExecutor` writes to AuditStore |
| `feedback.write` | internal | internal | **implemented** | Phase 3A | `feedbackWriteExecutor` writes to FeedbackStore |
| `memory_signal.write` | internal | internal | **implemented** | Phase 3A | `memorySignalUpsertExecutor` upserts MemorySignalStore |

For the three implemented tools, the gate consults policy normally (kill
switches, consent, OAuth) and on allow, dispatch routes to the executor.
For the three still-declared tools, the gate denies `not_implemented`
before any dispatch is attempted. This is the honest invariant the
integration harness asserts.

---

## Phase 3 map (revised)

| Subphase | Scope |
|---|---|
| **3A** | Internal Dispatch — dispatch table + 3 internal executors + flip those 3 tools to `implemented`. **(done)** |
| **3B.1** | Gmail HTTP client + OAuth go-live routes + `gmail_cursors` table + cursor store (in-memory + Postgres). `gmail.read` stays declared. **(done)** |
| 3B.2 | Gmail polling worker + `gmail.read` executor wiring + flip `gmail.read` to `implemented`. Polling kill-switch `FOMO_GMAIL_POLLING_ENABLED` default false |
| 3C | Ranker + Real Model Backends — OpenAI/Anthropic via existing router + ranker prompt + real fixtures |
| 3D | Slack Founder Review Only — Slack adapter + founder-review path; flip `slack.founder_review` to `implemented`. **No live user-facing texts yet.** |
| 3E | SendBlue Founder-Only Outbound — SendBlue HTTP client + first live send (founder-only); flip `sendblue.send_user_message` to `implemented` |
| 3F | SendBlue Inbound Reply Parser — webhook + reply classification + snooze scheduler + memory/feedback updates from replies |
| 3G | Full Founder Demo Gate — end-to-end scenario proven; demo ready |

Slack review (3D) and SendBlue sends (3E) are intentionally separate
subphases so founder review is proven before any live text goes out.

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
- Bypass the Permission Gate from within dispatch executors (executors receive args + context, never an AuthorizedToolCall or the dispatch table itself)
- Cause recursive audit logging from inside an executor
- Mint an `AuthorizedToolCall` by any path other than `AuthorizedToolCall.fromDecision(decision)` — the private constructor is the structural lock; bypassing it via type-cast triggers the runtime `unauthorized` deny

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
