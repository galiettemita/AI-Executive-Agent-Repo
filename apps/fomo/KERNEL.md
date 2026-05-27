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
- [x] **Gmail cursor store** (Phase 3B.1) — per-user Gmail history_id; in-memory + Postgres; new `gmail_cursors` Drizzle table (migration `0001_gmail_cursors.sql`); `listUserIds()` added in 3B.2 as canonical "Gmail-connected users" source for the polling worker
- [x] **OAuth go-live routes** (Phase 3B.1) — `POST /oauth/google/start` (session-authenticated) + `GET /oauth/google/callback` (state-authenticated). PKCE + nonce single-use replay protection; state HMAC binds user_id; defense-in-depth nonce-row vs state user_id check
- [x] **gmail.read executor** (Phase 3B.2) — `gmailReadExecutor` shim over `GmailClient.getMessage` keyed by `{ message_id }`; loads access token via `TokenStore.loadAccessToken`; on 401 marks `needs_reauth` and re-throws; wired through `wireExternalExecutors`
- [x] **Gmail polling worker** (Phase 3B.2) — `runOnce(deps)` cycle: per user, calls `listHistorySince`, dispatches `gmail.read` for each new message id, advances cursor, writes per-message audit + aggregate `gmail.poll.cycle` entry; `startPolling(deps, opts)` interval wrapper; founder-gated via `FOMO_GMAIL_POLLING_ENABLED` (default false)
- [x] **Anthropic model backend** (Phase 3C.1, dormant) — `AnthropicBackend` implements `ModelBackend`; direct HTTP to `api.anthropic.com/v1/messages`, injectable `FetchLike`, 401/403 → `AnthropicAuthError`, retryable 429/5xx/529 → `AnthropicApiError`; `claude-haiku-4-5-20251001` + `claude-sonnet-4-6` priced in `MODEL_PRICING`. **Founder directive 2026-05-24 made OpenAI the initial Brevio ranker; AnthropicBackend stays in main as future-provider support but is NOT the 3C.2 decision path.**
- [x] **OpenAI model backend** (Phase 3C.2) — `OpenAIBackend` implements `ModelBackend`; direct HTTP to `api.openai.com/v1/chat/completions`, injectable `FetchLike`, optional strict `response_format: json_schema` for server-side structured-output enforcement, 401/403 → `OpenAIAuthError`, retryable 429/5xx → `OpenAIApiError`, model refusal surfaces as `OpenAIApiError(model_refusal)`. `gpt-5-mini` (founder primary) + `gpt-5-nano` + `gpt-5` + `gpt-4o-mini` fallback all priced in `MODEL_PRICING`
- [x] **FOMO Ranker** (Phase 3C.1) — pure `rankEmail(raw, deps)` function: applies egress, builds versioned prompt (`PROMPT_VERSION='ranker-v0.1.0'`), routes through `ModelRouter('classification')`, JSON validator enforces `{ label: 'important'|'not_important', score: 0..1, reason: string }`. Egress invariant tested end-to-end (leak canaries in `body_html` / headers / attachment filenames never reach the prompt)
- [x] **Ranker eval fixtures + harness wiring** (Phase 3C.1) — 20 hand-written anonymized synthetic fixtures under [`src/eval/ranker-fixtures/`](src/eval/ranker-fixtures/); `runRankerEval(backend)` converts each through `applyEgressForRanker` + `buildRankerPrompt`, runs through the generic eval harness, returns `EvalResult` (TP/FP/TN/FN, precision/recall, json_valid). Tests cover constant-positive / constant-negative / broken-JSON / perfect-mock backends
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
- [x] **Phase 3B.2 flipped `gmail.read` to `'implemented'`** alongside `gmailReadExecutor` wireup in [`src/dispatch/external-executors.ts`](src/dispatch/external-executors.ts). The gate still requires consent (`requires_consent: true`) and a non-needs-reauth Google OAuth token (`requires_oauth_provider: 'google'`) before a dispatch is authorized
- [x] One external tool (`sendblue.send_user_message`) remains `'declared'` until 3E. `slack.founder_review` flipped to `'implemented'` in Phase 3D.1
- [x] Phase 3B.1 adds two HTTP routes: `POST /oauth/google/start` (session-authenticated) and `GET /oauth/google/callback` (state-HMAC-authenticated). The Phase 2F invariant "no HTTP route beyond `GET /health`" was scoped to Phase 2; Phase 3 explicitly authorizes workflow HTTP surface per the revised map
- [x] **Polling kill-switch** (Phase 3B.2): `FOMO_GMAIL_POLLING_ENABLED` defaults to `false`. The polling worker bootstrap in `apps/fomo/src/index.ts` reads this once at boot — when `false`, no interval is installed and no autonomous Gmail reads happen. `FOMO_GMAIL_POLLING_INTERVAL_MS` (default 60_000) tunes the cycle cadence when enabled. The gate does NOT consult the polling switch — ad-hoc `gmail.read` dispatches (admin endpoint, harness) remain possible when polling is off
- [x] **Bounded polling window** (Phase 3B.3): `FOMO_GMAIL_POLLING_MAX_CYCLES` defaults to unset (unbounded). When set to a positive integer N, the polling loop auto-stops after N cycles and emits `fomo.poll.cycle_cap_reached`. The 3B.3 founder smoke test sets this to a small N so the worker cannot accidentally keep polling against the live founder inbox
- [x] **No OAuth refresh-token flow in 3B.2.** On 401 the worker marks `needs_reauth` and skips that user. The next OAuth-connect through `/oauth/google/start` re-mints fresh tokens. A dedicated refresh path can land later (3B.3 or folded into 3C) without changing the worker's API
- [x] No SendBlue / Slack / real-model adapter exists yet (Phase 3D / 3E / 3C)
- [x] **Dispatch table cannot bypass the Permission Gate — structural guarantee** (Phase 3A.1). `dispatch.execute()` signature requires an `AuthorizedToolCall`, not a raw `tool_id`. The class has a private constructor; the only factory is `AuthorizedToolCall.fromDecision(decision)` which returns `null` unless `decision.allowed === true && decision.code === 'allowed' && isToolId(decision.tool_id)`. Runtime `instanceof` check in `execute()` rejects forged objects with code `'unauthorized'`
- [x] **`audit.write` dispatch never creates recursive audit logging** — executor calls `store.audit.write` directly; harness's `policy.decided` / `tool.invoked` audits go direct to store, not through dispatch

### Integration
- [x] [`apps/fomo/src/kernel/integration-harness.ts`](src/kernel/integration-harness.ts) exercises every kernel piece in one in-process scenario
- [x] [`apps/fomo/src/kernel/integration-harness.test.ts`](src/kernel/integration-harness.test.ts) asserts the substrate cooperates — fails loudly on any regression
- [x] `KernelScenarioReport` is deeply frozen — callers cannot mutate it
- [x] Two scenario runs are independent — each constructs its own in-memory stores
- [x] **Audit log participates in the integrated path** (Phase 2F.1) — harness writes a sanitized audit entry at every kernel touch. Gate test asserts entries > 0, per-action breakdown, and forbidden-content leak canary (no raw email body / headers / attachment filenames / prompt text / reply text in any audit entry).
- [x] **Dispatch table participates in the integrated path** (Phase 3A + 3B.2) — harness wires `wireInternalExecutors` + `wireExternalExecutors`, then routes 5 explicit internal invocations as `gate → AuthorizedToolCall.fromDecision → dispatch.execute → store write`. The 3 explicit declared/unknown probes return `null` from the factory. The runtime `unauthorized` deny path is covered by dedicated tests in [`dispatch/dispatcher.test.ts`](src/dispatch/dispatcher.test.ts).
- [x] **Polling worker participates in the integrated path** (Phase 3B.2) — harness seeds a token + cursor for the synthetic user, injects a mock `GmailClient` (with leak-canary headers/body) that returns one new message, then runs `gmail-poll.runOnce` once. The worker dispatches `gmail.read` end-to-end, advances the cursor, and writes its own per-message audit. The audit-leak canary test runs over the worker's audit entries too, proving the dispatch path emits no raw email content.

### CI + safety
- [x] `pnpm build` green
- [x] `pnpm test` green (default: in-memory stores only)
- [x] `pnpm test` green with `BREVIO_RUN_PG_TESTS=true` (Phase 2E.1 gated suite)
- [x] `pnpm lint` green (zero violations across all .ts files)
- [x] No live network calls in CI (asserted by `model-backends/mock.test.ts` tripwire)
- [x] No live DB connection in CI

### Founder sign-off
- [ ] Founder has reviewed this file and confirmed Phase 3 may begin

### Phase 3B.3 — Founder Real Gmail smoke test (separate gate before Phase 3C)
- [x] `FOMO_GMAIL_POLLING_MAX_CYCLES` env var added with auto-stop + `fomo.poll.cycle_cap_reached` log event
- [x] Preflight script ([scripts/preflight-3b3.ts](scripts/preflight-3b3.ts)) — verifies every required env var BEFORE the founder boots the server, fails loud on missing/invalid values, forbids `FOMO_SEND_ENABLED` / `FOMO_AUTO_SEND_ENABLED` / `FOMO_FRIEND_BETA_ENABLED` flips during the smoke window
- [x] Evidence script ([scripts/smoke-evidence-3b3.ts](scripts/smoke-evidence-3b3.ts)) — reads live Neon Postgres, asserts: gmail.readonly scope only, cursor advanced, polling cycle audit written, gmail.read dispatch audits + tool_invocations rows, leak-canary scan over 500 most recent audit + tool_invocations records
- [x] Runbook ([docs/smoke-test-3b3-gmail.md](../../docs/smoke-test-3b3-gmail.md)) — Google Cloud setup, redirect URI, env vars, OAuth handshake, polling observation, 401 path, stop, evidence
- [x] Report template ([docs/SMOKE_REPORT_TEMPLATE_3B3.md](../../docs/SMOKE_REPORT_TEMPLATE_3B3.md)) — founder fills in and commits as `docs/SMOKE_REPORT_3B3.md`
- [x] **Founder has executed the smoke test on one real Gmail account and committed `docs/SMOKE_REPORT_3B3.md` with `VERDICT: PASS`** (commit `733c8cff`, PR #21 merged) — Phase 3C unblocked

### Phase 3C.2 — OpenAI ranker smoke eval (separate gate before Phase 3C.3)
*Founder directive 2026-05-24: this gate supersedes the prior Anthropic Haiku-vs-Sonnet bake-off. OpenAI is the initial Brevio ranker; AnthropicBackend remains in main as dormant future-provider support.*
- [x] `OpenAIBackend` shipped in 3C.2 with strict `response_format: json_schema` support; `gpt-5-mini` priced in `MODEL_PRICING` plus `gpt-5-nano`, `gpt-5`, and `gpt-4o-mini` fallback rows
- [x] Preflight script ([scripts/preflight-3c2.ts](scripts/preflight-3c2.ts)) — validates `OPENAI_API_KEY`, resolves `FOMO_OPENAI_MODEL` (default `gpt-5-mini`), prints per-fixture cost estimate; no DB / no network
- [x] Smoke eval script ([scripts/smoke-eval-3c2.ts](scripts/smoke-eval-3c2.ts)) — runs the resolved model against the 20 synthetic fixtures with the ranker JSON-schema enforced server-side, applies Conservative pass gate (`precision ≥ 0.85`, `recall ≥ 0.85`, `json_valid ≥ 0.95`), writes `docs/openai-smoke-eval-3c2-results.json`, exits 0 on PASS / 1 on INVESTIGATE
- [x] Runbook ([docs/openai-smoke-eval-3c2-runbook.md](../../docs/openai-smoke-eval-3c2-runbook.md)) and report template ([docs/OPENAI_SMOKE_REPORT_TEMPLATE_3C2.md](../../docs/OPENAI_SMOKE_REPORT_TEMPLATE_3C2.md))
- [x] **Founder has executed the smoke eval and committed `docs/OPENAI_SMOKE_REPORT_3C2.md` + `docs/openai-smoke-eval-3c2-results.json` with `VERDICT: PASS`** (gpt-5-mini classified 20/20 synthetic fixtures correctly; commit `0fe41935`, PR #24 merged) — Phase 3C.3 unblocked

### Phase 3C.3 — Ranker-on-poll wiring (substrate, no smoke gate)
*Substrate wiring only: connects the (3B.3-validated) Gmail polling worker to the (3C.2-validated) OpenAI ranker behind the new `FOMO_RANKER_ENABLED` kill switch. No new external integration, so no smoke gate of its own. The next gate is 3C.4, where real Gmail + real ranker run together against the founder inbox.*
- [x] **Kill switch** `FOMO_RANKER_ENABLED` (default false) added to [src/core/kill-switches.ts](src/core/kill-switches.ts); strict opt-in parsing; unit-tested independently. When false, the polling worker skips the ranker entirely — no model calls, no `rank_results` writes
- [x] **`rank_results` table** added: migration `src/db/migrations/0002_rank_results.sql`, schema entry, unique constraint on `(user_id, message_id)` for idempotency. Decision-only columns (`label`, `score`, `reason`, plus model metadata); zero payload-shaped columns (body / headers / attachments / prompt). Privacy invariant asserted in `gated-pg.test.ts`
- [x] **`RankResultStore`** (interface + `InMemoryRankResultStore` in [src/memory/rank-results.ts](src/memory/rank-results.ts) + `PostgresRankResultStore` in [src/db/stores/rank-results-postgres.ts](src/db/stores/rank-results-postgres.ts)): `write()` returns `{ inserted }` so the worker can distinguish first-rank from idempotency hit; `get` / `count` / `recent` for evidence queries. Wired into `SubstrateStores` via [src/db/store-factory.ts](src/db/store-factory.ts)
- [x] **Polling worker integration** ([src/workers/gmail-poll.ts](src/workers/gmail-poll.ts)): optional `ranker` dep on `GmailPollDeps` — absent → 3B.2/3B.3 behavior unchanged (backward-compatible); present → every successful `gmail.read` dispatch is handed to `ranker.rank()` and persisted. RankerFailure / unexpected throws are audited as `fomo.rank.failed` and never abort the cycle. Idempotency hits audited as `fomo.rank.already_ranked`. Per-user outcome + cycle report carry the new counters (`messages_ranked`, `messages_rank_already`, `messages_rank_failed`)
- [x] **Audit actions** added to `AuditAction` union: `fomo.rank.completed` / `fomo.rank.already_ranked` / `fomo.rank.failed`. Details surface only operational fields (model_name, prompt_version, label, score, latency, tokens, cost); never body content. The audit redactor strips `code`, so the ranker error code field is named `error_code`
- [x] **Bootstrap** ([src/index.ts](src/index.ts)): `buildRankerDep()` constructs `OpenAIBackend` + `createModelRouter` + binds `rankEmail`. Returns null when the kill switch is off; THROWS at boot when the kill switch is on but `OPENAI_API_KEY` is missing — "real or absent, never half-wired." Boot logs surface `fomo.ranker.enabled` / `fomo.ranker.disabled`; `fomo.poll.cycle` now carries the new counters
- [x] **Shared OpenAI response-format constant** ([src/ranker/openai-response-format.ts](src/ranker/openai-response-format.ts)): the JSON schema sent to OpenAI lives in one place; both the 3C.2 smoke eval and the 3C.3 production bootstrap import it so the gate verdict applies to the production contract
- [x] Unit-test coverage for: ranker-absent (3B.2 backward-compat), ranker-success (rank_results row + audit), ranker-failure (audit only, no row), unexpected-throw (cycle continues), idempotency on second-cycle replay, audit detail privacy (no body keys), cycle audit detail includes ranker counters
- [x] **Substrate wiring only — no founder smoke run required for 3C.3.** The next gate is **3C.4**: real Gmail + real ranker against the founder inbox, with a leak-canary scan over `rank_results` + `audit_log` and an evidence script that verifies per-fixture decisions persisted as expected

### Phase 3C.4 — Founder Real Gmail + Real Ranker smoke test (gate before 3D)
*First end-to-end run of the full chain — real Gmail → polling worker → gmail.read → real OpenAI ranker → rank_results row → audit. Prerequisites are 3B.3 PASS and 3C.2 PASS, both already on `main`. Phase 3D Slack adapter does NOT begin until this gate's PASS report is committed.*
- [x] Preflight script ([scripts/preflight-3c4.ts](scripts/preflight-3c4.ts)) — validates 3B.3 env vars + `FOMO_RANKER_ENABLED=true` + `OPENAI_API_KEY`; warns on forbidden flags; no DB / no network
- [x] Evidence script ([scripts/smoke-evidence-3c4.ts](scripts/smoke-evidence-3c4.ts)) — queries live Neon for: gmail.readonly scope (regression), gmail_cursors present, `fomo.poll.cycle` audit (with ranker counters in detail), gmail.read dispatch audits, **`fomo.rank.completed` ≥ 1** (real ranks happened), **`fomo.rank.already_ranked` ≥ 1** (idempotency seam exercised against live Postgres), `rank_results` ≥ 1 row, leak-canary scan over audit + tool_invocations + `rank_results.reason`. Exits 1 on any FAIL
- [x] Runbook ([docs/smoke-test-3c4-rank-on-poll.md](../../docs/smoke-test-3c4-rank-on-poll.md)) — extends .env from 3B.3 with `FOMO_RANKER_ENABLED=true` + `OPENAI_API_KEY`, walks through cycle 1 (real ranks), cycle 2 (cursor rewind → exercise `ON CONFLICT DO NOTHING`), evidence, clean-stop confirmation
- [x] Report template ([docs/SMOKE_REPORT_TEMPLATE_3C4.md](../../docs/SMOKE_REPORT_TEMPLATE_3C4.md)) — founder fills in and commits as `docs/SMOKE_REPORT_3C4.md`. Includes load-bearing §6 (evidence-script stdout) and informational §7 (founder eyeball on label reasonableness — not a gate criterion)
- [x] pnpm scripts wired: `preflight:3c4` + `smoke-evidence:3c4`
- [x] **PASS gate (founder-confirmed): seam-works only.** Every required check in evidence must be green: rank_results rows exist, audit events fire with correct fields, no leaks, no crashes, idempotency holds, scope still readonly. Label correctness is NOT a gate criterion — that is a separate phase
- [x] **Founder has executed the smoke test on the real founder Gmail with real OpenAI calls and committed `docs/SMOKE_REPORT_3C4.md` with `VERDICT: PASS`** (16 successful ranks, 5 already_ranked idempotency hits against live Neon Postgres, 0 leak-canary hits; commit `cbabe779`) — Phase 3D.1 unblocked

### Phase 3D.1 — Slack Candidate Review Posting (substrate; smoke gate is 3D.2)
*Founder directive 2026-05-25: 3D split into 3D.1 (posting) + 3D.2 (approval capture + smoke). 3D.1 ships the outbound half only — alerts created here sit at `queued_for_review` indefinitely until 3D.2 wires the inbound Slack callback that captures founder approve/reject. **3E SendBlue does NOT begin until 3D.2 PASS.***
- [x] **Kill switch** `FOMO_SLACK_REVIEW_ENABLED` (default false) added to [src/core/kill-switches.ts](src/core/kill-switches.ts); strict opt-in; unit-tested independently. When false, the polling worker skips the Slack flow entirely — no `alerts` rows, no Slack API calls
- [x] **`alerts` table** added: migration `src/db/migrations/0003_alerts.sql`, schema entry, UNIQUE constraint on `rank_result_id` for idempotency. Holds operational identifiers only (alert_id, user_id, message_id, rank_result_id, label, score); zero payload-shaped columns (no body / no subject / no sender_email). Privacy invariant asserted in `gated-pg.test.ts`
- [x] **`AlertStore`** (interface + `InMemoryAlertStore` in [src/memory/alerts.ts](src/memory/alerts.ts) + `PostgresAlertStore` in [src/db/stores/alerts-postgres.ts](src/db/stores/alerts-postgres.ts)): `create()` returns `{ inserted, alert }` so the worker can distinguish first-post from idempotency hit. Wired into `SubstrateStores` via [src/db/store-factory.ts](src/db/store-factory.ts)
- [x] **`SlackClient`** ([src/adapters/slack/client.ts](src/adapters/slack/client.ts)): direct `chat.postMessage` HTTP client. Fail-closed at construction (botToken / channelId / shape checks). Error classes: `SlackAuthError` (401/403, app-layer `invalid_auth` / `token_revoked`), `SlackApiError` (rate_limited / channel_not_found / 5xx / network — `retryable` boolean for callers). `buildFounderReviewBlocks` exported as the canonical card layout; uses the egress-redacted `SlackEgressView` (masked sender, ≤N-char snippet, no body / headers / attachments)
- [x] **`slack.founder_review` executor** wired in [src/dispatch/external-executors.ts](src/dispatch/external-executors.ts). Tool registry flipped `slack.founder_review` from `declared` → `implemented`. **`risk_tier` demoted `send` → `internal`** because posting to the founder's own channel for review is internal observability, NOT a user-facing send; FOMO_SLACK_REVIEW_ENABLED (enforced at bootstrap) is the proper gate
- [x] **Worker integration** ([src/workers/gmail-poll.ts](src/workers/gmail-poll.ts)): optional `slackReview` dep on `GmailPollDeps` — absent → 3C.3 behavior unchanged. Present + RankerSuccess with `label='important'` → walks `detected → ranked`, creates `alerts` row, dispatches `slack.founder_review`, on success transitions `ranked → queued_for_review` (audit `fomo.slack.posted`), on failure `ranked → failed` (audit `fomo.slack.failed`). Idempotency hit (alerts UNIQUE on rank_result_id) audits `fomo.slack.already_alerted` and skips the Slack call entirely. **Re-ranking the same message NEVER produces a duplicate Slack post** — proven in `gmail-poll.test.ts` ("IDEMPOTENCY (load-bearing Phase 3D.1 invariant)" suite)
- [x] **Audit actions** added: `alert.created`, `fomo.slack.posted`, `fomo.slack.already_alerted`, `fomo.slack.failed`. Detail surfaces only operational fields (alert_id, rank_result_id, slack_ts, model_name, label, score); NEVER body content. The Slack card payload itself is egress-redacted via `applyEgressForSlackCard` and is NOT included in any audit row
- [x] **Bootstrap** ([src/index.ts](src/index.ts)): `buildSlackReviewWiring()` constructs `SlackClient` + the `slackReview` dep, throws at boot when the switch is on but `SLACK_BOT_TOKEN` or `SLACK_FOUNDER_CHANNEL_ID` is missing ("real or absent, never half-wired"). Boot logs surface `fomo.slack.review.enabled` / `fomo.slack.review.disabled`; `fomo.poll.enabled` + `fomo.server.listening` include `slack_review_enabled`
- [x] Unit-test coverage for: slackReview-absent (3C.3 backward-compat), slackReview present + happy path (alerts + slack.posted + transitions), label=not_important skips the Slack flow, **load-bearing idempotency invariant** (re-rank does not double-post AND pre-existing alert produces `fomo.slack.already_alerted`), Slack failure (transition ranked→failed + audit), aggregate cycle audit includes Slack counters
- [x] **Substrate wiring only — no founder smoke run required for 3D.1.** The next gate is **3D.2**: Slack approval capture (inbound webhook with signature verification) + founder smoke test proving the full alert lifecycle (`queued_for_review → approved | rejected` with real Slack interaction)

### Phase 3D.2 — Slack Approval Capture + Slack Smoke Test (gate before 3E)
*Founder directive 2026-05-25 (tightened): closes the trust-checkpoint loop. After PR #28 (3D.1 substrate), alerts sit at `queued_for_review` forever. 3D.2 wires the **first inbound HTTP integration with HMAC signature verification** and the first user-approval engine in the codebase. **3E SendBlue does NOT begin until `docs/SMOKE_REPORT_3D2.md` PASS is on `main`.***
- [x] **SlackClient extension**: added `verifySlackSignature` (HMAC-SHA256 over `v0:${ts}:${body}` with timing-safe compare + 300s freshness window) and `updateFounderReviewCard` (chat.update with resolution blocks). [adapters/slack/client.ts](src/adapters/slack/client.ts) + [test](src/adapters/slack/client.test.ts) — 9 new verifier cases (stale / future / malformed / wrong-secret / tampered-body / wrong prefix / wrong length) + 4 chat.update cases + resolution-block privacy cases
- [x] **Approve/Reject buttons** added to `buildFounderReviewBlocks`. `block_id="fomo_alert:<alert_id>"`, `action_id="fomo.approve"` or `"fomo.reject"`. Resolution card (`buildFounderReviewResolutionBlocks`) removes the buttons and shows `Approved by <@user> at <ts>` (or Rejected)
- [x] **New audit actions** (6): `fomo.slack.interaction_received` (every inbound POST, BEFORE signature verify — flood-of-unsigned-requests is still visible), `fomo.slack.signature_invalid`, `fomo.slack.payload_invalid`, `fomo.slack.approval_unauthorized` (wrong channel / wrong user), `fomo.slack.approval_captured` (success), `fomo.slack.approval_duplicate` (idempotency hit; first-wins)
- [x] **`/slack/interactivity` route** ([src/routes/slack-interactivity.ts](src/routes/slack-interactivity.ts) + [test](src/routes/slack-interactivity.test.ts) — **16 cases** covering happy path, signature verification (invalid sig / stale ts / tampered body), channel/user authorization, idempotency / first-wins, unknown alert_id, malformed payload, kill-switch off). Pattern matches `oauth-google` (pure handler + HTTP adapter).
- [x] **Bootstrap** ([src/index.ts](src/index.ts)): `buildSlackReviewWiring` now requires `SLACK_SIGNING_SECRET` when `FOMO_SLACK_REVIEW_ENABLED=true` (boot-time fail-closed). Optional `SLACK_FOUNDER_USER_ID` restricts approval to one user. Route is mounted on the HTTP server only when wiring exists. Boot logs surface `interactivity_route_mounted` + `founder_user_restricted`
- [x] **State machine**: production caller for the `queued_for_review → approved | rejected` transitions (defined in Phase 2C; first-time-fired here). `transition()` validation runs before each write; idempotency check via `currentState()` prevents re-firing
- [x] **Feedback events**: `founder_approved` and `founder_rejected` written via the existing `FeedbackStore.write` path. Feeds the adaptive-learning layer per FOMO_DESIGN §13
- [x] **Sanitized audit detail**: route never persists raw Slack payload, full Slack user_id, or message text. Only operational identifiers: alert_id, action_id, decision_code, user_slug (4-char suffix), channel_slug, from_state, to_state, decided_at. Asserted by `NEVER persists the raw body in audit detail` + `NEVER persists the full Slack user_id` tests
- [x] Preflight script ([scripts/preflight-3d2.ts](scripts/preflight-3d2.ts)) — validates `SLACK_*` env vars, warns on missing `SLACK_FOUNDER_USER_ID` (best-effort mode), checks channel id and user id shape, fails on forbidden flags
- [x] Evidence script ([scripts/smoke-evidence-3d2.ts](scripts/smoke-evidence-3d2.ts)) — queries Neon for: alerts row, transitions to queued_for_review (3D.1 carry-forward), transitions to approved/rejected (3D.2 REQUIRED), feedback events, all 6 new audit actions, leak-canary scan extended to `feedback_events.detail` + `alert_state_transitions.reason`
- [x] Runbook ([docs/smoke-test-3d2-slack-approval.md](../../docs/smoke-test-3d2-slack-approval.md)) — walks Slack app creation, ngrok/cloudflared setup, env additions, the cycle that produces the alert, the approve click, the idempotency exercise, evidence, clean-stop
- [x] Report template ([docs/SMOKE_REPORT_TEMPLATE_3D2.md](../../docs/SMOKE_REPORT_TEMPLATE_3D2.md))
- [x] pnpm scripts: `preflight:3d2` + `smoke-evidence:3d2`
- [x] **Defense-in-depth at 3 layers (mirrors 3D.1 pattern + extends it)**:
  1. Bootstrap — route NOT mounted when switch is off; throws when switch is on but creds missing
  2. Route handler — re-checks kill switch at request time; rejects with `kill_switch_off` audit
  3. Signature verification — every inbound request HMAC-verified; stale-timestamp protection
- [x] **Founder has executed the smoke test against a real Slack workspace with a real button click and committed `docs/SMOKE_REPORT_3D2.md` with `VERDICT: PASS`** (PR #29 merged onto `main` 2026-05-25; Phase 3E.1 unblocked)

### Phase 3E.1 — SendBlue Outbound Substrate (substrate; smoke gate is 3E.2)
*Founder directive 2026-05-25 (tightened): 3E split into **3E.1 (substrate)** and **3E.2 (real-send founder smoke)**. 3E.1 wires the SendBlue HTTP adapter, the `sendblue.send_user_message` executor, the deterministic founder-text template, and the outbound-sender worker that picks up alerts in state `approved` and walks them to `sent | send_status_unknown | failed`. **NO live SendBlue calls happen in 3E.1** — the substrate ships mock-tested only (founder did not yet have a SendBlue account at PR time). 3E.2 unblocks once the founder provisions the account; 3F SendBlue inbound NOT begun until 3E.2 PASS.*
- [x] **Kill switches verified strict-opt-in**: `FOMO_SEND_ENABLED` (default false) gates send-tier dispatch at the policy gate (`risk_tier='send' → deny('send_disabled')`); `FOMO_AUTO_SEND_ENABLED` (default false) gates `intent='auto_send'` dispatches (3E.1 only sends `intent='manual_send'`, so auto-send stays off through v0.1). The outbound-sender worker also fail-closes at boot when `FOMO_SEND_ENABLED=true` but `SENDBLUE_API_KEY_ID` / `SENDBLUE_API_SECRET_KEY` / `FOMO_FOUNDER_PHONE_NUMBER` (E.164) / `FOMO_FOUNDER_USER_ID` is missing — "real or absent, never half-wired"
- [x] **`SendBlueClient`** ([src/adapters/sendblue/client.ts](src/adapters/sendblue/client.ts)): direct `POST /api/send-message` HTTP client, injectable `FetchLike`, configurable timeout (default 10s). **Three-outcome semantics** (founder directive — never auto-retry ambiguous):
  - `sent` → HTTP 2xx + provider status `QUEUED`/`SENT`/`DELIVERED`
  - `failed` → HTTP 2xx + provider status `FAILED`/`ERROR`; HTTP 4xx (incl. 401/403 auth); argument errors
  - `send_status_unknown` → network failure, timeout, HTTP 5xx, HTTP 429, HTTP 2xx with unrecognized status, malformed JSON. Caller MUST NOT auto-retry — duplicate sends could deliver real iMessages twice
- [x] **Deterministic founder-text template** ([src/core/founder-text-template.ts](src/core/founder-text-template.ts)) — **no LLM voice generation** per founder correction 2026-05-25. Renders ≤280 chars from egress-redacted safe fields only: `FOMO · <LABEL> (score) / <masked sender> / <subject ≤80> / <snippet ≤120>`. Drops snippet → subject in overflow order; header + sender are mandatory. `FOUNDER_TEXT_TEMPLATE_VERSION` stamped into audit detail for traceability. Test coverage proves the template NEVER unmasks sender, NEVER includes message_id / received_at, ALWAYS bounds total length to 280 chars
- [x] **`sendblue.send_user_message` executor** wired in [src/dispatch/external-executors.ts](src/dispatch/external-executors.ts). Tool registry flipped `sendblue.send_user_message` from `declared → implemented`. Risk tier remains `send` so `FOMO_SEND_ENABLED=false` denies at the gate's `send_disabled` branch — defense-in-depth complements the bootstrap fail-closed
- [x] **Outbound-sender worker** ([src/workers/outbound-sender.ts](src/workers/outbound-sender.ts)): `runOutboundOnce(deps)` cycle iterates users via `gmailCursors.listUserIds()`, finds alerts whose latest transition is `approved` via the new `AlertStateTransitionStore.findAlertIdsInState(userId, 'approved', limit)`, re-reads Gmail via existing `gmail.read` dispatch to recover the `SlackEgressView`, renders the deterministic template, dispatches `sendblue.send_user_message`, and transitions the alert per the three-outcome rule. **Idempotency via the state machine itself** — `findAlertIdsInState('approved', ...)` does not return alerts already in `sent`/`failed`/`send_status_unknown`, so the next cycle never re-fires
- [x] **Founder-phone allowlist (defense-in-depth)**: `destinationFor(user_id)` returns `FOMO_FOUNDER_PHONE_NUMBER` ONLY for `FOMO_FOUNDER_USER_ID`, null for everyone else. Worker audits `fomo.send.unauthorized_destination` and transitions `approved → failed` WITHOUT calling SendBlue when destination is null
- [x] **`findAlertIdsInState` query** added to `AlertStateTransitionStore` (in-memory + Postgres). Postgres uses `SELECT DISTINCT ON (alert_id) ORDER BY alert_id, id DESC` to pick the latest transition per alert, then filters by `to_state`. Test coverage in [src/core/alert-state-transitions.test.ts](src/core/alert-state-transitions.test.ts) and [src/db/stores/gated-pg.test.ts](src/db/stores/gated-pg.test.ts)
- [x] **Audit actions** added (6): `fomo.send.attempted` (BEFORE the SendBlue call so a flood of crashed attempts is still visible), `fomo.send.succeeded`, `fomo.send.failed`, `fomo.send.status_unknown`, `fomo.send.unauthorized_destination`, `fomo.send.kill_switch_off`. Detail surfaces only operational fields: `alert_id`, `message_id`, `rank_result_id`, `destination_slug` (4-char suffix of the phone — NEVER the full number), `template_version`, `provider_status`, `provider_message_handle`. NEVER the rendered message text, NEVER the full phone, NEVER the SendBlue API key
- [x] **Bootstrap** ([src/index.ts](src/index.ts)): `buildSendWiring()` constructs `SendBlueClient` + the outbound-sender deps factory, throws at boot on missing creds, returns null fields when `FOMO_SEND_ENABLED=false`. `startOutboundSenderInterval()` runs the worker on the same `FOMO_GMAIL_POLLING_INTERVAL_MS` cadence as the Gmail polling worker (one rhythm to reason about). Boot logs surface `fomo.send.enabled` / `fomo.send.disabled`, `fomo.outbound.enabled` / `fomo.outbound.disabled`, and `fomo.server.listening` includes `send_enabled` + `outbound_worker_started`
- [x] **Integration harness updated**: `sendblue.send_user_message` no longer denies `not_implemented`. Under safe defaults it denies `send_disabled` (the kill-switch action-boundary branch). All six v0.1 tools are now `implemented`; no tool remains `declared`. Audit-entry counts unchanged (28 per scenario run); test now asserts `send_disabled` for the explicit sendblue invocation
- [x] **Unit-test coverage** (53 new tests across founder-text-template / SendBlueClient / outbound-sender / executor / transitions / harness updates): happy-path sent, idempotency (sent / failed / status_unknown do NOT re-fire), all three terminal outcomes, defense-in-depth founder-phone allowlist, kill-switch denial at gate, Gmail re-read failure → status_unknown, audit privacy invariant (no rendered text, no full phone), bounded message length, no LLM voice generation
- [x] **Substrate wiring only — no founder smoke run required for 3E.1.** The next gate is **3E.2 — SendBlue Outbound Founder-Only Smoke Test**: founder provisions a SendBlue account, sets `FOMO_SEND_ENABLED=true` + `FOMO_FOUNDER_PHONE_NUMBER`, walks a real important email through Gmail → ranker → Slack approval → outbound-sender → real iMessage to the founder's own phone, commits `docs/SMOKE_REPORT_3E2.md` with `VERDICT: PASS`

### Phase 3E.2 — SendBlue Outbound Founder-Only Smoke Test (gate before 3F)
*Founder directive 2026-05-26: founder has a SendBlue account ready. Scaffolding lands AND smoke runs in the same PR (same pattern as 3D.2). Phase 3F SendBlue inbound NOT begun until this gate's PASS report lands on `main`.*

**Smoke-surfaced substrate findings (2026-05-26)** — same pattern as 3B.3 (session-token shape) and 3C.2 (gpt-5 temperature reject). Both fixes are in the same PR as the smoke pass.
- [x] **SendBlue requires `from_number` in every send-message POST body.** Real provider returned HTTP 400 `missing required parameter: "from_number"` against the 3E.1 SendBlueClient. Mock tests didn't catch this because the synthetic responses ignored body shape. Fix: added required `fromNumber` field to `SendBlueClientConfig`, included `from_number` in the JSON body, new `SENDBLUE_FROM_NUMBER` env var, wired through `buildSendWiring` (fail-closed at boot when `FOMO_SEND_ENABLED=true` but missing), validated in `preflight-3e2`. Unit-test added: confirms POST body now carries `from_number`. Tests confirm constructor throws when `fromNumber` missing or not E.164
- [x] **Default SendBlueClient timeout bumped 10s → 30s.** Free-tier SendBlue responded in ~13s during the smoke (HTTP 400 included), past the 10s default. The 10s timeout produced `send_status_unknown` (per design — never auto-retry ambiguous) before the response arrived. 30s is conservative; bypassable via `timeoutMs` config field. This is the design-correct three-outcome behavior — the smoke run produced an `approved → send_status_unknown` transition that stayed terminal per founder directive (no duplicate iMessages from auto-retry)
- [x] **Bounded smoke window** — `FOMO_OUTBOUND_MAX_CYCLES` kill switch added to [src/core/kill-switches.ts](src/core/kill-switches.ts); null default (unbounded production), positive integer caps the outbound-sender worker to N cycles before auto-stop. Mirrors `FOMO_GMAIL_POLLING_MAX_CYCLES` (3B.3). The 3E.2 runbook sets this to 1-3 so the worker cannot keep firing real iMessages during the smoke window
- [x] **`fomo.outbound.cycle_cap_reached`** log event added to `startOutboundSenderInterval` in [src/index.ts](src/index.ts) — emits once when the cap fires; the `fomo.outbound.cycle` log line now carries `cycle_number` + `cycle_cap` so the founder can verify the cap is active
- [x] Preflight script ([scripts/preflight-3e2.ts](scripts/preflight-3e2.ts)) — validates `FOMO_SEND_ENABLED=true`, `SENDBLUE_API_KEY_ID`, `SENDBLUE_API_SECRET_KEY`, `FOMO_FOUNDER_PHONE_NUMBER` (E.164), `FOMO_FOUNDER_USER_ID`; warns when `FOMO_OUTBOUND_MAX_CYCLES` is unset (cap strongly recommended for smoke); refuses `FOMO_AUTO_SEND_ENABLED=true` / `FOMO_FRIEND_BETA_ENABLED=true`
- [x] Evidence script ([scripts/smoke-evidence-3e2.ts](scripts/smoke-evidence-3e2.ts)) — queries Neon for: ≥1 `alert_state_transitions` row `approved → sent` (LOAD-BEARING), ≥1 `tool_invocations.sendblue.send_user_message` row with `status=success`, `fomo.send.attempted` ≥ 1, `fomo.send.succeeded` ≥ 1, `fomo.send.unauthorized_destination` == 0 (allowlist held), leak-canary scan over audit + tool_invocations + feedback + transitions confirms NO full phone number / SendBlue API key / rendered text leaks. Resolves `FOMO_FOUNDER_PHONE_NUMBER` at runtime so the literal-phone canary can run direct-match
- [x] Runbook ([docs/smoke-test-3e2-sendblue-outbound.md](../../docs/smoke-test-3e2-sendblue-outbound.md)) — SendBlue account setup, sandbox-first dry-run option, env additions including `FOMO_OUTBOUND_MAX_CYCLES`, full cycle walkthrough (Gmail → rank → Slack approve → outbound-sender → real iMessage → state transition `approved → sent`), idempotency exercise (second cycle does NOT re-send), `fomo.outbound.cycle_cap_reached` verification, optional failure-path exploration, clean-stop with kill switch off
- [x] Report template ([docs/SMOKE_REPORT_TEMPLATE_3E2.md](../../docs/SMOKE_REPORT_TEMPLATE_3E2.md)) — founder fills in and commits as `docs/SMOKE_REPORT_3E2.md`; includes load-bearing §6 (evidence-script stdout) and §7 founder observations (iMessage contents, idempotency proof, no surprising content)
- [x] pnpm scripts wired: `preflight:3e2` + `smoke-evidence:3e2`
- [x] **PASS gate (founder-confirmed): seam-works + real-delivery.** Every required check in evidence must be green: real iMessage delivered (exactly one), no `fomo.send.unauthorized_destination`, no leaks, idempotency holds, clean stop. Sandbox dry-run is OPTIONAL but RECOMMENDED before the production-delivery run
- [x] **Founder has executed the smoke test against a real SendBlue account with a real iMessage to the founder's own phone and committed `docs/SMOKE_REPORT_3E2.md` with `VERDICT: PASS`** (PR #32 merged onto `main` 2026-05-26; Phase 3F.1 unblocked)

### Phase 3F.1 — SendBlue Inbound Reply Substrate (substrate; smoke gate is 3F.2)
*Founder directive 2026-05-26: 3F split into **3F.1 (substrate)** + **3F.2 (founder reply smoke)**. 3F.1 wires the signed `/sendblue/inbound` route, the reply parser (deterministic safety pre-pass → OpenAI classifier with low-confidence fail-safe), state-machine transitions for replies, feedback events, memory signal updates, and STOP enforcement in the outbound-sender. **No snooze resurfacing in 3F.1** (deferred to 3F.3 or 3G). Phase 3G demo gate NOT begun until 3F.2 PASS is on `main`.*
- [x] **Kill switch** `FOMO_SENDBLUE_INBOUND_ENABLED` (default false) added to [src/core/kill-switches.ts](src/core/kill-switches.ts); strict opt-in. When false the `/sendblue/inbound` route is NOT mounted; reply parser dormant. Defense-in-depth at three layers (mirrors 3D.2 + 3E.1): bootstrap, route handler re-check, signature verification
- [x] **`verifySendBlueWebhookSecret`** added to [src/adapters/sendblue/client.ts](src/adapters/sendblue/client.ts). **Founder-mandated correction during 3F.1 review (2026-05-26):** the initial substrate (commit `debe9c02`) implemented HMAC-SHA256 over the body, invented from common-webhook patterns rather than from SendBlue's docs. SendBlue actually uses a **plain shared secret in a request header**, NOT HMAC. Documentation evidence (fetched 2026-05-26 from https://docs.sendblue.com/getting-started/webhooks/): *"When you configure a secret, Sendblue will include it in the webhook request headers, allowing you to verify that the request is genuinely from Sendblue."* The verifier compares the configured secret against the header value with a timing-safe compare via `node:crypto.timingSafeEqual`. The SendBlue docs do NOT name the exact header carrying the secret — the route layer reads the header configured via `SENDBLUE_WEBHOOK_SECRET_HEADER` env var (default `sb-signing-secret`, matching the `sb-*` API-header naming pattern). 3F.2 founder smoke will confirm/override the actual header name without a code change. Result shape: `{ ok: true }` or `{ ok: false, reason: 'missing_header' | 'secret_mismatch' }`. ~9 unit tests cover happy path, whitespace tolerance, missing-header, wrong-secret of varying lengths, timing-safe compare semantics, and result immutability
- [x] **Reply parser** — three modules under [src/reply-parser/](src/reply-parser/):
  - [`deterministic.ts`](src/reply-parser/deterministic.ts) — frozen exact-match set for compliance commands. STOP / UNSUBSCRIBE / CANCEL / END / QUIT → intent `stop`; START / UNSTOP / RESUME → intent `start`. Case-insensitive + trailing-punctuation tolerant. **NEVER consults the LLM** (founder directive 2026-05-26). 50 unit tests verify the bare-command match boundary — phrasings like "please stop" / "I want to unsubscribe" do NOT match deterministically (they fall through to the classifier)
  - [`prompt.ts`](src/reply-parser/prompt.ts) + [`validator.ts`](src/reply-parser/validator.ts) + [`openai-response-format.ts`](src/reply-parser/openai-response-format.ts) — OpenAI strict-JSON-schema classifier for soft intents (`snooze` / `ignore` / `ignore_sender` / `why` / `false_positive` / `unclear`) with `snooze_hint` sub-field (`later` / `tomorrow` / `remind_me_later` / `unspecified` / null). `PROMPT_VERSION='reply-parser-v0.1.0'`. 19 validator tests
  - [`index.ts`](src/reply-parser/index.ts) — orchestrator: deterministic pre-pass → classifier → low-confidence fail-safe. Default threshold 0.7 (tunable per call). When classifier confidence < threshold AND intent ≠ unclear, the orchestrator FORCES intent to `unclear` with `low_confidence_forced_unclear: true`. **No auto-retry** (founder directive). 14 orchestrator tests including egress-canary scan that proves the classifier prompt never carries `body_plain` / `body_html` / `Received:` / attachment filenames / `X-Probe` canaries
- [x] **`inbound_replies` table** — migration `src/db/migrations/0004_inbound_replies.sql`, schema entry, UNIQUE constraint on `provider_message_id` for SendBlue-retry idempotency. ONLY operational columns (`id`, `provider_message_id`, `user_id`, `received_at`); zero payload-shaped columns. Privacy invariant asserted in `gated-pg.test.ts`
- [x] **`InboundReplyStore`** (interface + [`InMemoryInboundReplyStore`](src/memory/inbound-replies.ts) + [`PostgresInboundReplyStore`](src/db/stores/inbound-replies-postgres.ts)): `record()` returns `{ inserted, record }` so the route can distinguish first-receipt from idempotency hit. Wired into `SubstrateStores`. 17 in-memory + 4 gated-PG tests
- [x] **`stop_active` memory signal kind** added to [src/memory/memory-signals.ts](src/memory/memory-signals.ts). Identity (user_id, kind='stop_active', scope_key=null). Detail shape: `{ active: boolean, recorded_at: ISO }`. STOP flips to true; START flips to false. The 7th memory signal kind alongside the 6 personalization signals. Memory-signals-test count updated
- [x] **`POST /sendblue/inbound` route** ([src/routes/sendblue-inbound.ts](src/routes/sendblue-inbound.ts) + [test](src/routes/sendblue-inbound.test.ts) — **~16 cases**, including a dedicated "auth-failed requests do NOT parse / NOT transition / NOT update memory" load-bearing fail-closed test). Audit-first (BEFORE auth verify) → kill-switch re-check → webhook-secret header verify (plaintext equality, timing-safe) → JSON parse → tolerant payload extraction → from-number allowlist → `inbound_replies` dedup → parser invocation → outcome dispatch. Outcomes:
  - **stop** (deterministic) → `stop` feedback event + `stop_active=true` memory_signal + `fomo.sendblue.stop_recorded` audit. No alert state transition (STOP is global, not per-alert)
  - **start** (deterministic) → `stop_active=false` memory_signal + `fomo.sendblue.start_recorded` audit
  - **snooze** (classifier) → `user_snoozed` feedback with `snooze_until` + alert state `sent → replied → snoozed` (snooze hint → duration: later=1h / tomorrow=24h / remind_me_later=4h / unspecified=1h)
  - **ignore** / **false_positive** (classifier) → feedback + alert state `sent → replied → ignored`
  - **ignore_sender** (classifier) → `ignored_sender` feedback + `sender_suppressed` memory_signal scoped by `message:<id>` + alert state `sent → replied → ignored`
  - **why** (classifier) → `asked_why` feedback + alert state `sent → replied` (no terminal transition)
  - **unclear** → audit `fomo.sendblue.reply_unclear`; NO state transition past `replied`. Fail-safe — operator inspects
- [x] **11 new audit actions** added to [src/core/audit.ts](src/core/audit.ts): `fomo.sendblue.{inbound_received, signature_invalid, payload_invalid, reply_unauthorized, reply_duplicate, reply_parsed, reply_unclear, stop_recorded, start_recorded, kill_switch_off}` PLUS outbound-side `fomo.send.stop_enforced`. Detail surfaces only operational identifiers + 4-char `from_slug` suffix. NEVER the raw webhook payload, NEVER the founder reply text, NEVER the full from-phone, NEVER the SendBlue signing secret. Asserted by 3 privacy-invariant tests
- [x] **STOP enforcement in outbound-sender** ([src/workers/outbound-sender.ts](src/workers/outbound-sender.ts)). At the top of every cycle per-user, the worker calls `isStopActive(user_id)`. When `stop_active.detail.active === true`, every approved alert this cycle is refused: audit `fomo.send.stop_enforced` + transition `approved → failed` (reason: stop_enforced). **NEVER calls SendBlue**. Per founder directive 2026-05-26: deterministic, no LLM. 5 new STOP-enforcement tests: refuses + audits + no SendBlue call; does NOT enforce when `active=false`; does NOT enforce when no signal; per-user (not per-alert) check; no phone leak
- [x] **Bootstrap** ([src/index.ts](src/index.ts)): `buildSendBlueInboundWiring()` constructs the route deps + a dedicated reply-parser OpenAI router (shares cost store with ranker; uses reply-parser response_format). Throws at boot when switch is on but `SENDBLUE_WEBHOOK_SECRET` / `FOMO_FOUNDER_PHONE_NUMBER` / `FOMO_FOUNDER_USER_ID` / `OPENAI_API_KEY` are missing. Optional `SENDBLUE_WEBHOOK_SECRET_HEADER` overrides the default header name `sb-signing-secret` (overrideable at smoke-run time once we observe the actual SendBlue header). Route mounted on the HTTP server only when wiring exists. Outbound-sender worker dep extended with `memoryStore` so STOP enforcement actually consults the signal. Boot logs surface `fomo.sendblue.inbound.enabled` (with `webhook_secret_header` so the founder can see exactly which header the route reads) / `fomo.sendblue.inbound.disabled`; `fomo.server.listening` includes `sendblue_inbound_route_mounted`
- [x] **Unit-test coverage** — ~140 new tests across kill switches / audit actions / memory signal / inbound-replies (in-memory + gated-PG) / signature verifier / reply parser (deterministic + validator + orchestrator) / inbound route / outbound STOP enforcement. **869 tests green total (+138 from main)**. Lint clean. Build clean
- [x] **Substrate wiring only — no founder smoke run required for 3F.1.** The next gate is **3F.2 — SendBlue Inbound Reply Founder-Only Smoke Test**: founder configures the SendBlue webhook against an ngrok tunnel, texts back to a real FOMO iMessage (a soft intent like "tomorrow" + STOP + START), commits `docs/SMOKE_REPORT_3F2.md` with `VERDICT: PASS`

### Phase 3F.2 — SendBlue Inbound Reply Founder-Only Smoke Test (gate before 3G)
*Scaffolding for runbook + preflight + evidence + report template will be authored in a follow-up PR alongside the founder's smoke run. Phase 3G demo gate NOT begun until 3F.2 PASS lands on `main`.*
- [ ] Preflight script (`scripts/preflight-3f2.ts`) — validates `FOMO_SENDBLUE_INBOUND_ENABLED=true` + `SENDBLUE_WEBHOOK_SECRET` + `FOMO_FOUNDER_PHONE_NUMBER` + `FOMO_FOUNDER_USER_ID` + `OPENAI_API_KEY`; warns on missing `FOMO_OUTBOUND_MAX_CYCLES` and on missing/non-default `SENDBLUE_WEBHOOK_SECRET_HEADER` (asks the founder to confirm SendBlue's actual header name from a real webhook request during smoke); refuses `FOMO_AUTO_SEND_ENABLED=true` / `FOMO_FRIEND_BETA_ENABLED=true`
- [ ] Evidence script (`scripts/smoke-evidence-3f2.ts`) — queries Neon for: ≥1 `inbound_replies` row; duplicate-webhook test gave inbound_replies count of 1 + `fomo.sendblue.reply_duplicate` audit (idempotency); ≥1 `fomo.sendblue.reply_parsed` for a soft intent + ≥1 `fomo.sendblue.stop_recorded`; `stop_active` memory_signal with `detail.active=true` at peak; ≥1 `fomo.send.stop_enforced` (proves STOP blocked outbound); START flipped `stop_active.detail.active` back to false IF tested; alert state transitioned `sent → replied → snoozed` for soft intent; leak-canary scan: NO full from-phone / NO reply text / NO signing secret in audit + feedback + memory + transitions + inbound_replies
- [ ] Runbook (`docs/smoke-test-3f2-sendblue-inbound.md`) — SendBlue webhook setup (ngrok tunnel + dashboard secret config), env additions (`SENDBLUE_WEBHOOK_SECRET`, `FOMO_SENDBLUE_INBOUND_ENABLED=true`); LOAD-BEARING step: inspect the very first SendBlue POST in the server log to identify the actual secret-bearing header name, then either confirm it matches the default `sb-signing-secret` or set `SENDBLUE_WEBHOOK_SECRET_HEADER` to the observed name and restart; smoke walkthrough (text "tomorrow" → snooze parsed; text "STOP" → stop_active + new alert blocked; text "START" → stop_active cleared), idempotency exercise, clean-stop
- [ ] Report template (`docs/SMOKE_REPORT_TEMPLATE_3F2.md`)
- [ ] pnpm scripts: `preflight:3f2` + `smoke-evidence:3f2`
- [ ] **Founder has executed the smoke test against a real SendBlue webhook with real iMessage replies and committed `docs/SMOKE_REPORT_3F2.md` with `VERDICT: PASS`** (load-bearing — Phase 3G demo gate may NOT begin until checked)

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
| **Gmail HTTP client** (Phase 3B.1) | [src/adapters/gmail/client.ts](src/adapters/gmail/client.ts) | [adapters/gmail/client.test.ts](src/adapters/gmail/client.test.ts) | ~15 | OAuth callback (seeds cursor); polling worker (every cycle); gmail.read executor (per message) | — |
| **Gmail cursor store** (Phase 3B.1) | [src/memory/gmail-cursors.ts](src/memory/gmail-cursors.ts) | [memory/gmail-cursors.test.ts](src/memory/gmail-cursors.test.ts) | ~13 (in-memory) + ~4 (gated PG) | OAuth callback (initializes); polling worker (advances + `listUserIds`) | — |
| **OAuth Google routes** (Phase 3B.1) | [src/routes/oauth-google.ts](src/routes/oauth-google.ts) | [routes/oauth-google.test.ts](src/routes/oauth-google.test.ts) | ~10 | `apps/fomo/src/index.ts` (wired when GOOGLE_CLIENT_ID/SECRET/REDIRECT env set) | — |
| **External Executors** (NEW Phase 3B.2) | [src/dispatch/external-executors.ts](src/dispatch/external-executors.ts) | [dispatch/external-executors.test.ts](src/dispatch/external-executors.test.ts) | ~9 | `wireExternalExecutors` registers `gmail.read` at dispatch construction; called by index.ts + integration harness | Phase 3D / 3E add slack / sendblue executors alongside |
| **Gmail Polling Worker** (Phase 3B.2, extended 3C.3) | [src/workers/gmail-poll.ts](src/workers/gmail-poll.ts) | [workers/gmail-poll.test.ts](src/workers/gmail-poll.test.ts) | ~20 | `apps/fomo/src/index.ts` (when `FOMO_GMAIL_POLLING_ENABLED=true`); integration harness (runs one cycle) | Phase 3C.4 founder real-Gmail smoke run validates real-ranker behavior end-to-end |
| **Anthropic backend** (Phase 3C.1, dormant) | [src/core/model-backends/anthropic.ts](src/core/model-backends/anthropic.ts) | [core/model-backends/anthropic.test.ts](src/core/model-backends/anthropic.test.ts) | ~14 | (no active caller; future-provider support per founder directive 2026-05-24) | Future-only: re-introduce if OpenAI fails or as a second provider |
| **OpenAI backend** (Phase 3C.2, wired 3C.3) | [src/core/model-backends/openai.ts](src/core/model-backends/openai.ts) | [core/model-backends/openai.test.ts](src/core/model-backends/openai.test.ts) | ~16 | 3C.2 smoke eval script; 3C.3 bootstrap (when `FOMO_RANKER_ENABLED=true`) registers it on the model router for `classification` | Phase 3C.4 exercises end-to-end against real Gmail |
| **FOMO Ranker** (Phase 3C.1, wired 3C.3) | [src/ranker/index.ts](src/ranker/index.ts) + [prompt.ts](src/ranker/prompt.ts) + [validator.ts](src/ranker/validator.ts) + [openai-response-format.ts](src/ranker/openai-response-format.ts) | [ranker/index.test.ts](src/ranker/index.test.ts) + [validator.test.ts](src/ranker/validator.test.ts) | ~27 | Gmail polling worker (3C.3) when `FOMO_RANKER_ENABLED=true` | Phase 3C.4 founder smoke validates against real Gmail |
| **RankResultStore** (Phase 3C.3, consumed 3D.1) | [src/memory/rank-results.ts](src/memory/rank-results.ts) + [src/db/stores/rank-results-postgres.ts](src/db/stores/rank-results-postgres.ts) | [memory/rank-results.test.ts](src/memory/rank-results.test.ts) + gated-pg coverage | ~14 + ~5 PG | Gmail polling worker (3C.3 ranker step + 3D.1 alert creation FK) | Phase 3D.2 Slack approval flow joins `alerts → rank_results` to surface decisions in approve/reject UX |
| **AlertStore** (NEW Phase 3D.1) | [src/memory/alerts.ts](src/memory/alerts.ts) + [src/db/stores/alerts-postgres.ts](src/db/stores/alerts-postgres.ts) | [memory/alerts.test.ts](src/memory/alerts.test.ts) + gated-pg coverage | ~14 + ~5 PG | Gmail polling worker (3D.1 Slack-review step) when `FOMO_SLACK_REVIEW_ENABLED=true` | Phase 3D.2 inbound Slack callback reads `alerts` to map a Slack message back to its alert_id for approve/reject capture |
| **SlackClient + buildFounderReviewBlocks** (NEW Phase 3D.1) | [src/adapters/slack/client.ts](src/adapters/slack/client.ts) | [adapters/slack/client.test.ts](src/adapters/slack/client.test.ts) | ~17 | `wireExternalExecutors` when `FOMO_SLACK_REVIEW_ENABLED=true` + creds present; registered on `slack.founder_review` tool | Phase 3D.2 reuses the same adapter for `chat.update` (edit posted card on approve/reject) and adds inbound Slack signature verification |
| **Ranker fixtures + eval wiring** (Phase 3C.1) | [src/eval/ranker-eval.ts](src/eval/ranker-eval.ts) + [src/eval/ranker-fixtures/fixtures.ts](src/eval/ranker-fixtures/fixtures.ts) | [eval/ranker-eval.test.ts](src/eval/ranker-eval.test.ts) | ~9 + 20 fixtures | OpenAI smoke eval script (3C.2); developer/CI tool | Phase 3C.4 founder real-Gmail smoke uses these as ongoing regression coverage |

---

## Audit participation matrix (Phase 2F.1 + 3A + 3B.2)

The integration harness writes a sanitized audit entry at every kernel
touch. The gate test asserts both the count and that no payload content
leaks into any entry. Phase 3B.2 changed the breakdown without changing
the total: removed the `gmail.read` denial entry pair (2 entries) and
added the polling worker's `gmail.read` dispatch entry pair (2 entries).
The polling worker also writes one `gmail.poll.cycle` entry per cycle
with `actor_user_id=null` (system actor) — that entry does NOT count
toward `audit.entries_written` (which reads `recent(SYNTHETIC_USER_ID)`)
but is asserted in the worker's own unit tests.

| Kernel touch | Audit action | Entries per scenario | Sanitized detail (no payload) |
|---|---|---|---|
| Gate decision | `policy.decided` | 9 | `tool_id`, `code`, `allowed` |
| Tool invocation write | `tool.invoked` | 9 | `tool_id`, `invocation_id`, `policy_decision`, `status` |
| Dispatched `audit.write` lifecycle event | `session.created` | 1 | `source` (executor calls `store.audit.write` directly — no recursive audit wrapper) |
| Alert state transition | `state.transitioned` | 6 | `alert_id`, `from_state`, `to_state` |
| Feedback write | `feedback.written` | 2 | `kind`, `alert_id`, `sender_present` (boolean, not the email) |
| Memory upsert | `memory.upserted` | 2 | `kind`, `scope_present` (boolean, not the scope_key), `source` |
| Model route | `model.routed` | 1 | `capability`, `model_name`, `prompt_version`, `schema_valid` (NOT prompt text or output text) |
| **Total** (user-scoped) | — | **30** | |
| Polling cycle (system actor) | `gmail.poll.cycle` | 1 (not user-scoped) | `users_total`, `users_polled`, `users_skipped`, `users_unauthorized`, `users_api_error`, `messages_observed`, `messages_dispatched`, `messages_failed` |

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

The audit action types (`policy.decided`, `tool.invoked`,
`state.transitioned`, `feedback.written`, `memory.upserted`,
`model.routed`, `gmail.poll.cycle`) are legitimate Phase 3 audit
categories. Phase 3 callers should continue writing them when the real
dispatch wiring lands.

## v0.1 Tool Registry status (Phase 3A + 3B.2 + 3D.1 + 3E.1)

All six v0.1 tools are now `'implemented'`. No tool remains
`'declared'` after Phase 3E.1 flipped `sendblue.send_user_message`
alongside the `SendBlueClient` adapter wireup. The gate enforces
per-tool policy (kill switches, consent, OAuth, tool-specific
defense-in-depth) before any dispatch is attempted.

| Tool ID | Surface | Risk tier | Executor status | Wired in | Gate-level guard |
|---|---|---|---|---|---|
| `gmail.read` | external | read | **implemented** | Phase 3B.2 | `requires_consent: true`, `requires_oauth_provider: 'google'` |
| `sendblue.send_user_message` | external | send | **implemented** | **Phase 3E.1** | `risk_tier='send'` denies `send_disabled` under `FOMO_SEND_ENABLED=false`; `intent='auto_send'` denies `auto_send_disabled` under `FOMO_AUTO_SEND_ENABLED=false` |
| `slack.founder_review` | internal | internal | **implemented** | Phase 3D.1 | tool-id-specific check: denies `slack_review_disabled` under `FOMO_SLACK_REVIEW_ENABLED=false` |
| `audit.write` | internal | internal | **implemented** | Phase 3A | — |
| `feedback.write` | internal | internal | **implemented** | Phase 3A | — |
| `memory_signal.write` | internal | internal | **implemented** | Phase 3A | — |

For every implemented tool, the gate consults policy (kill switches,
consent, OAuth, tool-id-specific defense-in-depth) and on allow,
dispatch routes to the executor. The integration harness asserts
this honest invariant — under safe-defaults `sendblue.send_user_message`
denies `send_disabled` and `slack.founder_review` denies
`slack_review_disabled`, both at the action boundary.

---

## Phase 3 map (revised)

| Subphase | Scope |
|---|---|
| **3A** | Internal Dispatch — dispatch table + 3 internal executors + flip those 3 tools to `implemented`. **(done)** |
| **3B.1** | Gmail HTTP client + OAuth go-live routes + `gmail_cursors` table + cursor store (in-memory + Postgres). `gmail.read` stays declared. **(done)** |
| **3B.2** | Gmail polling worker + `gmail.read` executor wiring + flip `gmail.read` to `implemented`. Polling kill-switch `FOMO_GMAIL_POLLING_ENABLED` default false. **(done)** |
| **3B.3** | Founder Real Gmail Smoke Test — prove the full OAuth + polling path against one real founder Gmail account with readonly scope only, persisted to real Neon Postgres. Adds `FOMO_GMAIL_POLLING_MAX_CYCLES` bounded-window cap, preflight + evidence scripts, runbook + report template. **Founder smoke run completed: `docs/SMOKE_REPORT_3B3.md` VERDICT=PASS (2026-05-23).** |
| **3C.1** | Ranker substrate — AnthropicBackend (Haiku 4.5 + Sonnet 4.6), versioned ranker prompt + JSON validator (`{ label, score, reason }`), pure `rankEmail()` function (egress-enforced), 20 synthetic anonymized fixtures, `runRankerEval(backend)` wiring. No worker integration, no kill switch, no real model call in CI. **(done)** |
| **3C.2** | **OpenAI ranker provider + smoke eval (founder directive 2026-05-24, supersedes Anthropic Haiku-vs-Sonnet bake-off).** Add OpenAIBackend behind the existing model router, run `gpt-5-mini` against the 20 synthetic fixtures, apply Conservative pass gate (precision≥0.85, recall≥0.85, json_valid≥0.95). PASS → 3C.3 unblocked; INVESTIGATE → blocked. **No Gmail-worker integration in this PR.** Scaffolding shipped (preflight + smoke-eval scripts + runbook + report template); **founder run still required** (sign-off via `docs/OPENAI_SMOKE_REPORT_3C2.md`) before Phase 3C.3 |
| 3C.3 | **DONE.** OpenAI ranker wired into the polling worker behind `FOMO_RANKER_ENABLED` (default false). `rank_results` table + store added; per-cycle counters + audit events surface ranker outcomes. Substrate wiring only — no smoke gate. |
| **3C.4** | **Founder real Gmail + real ranker smoke gate.** Scaffolding committed (preflight + evidence + runbook + report template + pnpm scripts). **Founder run still required** — sign-off via `docs/SMOKE_REPORT_3C4.md` PASS before Phase 3D begins. PASS gate is seam-works only (label correctness is a later, separate concern). |
| **3D.1** | **DONE.** Slack Candidate Review Posting — `SlackClient` + `slack.founder_review` executor + `alerts` table + worker integration behind `FOMO_SLACK_REVIEW_ENABLED` (default false). Outbound only; alerts sit at `queued_for_review`. risk_tier on `slack.founder_review` demoted `send → internal` to decouple from `FOMO_SEND_ENABLED`. Substrate wiring only — no smoke gate. |
| **3D.2** | **DONE.** Slack Approval Capture + Slack Smoke Test. Inbound `/slack/interactivity` route with HMAC signature verification + timestamp freshness check, captures Approve/Reject button clicks, transitions alert `queued_for_review → approved | rejected`, writes feedback events, edits the Slack card via `chat.update`. First-wins idempotency at the state-machine layer. Founder ran the smoke test on 2026-05-25 and committed `docs/SMOKE_REPORT_3D2.md` with `VERDICT: PASS` (PR #29 merged onto `main`). |
| **3E.1** | **DONE.** SendBlue Outbound Substrate — `SendBlueClient` + `sendblue.send_user_message` executor + deterministic founder-text template + outbound-sender worker + founder-phone allowlist + three-outcome semantics. Flipped `sendblue.send_user_message` to `implemented`. |
| **3E.2** | **DONE.** SendBlue Outbound Founder-Only Smoke Test. Founder ran the smoke 2026-05-26 and committed `docs/SMOKE_REPORT_3E2.md` PASS (PR #32 merged onto `main`). Real iMessage delivered to founder phone end-to-end. |
| **3F.1** | **DONE.** SendBlue Inbound Reply Substrate — signed `/sendblue/inbound` route + reply parser (deterministic STOP/START + OpenAI classifier with low-confidence fail-safe) + `inbound_replies` dedup table + `stop_active` memory signal + STOP enforcement in outbound-sender + sender-suppression on `ignore_sender` + snooze recording (no resurface in 3F.1). |
| **3F.2** | **SendBlue Inbound Reply Founder-Only Smoke Test.** Founder configures the SendBlue webhook against an ngrok tunnel, texts back to a real FOMO iMessage with a soft intent + STOP + START, commits `docs/SMOKE_REPORT_3F2.md` PASS. **Blocked on scaffolding PR; Phase 3G demo gate blocked until 3F.2 PASS is on `main`.** |
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

## Architecture Rule §4.8 — API-first, Browser-fallback, Approval-required

> Permanent Brevio architecture law (founder directive 2026-05-25). Companion to the seven MCP/agent rules — not separable, not optional, not a temporary note. Lives in [`FOMO_DESIGN.md §6 Rule 8`](../../FOMO_DESIGN.md) and [`FOMO_PLAN.md §4.8`](../../FOMO_PLAN.md). Every future Brevio capability beyond Gmail-read must satisfy it before it ships.

> **API first. Browser fallback only when sandboxed. User approval before final commitment.**
>
> Brevio may decide that a missing tool is needed, but it may not silently obtain access, silently use browser automation, or silently complete high-risk actions. The AI may propose; the system gates; the user approves.
>
> Mock tests prove code. Smoke tests prove reality. Every real-world capability needs a founder-only smoke test before it is trusted.

The kernel-level implementations of §4.8 in v0.1:

- Tool Registry + Permission Gate + risk_tier classification (`read` / `send` / `internal`)
- Kill switches default-off, strict opt-in: `FOMO_SEND_ENABLED`, `FOMO_AUTO_SEND_ENABLED`, `FOMO_FRIEND_BETA_ENABLED`, `FOMO_SLACK_REVIEW_ENABLED`, `FOMO_GMAIL_POLLING_ENABLED`
- Egress Policy (no raw body / headers / attachment filenames leave Brevio)
- Audit Log (sanitized detail; no payload-content columns)
- Slack founder-review checkpoint between rank and send (3D.1 + 3D.2)
- Founder-phone allowlist on outbound (3E.1) — `destinationFor(user_id)` returns the founder phone ONLY for `FOMO_FOUNDER_USER_ID`
- Founder-only smoke gates per real external integration (3B.3, 3C.2, 3C.4, 3D.2, 3E.2, 3F.x, 3G — see [`FOMO_PLAN.md §19 Smoke Test Gates`](../../FOMO_PLAN.md))
- No browser automation, no payments, no purchases, no bookings, no MCP plugin platform in v0.1 (forbidden by [`FOMO_PLAN.md §5`](../../FOMO_PLAN.md))

When Phase 4+ adds calendar / drafting / sending / payments / bookings / MCP tools / browser-fallback / delegated agents, the kernel-completeness gate must verify §4.8 still holds for every new capability *in the same PR* — no separate compliance phase, no "we'll wire approval later." API first; browser fallback only when sandboxed; user approval before final commitment.

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
