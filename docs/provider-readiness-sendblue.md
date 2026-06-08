# Provider Production Readiness — SendBlue (TIER 2)

Status: docs-only. No runtime changes proposed in this doc.
Scope: assess current SendBlue posture for v0.5.x friend beta, name what is known from code vs. founder smokes vs. what only the founder can verify from the SendBlue dashboard / support, and define decision triggers for any plan change.

Brevio core dimensions advanced by this doc: Dim 6 (Security / Permission Gates — provider-confirmation track), Dim 10 (Observability / Evals / Reliability), Dim 12 (User Trust / Consent). No dimensions regressed; HMR, PIL, Feedback, 3E.1 untouched.

---

## 1. Current known state

### 1.1 Known from code/repo

Cited from the current tree on branch `phase-v0.5.12-live-ranker-pil-guarded`.

| Capability | File | Notes |
|---|---|---|
| 3-outcome `SendBlueClient.send` (`sent` / `failed` / `send_status_unknown`) | [apps/fomo/src/adapters/sendblue/client.ts](apps/fomo/src/adapters/sendblue/client.ts) (lines ~93, 127, 207–380) | Outbound to `POST https://api.sendblue.co/api/send-message`. Auto-retry forbidden on `send_status_unknown` per founder directive 2026-05-25. |
| `registerContact` via `POST /api/v2/contacts` | [apps/fomo/src/adapters/sendblue/client.ts](apps/fomo/src/adapters/sendblue/client.ts) (lines ~382–509, constant `SENDBLUE_CONTACTS_URL` line 512) | Called from `/onboard/callback` after `provisionUser` + `setPhone`. 409 treated as already-registered. Failure does NOT roll back OAuth (v0.5.3 correction #1). |
| Webhook auth: timing-safe header equality, NOT HMAC | [apps/fomo/src/adapters/sendblue/client.ts](apps/fomo/src/adapters/sendblue/client.ts) `verifySendBlueWebhookSecret` (lines ~612–713) | Plain shared-secret header (default `sb-signing-secret`, overridable via env). No body-signature. No documented replay window — idempotency comes from `provider_message_id` UNIQUE on `inbound_replies`. |
| OPTED_OUT extraction (named safe fields only) | [apps/fomo/src/adapters/sendblue/client.ts](apps/fomo/src/adapters/sendblue/client.ts) `extractProviderError` (lines ~577–609), `SendBlueProviderError` type (lines ~113–125) | Surfaces `error_message`, `error_reason`, `error_code` only. `error_detail` (which may carry recipient prose) is intentionally dropped. Bounded lengths (≤64 / ≤16). |
| `ops:reconcile-sendblue` script + pure logic | [apps/fomo/src/adapters/sendblue/reconcile.ts](apps/fomo/src/adapters/sendblue/reconcile.ts), [apps/fomo/package.json](apps/fomo/package.json) script entry `ops:reconcile-sendblue` line 34 | Paginates `/api/v2/messages`, diffs against `audit_log`, audits `fomo.sendblue.delivery_gap_detected`. On-demand, NOT a periodic worker. |
| Inbound webhook route + kill switch | [apps/fomo/src/routes/sendblue-inbound.ts](apps/fomo/src/routes/sendblue-inbound.ts) (header doc lines 1–69), [apps/fomo/src/core/kill-switches.ts](apps/fomo/src/core/kill-switches.ts) (`FOMO_SENDBLUE_INBOUND_ENABLED`, line ~228) | Audit-before-auth; STOP/START deterministic; soft intents via classifier. Route NOT mounted when kill switch off ([apps/fomo/src/index.ts](apps/fomo/src/index.ts) line ~1257). |
| Env vars (fail-closed at boot) | [apps/fomo/src/index.ts](apps/fomo/src/index.ts) (lines ~440–500, 726–807, 1236–1260) | `SENDBLUE_API_KEY_ID`, `SENDBLUE_API_SECRET_KEY`, `SENDBLUE_FROM_NUMBER` (E.164), `SENDBLUE_WEBHOOK_SECRET`, `SENDBLUE_WEBHOOK_SECRET_HEADER` (default `sb-signing-secret`). |
| `memory_signals.sendblue_contact_status` gates outbound | [apps/fomo/src/dispatch/external-executors.ts](apps/fomo/src/dispatch/external-executors.ts) (line ~176 for client-wiring fail-closed message), see also v0.5.3 PASS memory for the `fomo.send.contact_not_registered` audit + `approved → failed` transition | Outbound worker refuses dispatch when `registered=false`. |

### 1.2 Known from founder smoke

| Observation | Source |
|---|---|
| v0.5.2 Morris webhook delivery gap — SendBlue accepted Morris's inbound STOP, server was down (Neon ECONNRESET) during the retry window, SendBlue's retries exhausted; gap was invisible for ~11h until manual `/api/v2/messages` query. | [[v05-2-pass]] §6 caveat |
| v0.5.3 retroactively detected the exact Morris "Hi" gap via `ops:reconcile-sendblue` (`fomo.sendblue.delivery_gap_detected` for handle `615FCE9D-635D-4D9F-99D5-E725C7CEBB33`). | [[v05-3-pass]] item #4 |
| v0.5.3 contact auto-registration smoke PASS — throwaway account onboarded with `+15550100099` → SendBlue returned HTTP 200 → `registered: true` auto-recorded. | [[v05-3-pass]] item #1 |
| v0.5.4 cross-tenant isolation PASS — Sheila onboarded; SendBlue contact auto-registered via v0.5.3 hardening; Morris row UNTOUCHED; founder row UNTOUCHED. | [[v05-4-pass]] |
| Founder phone accumulated OPTED_OUT state from v0.5.2–v0.5.6 testing; v0.5.3 drift detector observed firing in the wild (`fomo.send.opt_out_drift_detected`); v0.5.5 polling-after-STOP suppression blocked subsequent dispatch correctly. Blocks Path X smokes until cleared. | [[v05-3-pass]], [[v05-6-pass]] |
| `/sendblue/inbound` route, ngrok forward, secret match, audit-before-auth all confirmed clean during v0.5.2 — wall is provider-side, not our code. | [[sendblue-plan-gates]], [[v05-2-pass]] |

### 1.3 TO VERIFY (SendBlue dashboard / support)

These cannot be confirmed without dashboard / support / current pricing-page access. Treat any prior-audit claims as PLACEHOLDERS until the founder confirms.

| Question | Why it matters | Prior-audit claim (TO VERIFY, do NOT treat as confirmed) |
|---|---|---|
| What is the current account plan name in the dashboard? | Determines outbound + inbound posture for any new contact. | "Free Sandbox" per [[sendblue-plan-gates]] 2026-06-01. Verify still current. |
| Exact contact cap on current plan? | Drives the "BLOCK at Nth friend" trigger. | "10 verified contacts" on Free Sandbox per prior audit / [[sendblue-plan-gates]]. TO VERIFY against current SendBlue plan page. |
| Does the AI Agent tier actually remove the friend-must-text-first step? | Drives whether the onboarding-friction trigger requires plan upgrade or a different fix. | Prior audit asserted yes. TO VERIFY against current SendBlue plan page or support ticket. |
| Does the AI Agent tier deliver carrier-level opt-out (OPTED_OUT) webhooks proactively? | Drives founder-phone clearance procedure + whether drift-detector remains the only path. | Prior audit asserted yes. TO VERIFY. |
| AI Agent inbound volume cap? | Determines scaling headroom before another plan move. | Prior audit asserted "1000 inbound/day". TO VERIFY. |
| Exact AI Agent pricing? | Real cost the founder will pay. | Prior audit asserted "$100/mo/line". TO VERIFY against current SendBlue plan page. |
| Inbound webhook retry policy + SLA (attempts, intervals, max age)? | Determines residual gap risk when our server is unreachable. | Not documented in code or memory. TO VERIFY via support. |
| Migration steps Free Sandbox → AI Agent (downtime, sender-number portability, contact-state portability)? | Drives whether an upgrade window risks losing Morris/Sheila/founder grandfathered state. | Not documented. TO VERIFY via support before any upgrade. |
| OPTED_OUT clearance procedure for the founder phone? | Unblocks Path X smokes (currently substituted via Path Y per [[v05-6-pass]]). | Not documented. TO VERIFY via support. |
| Support response time + channel? | Drives realistic external wait budget for any of the above. | Prior audit unspecified. TO VERIFY against SendBlue support page. |
| Current SendBlue-side dashboard webhook URL + secret status (matches our env)? | Drift between dashboard config and our env is a silent failure mode. | Configured at v0.5.2 setup; drift unknown. TO VERIFY by re-reading dashboard. |

---

## 2. Unknowns (explicit list for founder confirmation)

The founder must confirm each item below before any plan change or growth decision. None of these can be confirmed by Claude.

1. Current plan name as shown in the SendBlue dashboard today.
2. Exact contact cap on the current plan, including whether existing grandfathered contacts (founder, Morris, Sheila) count against it.
3. Whether the AI Agent tier removes the friend-must-text-first step.
4. Whether the AI Agent tier delivers carrier-opt-out webhooks proactively (i.e., without us needing the drift-detector).
5. AI Agent inbound volume cap.
6. Exact current AI Agent pricing (per line, per month, any minimum commitment).
7. Inbound webhook retry policy and SLA (attempts, backoff intervals, max age before SendBlue drops a retry).
8. Migration steps from Free Sandbox to AI Agent: downtime window, sender-number portability, whether existing contact statuses survive.
9. Founder-phone OPTED_OUT clearance procedure and expected support turnaround.
10. SendBlue support response-time expectation (channel + business-hours posture).
11. Dashboard-side webhook URL + secret currently configured vs. our `SENDBLUE_WEBHOOK_SECRET` / `SENDBLUE_WEBHOOK_SECRET_HEADER` env values (drift check).
12. Whether SendBlue offers any sender-number health or reputation signal we should be polling.

---

## 3. Classification

| Issue | Severity today | Severity at scale | Notes |
|---|---|---|---|
| Contact ceiling on current plan | POLISH at ≤ 6 active contacts | BLOCK at 7th friend onboarding (assumes 10-cap; TO VERIFY) | Friends-beta is capped at 3 per [[three-friend-beta-cap]], so this is not a near-term blocker. |
| Friend-must-text-first onboarding step | FRICTION (one-time per friend on current tier) | Same | Hit visibly in v0.5.2 Morris briefing window. Not blocking with briefing; ugly without it. |
| No proactive carrier-opt-out webhook on current tier | FRICTION (drift-detector covers it) | FRICTION → POLISH if AI Agent provides it (TO VERIFY) | v0.5.3 drift detector + v0.5.5 suppression compose correctly per [[v05-6-pass]] live evidence. |
| Founder phone OPTED_OUT (accumulated v0.5.2–v0.5.6) | POLISH (one support ticket) | POLISH | Blocks Path X smokes; Path Y mock substitution working per [[v05-6-pass]]. |
| Webhook delivery reliability (provider-side dropped retries) | POLISH today | BLOCK at higher volume | v0.5.2 Morris gap was the canonical incident; v0.5.3 reconcile script detects, does not prevent. |

---

## 4. Exact actions (owner column)

### 4.1 Founder-only

| # | Action | Outcome |
|---|---|---|
| F1 | Open SendBlue dashboard. Read off current plan name, contact cap, contact count, sender-number reputation indicators. | Closes unknowns #1, #2, #11. |
| F2 | File a SendBlue support ticket asking: (a) does AI Agent remove friend-must-text-first, (b) does AI Agent deliver carrier-opt-out webhooks, (c) AI Agent inbound cap, (d) current pricing, (e) inbound webhook retry policy, (f) migration steps from Free Sandbox to AI Agent, (g) OPTED_OUT clearance for founder phone. | Closes unknowns #3–#10. |
| F3 | Confirm dashboard webhook URL + secret match our deployed `SENDBLUE_WEBHOOK_SECRET` and `SENDBLUE_WEBHOOK_SECRET_HEADER` env values. | Closes unknown #11. Surfaces silent-drift if any. |
| F4 | Purchase AI Agent tier — ONLY after F2 confirms it actually solves the trigger that motivated the purchase (see §6). | Reduces friction OR unblocks growth, depending on which trigger fired. Do NOT purchase speculatively. |

### 4.2 Claude-safe (TIER 2/3, docs-only this workflow)

| # | Action | Outcome |
|---|---|---|
| C1 | Audit current code paths against the env vars listed in §1.1 (already done in this doc). | Confirms code-side env contract is stable. |
| C2 | Verify `ops:reconcile-sendblue` script entry resolves to [apps/fomo/src/adapters/sendblue/reconcile.ts](apps/fomo/src/adapters/sendblue/reconcile.ts) and produces `fomo.sendblue.delivery_gap_detected` audit kind. | Confirms the on-demand reconciliation path still wires correctly. No runtime change. |
| C3 | Cross-reference `extractProviderError` allowlist against current `SendBlueProviderError` consumers to confirm the named-safe-fields contract has not regressed. | Privacy invariant unchanged. No runtime change. |
| C4 | Read this doc against [[sendblue-plan-gates]] every time the founder reports a friend-onboarding friction or new OPTED_OUT incident, and surface a TO VERIFY before designing a code fix. | Prevents over-coding when the cause is provider-side. |

Out of scope this workflow (per HARD boundaries): no runtime code, no purchase recommendation as if confirmed, no dashboard proposal, no new tools.

---

## 5. External wait time

| Step | Estimated external wait | Source |
|---|---|---|
| Dashboard read-off (F1) | Same-day, founder-controlled | n/a |
| Support ticket response (F2) | TO VERIFY against SendBlue support page (no SLA in our memory or code). Plan as if multi-business-day until confirmed. | Unknown #10 |
| Drift check (F3) | Same-day, founder-controlled | n/a |
| Plan upgrade activation, if approved (F4) | TO VERIFY (migration window, sender-number portability, contact-state survival). | Unknown #8 |
| OPTED_OUT founder-phone clearance | TO VERIFY (likely support-mediated). | Unknown #9 |

---

## 6. Decision triggers

### 6.1 Stay on current plan (default)

Stay on the current plan if ALL of the following hold:
- Active end-user count ≤ 6 (one slot below the assumed 10-cap, TO VERIFY).
- No proven, repeated webhook-delivery reliability incident beyond the v0.5.2 Morris case.
- Beta growth is staged (next decision is a polish/HMR phase, not Friend C).
- Founder-phone OPTED_OUT can stay mock-substituted per [[v05-6-pass]] Path Y.

### 6.2 Upgrade to AI Agent (only after verified facts)

Consider upgrade ONLY when at least one trigger fires AND F2 has confirmed AI Agent actually solves it:
- 8th end-user onboarding decision approaches AND the verified contact cap forces a move.
- A second proven carrier-opt-out incident occurs that drift-detector + reconcile cannot cleanly cover.
- Friend B-style onboarding friction blocks a strategic onboarding (e.g., a prospective partner the founder cannot ask to text-first).

Do NOT upgrade speculatively. Do NOT upgrade because "the audit says AI Agent unlocks X" — that claim is in the TO VERIFY bucket until F2 returns.

### 6.3 Move off SendBlue

Out of scope this doc. Capture as a future-architecture note only.

---

## 7. What NOT to build yet

- NO purchase of AI Agent tier until F2 confirms the claimed feature actually solves the fired trigger.
- NO periodic reconciliation worker. Keep `ops:reconcile-sendblue` on-demand until inbound volume sustains ≥ 50/day.
- NO custom contact-cycling logic (e.g., "auto-evict oldest contact when nearing cap"). The fix is plan-tier, not code.
- NO local SendBlue sandbox / fake provider for "fastest safe proof." Founder rule: build production-grade paths, not crutches.
- NO additional auth scheme on `/sendblue/inbound` (e.g., HMAC over body) until SendBlue documents one — current header-equality matches their public docs as of [code comment dated 2026-05-26](apps/fomo/src/adapters/sendblue/client.ts) lines 612–653.
- NO change to the `extractProviderError` allowlist; `error_detail` MUST stay dropped.
- NO auto-send. Outside this doc's scope and a separate gate.

---

## 8. Reference links

Code:
- [apps/fomo/src/adapters/sendblue/client.ts](apps/fomo/src/adapters/sendblue/client.ts)
- [apps/fomo/src/adapters/sendblue/reconcile.ts](apps/fomo/src/adapters/sendblue/reconcile.ts)
- [apps/fomo/src/routes/sendblue-inbound.ts](apps/fomo/src/routes/sendblue-inbound.ts)
- [apps/fomo/src/dispatch/external-executors.ts](apps/fomo/src/dispatch/external-executors.ts)
- [apps/fomo/src/core/kill-switches.ts](apps/fomo/src/core/kill-switches.ts)
- [apps/fomo/src/index.ts](apps/fomo/src/index.ts) (env wiring lines ~440–500, 726–807, 1236–1260)
- [apps/fomo/package.json](apps/fomo/package.json) script `ops:reconcile-sendblue` (line 34)

Memory:
- [[sendblue-plan-gates]]
- [[v05-2-pass]]
- [[v05-3-pass]]
- [[v05-3-hardening-scope]]
- [[v05-4-pass]]
- [[v05-6-pass]]
- [[three-friend-beta-cap]]
- [[no-gate-creep-on-extra-smokes]]
- [[real-or-absent-no-half-wired]]
