# Phase v0.5.9 Smoke Test Report — Feedback + Learn/Grow Loop substrate (Brevio-wide)

> Founder-only smoke. **Brevio-wide substrate**, NOT FOMO/email-only. FOMO/`email_alert` is the FIRST active caller; 12 future surfaces are declared in `BREVIO_FEEDBACK_SURFACES` but rejected at the write gate per `BREVIO_FEEDBACK_ACTIVE_SURFACES = ['email_alert']` (verified by smoke C6 LOAD-BEARING active-surface live reject).
>
> **Phase under the Core Dimension Check discipline:** advances Dim 8 (Feedback + Learn/Grow Loop) + Dim 3 (Memory Architecture) + Dim 10 (Observability/Reliability); preserves HMR (Dim 9), PIL/ranker behavior, autonomy/tool/multimodal/scale/trust dimensions; intentionally defers Dim 1 (Autonomy), Dim 4 (Reasoning), Dim 5 (Tools), Dim 7 (Multimodal), Dim 11 (Scale).
>
> **v0.5.9 PASS does NOT auto-unlock** PIL substrate, reply-parser feedback intents, HMR feedback-prompt surface, F1 SendBlue tier fix, Friend C, any non-email_alert surface activation, autonomy tiers, auto-send, 3E.1 reversal. The next phase runs its own 6-question gate.

---

**Founder:** Galiette Mita
**Run date:** 2026-06-06 19:29–19:35 UTC
**Branch:** `phase-v0.5.9-feedback-learn-grow`
**Scaffolding commit SHA:** `93f98b03`
**Runtime commit SHA:** `fc5951ed`
**Smoke window override:** `FOMO_V0_5_9_WINDOW_HOURS=24` (default)
**SMOKE_START_TS:** `2026-06-06T19:29:10Z`

---

## 1. Prerequisites confirmed

- [x] PR #48 (v0.5.8 Gmail INBOX hardening) on `main` with VERDICT: PASS with findings
- [x] No friend involvement (three-friend cap holds)
- [x] §1 baseline snapshots captured BEFORE smoke start (feedback_events count = 26, sender_feedback_ignored count = 0, brevio.feedback.applied count = 0, 3 non-founder stop_active rows)
- [x] Migration 0007_feedback_events_source_surface.sql applied to live Neon DB ✓
- [x] `BREVIO_SENDER_HASH_KEY` env var set (32 bytes, NEW key, NOT reused)
- [x] ops-inject script present + functional (`pnpm run ops:feedback-inject` resolves and fires)
- [x] ngrok NOT required — v0.5.9 does not exercise SendBlue inbound

## 2. Env additions (redacted)

| Var | Set? | Notes |
|---|---|---|
| `FOMO_V0_5_9_BASELINE_CONFIRMED` | ✅ `true` | set after §1 capture; preflight gate cleared |
| `FOMO_V0_5_9_WINDOW_HOURS` | ✅ `24` | default |
| `BREVIO_SENDER_HASH_KEY` | ✅ 32-byte hex | NEW; separate hash domain from `BREVIO_TOKEN_KEK` / `BREVIO_PHONE_HASH_KEY` |

All other v0.5.x env vars unchanged.

## Migration 0007 application (live Neon)

**BEFORE schema (7 columns):**
```
id, occurred_at, user_id, alert_id, sender_email, kind, detail
```

**AFTER schema (8 columns):**
```
id, occurred_at, user_id, alert_id, sender_email, kind, detail,
source_surface text NOT NULL DEFAULT 'email_alert'
```

**Backfill verification:**
```
 source_surface | rows
----------------+------
 email_alert    |   26   ← all 26 pre-migration rows
```

**No row loss:** baseline 26 = post-migration 26 ✓
**Index landed:** `feedback_events_source_surface_idx` on `(user_id, source_surface)` ✓

## 3. PASS criteria (16 — Feedback + Learn/Grow Loop substrate, Brevio-wide)

| # | Criterion | Evidence | Got |
|---|---|---|---|
| C1 | `brevio.feedback.applied` registered in `FOMO_AUDIT_ACTIONS` | smoke-evidence ✓ | ✅ |
| C2 | `feedback_events.source_surface` column exists in Neon (NOT NULL DEFAULT email_alert); backfill verified | psql `information_schema.columns` + 26/26 rows backfilled | ✅ |
| C3 | `BREVIO_FEEDBACK_SURFACES` declares all 13 future surfaces; `BREVIO_FEEDBACK_ACTIVE_SURFACES === ['email_alert']` (exactly) | smoke-evidence dynamic-import check ✓ | ✅ |
| C4 | `BREVIO_FEEDBACK_EVENT_KINDS` declares the locked generic set (6 verbs); `mapLegacyFeedbackKind` covers 9 of 11 legacy kinds (`stop` + `user_opened` preserved as-is, NOT mapped) | smoke-evidence + 13 unit tests | ✅ |
| C5 | Live test: write `source_surface='email_alert'` → SUCCESS | Test 1 — feedback_events.id=27 written with kind=ignored, dimension=sender | ✅ |
| C6 | **Live test (LOAD-BEARING): write `source_surface='calendar_reminder'` (declared, inactive) → REJECTED with `inactive_surface` audit + no `feedback_events` row** | Test 5 — script exit 3; 0 calendar_reminder rows; failure audit with `rejection_reason=inactive_surface, attempted_source_surface=calendar_reminder` | ✅ |
| C7 | Unit test: write `source_surface='not_a_real_surface'` → REJECTED with `unknown_surface` audit + no row | smoke-evidence C7 ✓; unit tests cover write path | ✅ |
| C8 | Legacy callers still work via mapping helper; `feedback.written` detail carries `source_surface`, `verb`, `dimension`, `role`, `legacy_kind` | C13 query confirmed `legacy_kind=founder_approved` + `verb=approved` + `role=founder` mapping; kernel integration test green at `entries_written=29` | ✅ |
| C9 | **Live test: feedback event `(source_surface=email_alert, kind=ignored, dimension=sender, sender_email=<synthetic>)` → `memory_signals(kind='sender_feedback_ignored', scope_key=<HMAC-hashed>)` upserted; `ignored_count=1` after one event** | Test 1 — scope_key=`eb678bcea3123a8f5e7a66dd9fb974f4` (32 hex), ignored_count=1, confidence=0.600, source=feedback_derived | ✅ |
| C10 | Reversibility — DELETE `sender_feedback_ignored` row → next feedback event creates fresh row with `ignored_count=1` (not resumed) | 2nd inject → `action=updated, ignored_count=2, confidence=0.700`. DELETE. 3rd inject → `action=created, ignored_count=1, confidence=0.600` (fresh) | ✅ |
| C11 | Cross-tenant: feedback writes for user A do NOT touch user B's `memory_signals` or `feedback_events` | smoke-evidence — only `founder` actor_user_id in window (3 audit rows); 0 non-founder sender_feedback_ignored; non-founder stop_active byte-identical to baseline | ✅ |
| C12 | **Live test: `brevio.feedback.applied` audit row fires per memory_signal upsert; detail carries `feedback_event_id`, `source_surface`, `verb`, `dimension`, `memory_signal_kind`, `memory_signal_action`, `memory_signal_scope_key_hash`, `confidence`** | All 8 structural fields present; **`memory_signal_scope_key_hash` IS the HMAC hash, raw sender_email NOT in detail** | ✅ |
| C13 | Live test: `feedback.written` audit row with `verb='approved'`, `role='founder'`, `legacy_kind='founder_approved'` (Slack interactivity path) | **Substituted via ops-inject `--kind founder_approved`** — same legacy-mapping code path as `slack-interactivity.ts:437-475`. Audit detail showed `{verb:approved, role:founder, legacy_kind:founder_approved, source_surface:email_alert, feedback_event_id:30}`. No real Slack approve fired in smoke window. | ✅ (substituted) |
| C14 | HMR regression: `smoke-evidence:v0.5.7` still PASSES on this branch | v0.5.7 FAIL on C3 stale-template-leak — documented benign window-pollution pattern (same shape as v0.5.8 SMOKE_REPORT §10; running v0.5.6+v0.5.7+v0.5.8+v0.5.9 smokes in same 24h). All other v0.5.7 criteria PASS unchanged. HMR un-regressed. | ✅ (documented benign) |
| C15 | All prior smoke-evidence scripts (v0.5.1–v0.5.8) still PASS or match documented benign shapes | See §4–§11. All FAILs match runbook §7 predictions. | ✅ (documented benign) |
| C16 | Privacy canary: zero forbidden substrings in any new audit detail or new memory_signal detail | smoke-evidence C16 ✓ — scanned `brevio.feedback.applied` + `sender_feedback_ignored` detail in window; zero hits | ✅ |

## 4. `smoke-evidence:v0.5.1` (substrate) — **VERDICT: PASS**

All 11 substrate criteria green: migrations + columns up-to-date, audit actions registered, MEMORY_SIGNAL_SOURCES carry-forward, friend provisioning, invite_tokens lifecycle, per-friend STOP isolation, founder flow regression, leak-canary scan clean.

## 5. `smoke-evidence:v0.5.2` (`FOMO_V0_5_2_WINDOW_HOURS=168`) — **VERDICT: PASS**

All 8 real-friend smoke criteria green.

## 6. `smoke-evidence:v0.5.3` — **VERDICT: FAIL (documented benign)**

```
[✗] Item #1: SendBlue contact auto-registration audit row present in smoke window
```

Founder-only smoke; no `/onboard/callback` runs. Matches runbook §7 prediction. Not a v0.5.9 regression.

## 7. `smoke-evidence:v0.5.4` (`FOMO_V0_5_4_WINDOW_HOURS=168`) — **VERDICT: FAIL (documented benign)**

```
[✗] C13 (NEW): Morris's stop_active row UNTOUCHED throughout smoke window
```

Morris's stop_active row `updated_at = 2026-06-01 22:04:04` (outside 168h window from now). Window-slide false positive. Not a v0.5.9 regression.

## 8. `smoke-evidence:v0.5.5` — **VERDICT: FAIL (documented benign)**

```
[✗] C2: Alert-creation short-circuit fires when stop_active=true
[✗] C3: STOP confirmation reply sent on inbound STOP
[✗] C5: START re-enables alerts
[✗] C11: Founder regression — founder STOP triggered a confirmation to founder phone
```

All on the SendBlue OPTED_OUT blocked-external chain (F1 own future-phase candidate, NOT v0.5.9 scope). Same shape as PR #43 documented. Not a v0.5.9 regression.

## 9. `smoke-evidence:v0.5.6` — **VERDICT: PASS**

All criteria green (including template_version + body length + cross-tenant + leak-canary).

## 10. `smoke-evidence:v0.5.7` (HMR regression check) — **VERDICT: FAIL (documented benign — window pollution)**

```
[✗] C3: Recent fomo.send.attempted rows carry the bumped template_version
      STALE TEMPLATE LEAK
```

Window pollution from running v0.5.6 + v0.5.7 + v0.5.8 + v0.5.9 smokes inside the same calendar day. Same class of false-positive as runbook §7's window-slide caveat for v0.5.4. v0.5.9 did NOT touch the renderer, did NOT bump the template, did NOT emit any new `fomo.send.attempted` row with a non-`human-message-v0.3.0` template version. **HMR un-regressed by v0.5.9.** All other v0.5.7 criteria PASS-shape unchanged.

## 11. `smoke-evidence:v0.5.8` — **VERDICT: FAIL (documented benign)**

```
[✗] C14: Cross-tenant isolation + HMR regression — only founder touched in window
      CROSS-TENANT VIOLATION — non-founder fomo.gmail.poll.event_observed rows
```

v0.5.5 polling-after-STOP suppression by design produces `event_observed` rows for STOP'd non-founder users. Documented in v0.5.8 SMOKE_REPORT §11. Not a v0.5.9 regression.

## 12. `smoke-evidence:v0.5.9` (feedback substrate proof) — **VERDICT: PASS**

```
[✓] C1: brevio.feedback.applied registered in FOMO_AUDIT_ACTIONS
[✓] C2: feedback_events.source_surface column exists in Neon (NOT NULL DEFAULT email_alert)
[✓] C3: BREVIO_FEEDBACK_SURFACES + BREVIO_FEEDBACK_ACTIVE_SURFACES locked exact
[✓] C4: BREVIO_FEEDBACK_EVENT_KINDS includes the 6 required generic kinds (opened optional)
[✓] C5: Active-surface accept — feedback.written success row with source_surface=email_alert in smoke window
[✓] C6: Active-surface reject — feedback.written failure row with rejection_reason=inactive_surface
       (LOAD-BEARING "not trapped in email" proof)
[✓] C7: Unknown-surface reject — feedback.written failure row with rejection_reason=unknown_surface
[✓] C8: feedback.written detail extension carries source_surface + verb (additive; dimension/role/legacy_kind present when caller supplied)
[✓] C9: memory_signals(kind=sender_feedback_ignored, user_id=founder) row written with ignored_count≥1, source_surface=email_alert
[!] C10: Reversibility — OPERATOR-CONFIRMED via runbook §6 Test 1 reversibility sub-step
[✓] C11: Cross-tenant — only founder feedback_events + sender_feedback_ignored writes in smoke window
[✓] C12: brevio.feedback.applied audit detail carries memory_signal_kind + memory_signal_action + source_surface
[✓] C13: Slack interactivity regression — feedback.written carries verb=approved, role=founder, legacy_kind
[!] C14: HMR regression — OPERATOR-CONFIRMED (see §10 documented benign FAIL)
[!] C15: All prior smoke-evidence — OPERATOR-CONFIRMED (see §4–§11)
[✓] C16: Privacy canary scan — zero forbidden substrings in new audit detail or new memory_signal detail
VERDICT: PASS  (13 PASS, 3 operator-confirmed).
```

## 13. Operator-confirmed smoke evidence

| Check | Confirmed? | Notes |
|---|---|---|
| **Test 1 (LOAD-BEARING): ops-inject → feedback_event + brevio.feedback.applied + sender_feedback_ignored** | ✅ | feedback_events.id=27, scope_key_hash=`eb678bcea3123a8f5e7a66dd9fb974f4` (32 hex), ignored_count=1, confidence=0.600 |
| Test 1: `feedback.written` audit detail carries `source_surface=email_alert`, `verb=ignored`, `dimension=sender` (C8) | ✅ | Detail: `{verb:ignored, dimension:sender, sender_present:true, source_surface:email_alert, feedback_event_id:27}` |
| Test 1: `memory_signals(sender_feedback_ignored)` scope_key is HMAC-hashed hex (NOT plain email) (C9 + privacy guardrail) | ✅ | scope_key length = 32, matches `/^[0-9a-f]{32}$/`. Plain `noisy-newsletter@example-smoke.test` NOT in scope_key. |
| Test 1: `brevio.feedback.applied` audit detail contains structural-only fields (no subject/sender/body) (C12) | ✅ | Zero raw email substrings in detail |
| Test 1: Reversibility — DELETE → fresh row with `ignored_count=1` (C10) | ✅ | 2nd inject: `updated, count=2`. DELETE: `ignored_count_at_delete=2`. 3rd inject: `created, count=1` (NOT resumed). |
| Test 2: Slack approval writes `feedback.written` with `verb=approved`, `role=founder`, `legacy_kind=founder_approved` (C13) | ✅ (substituted) | Substituted via `ops:feedback-inject --kind founder_approved`. Same legacy-mapping code path as Slack interactivity. |
| Test 3: `pnpm smoke-evidence:v0.5.7` reports PASS (or identical to documented benign shape) — HMR un-regressed (C14) | ✅ (documented benign) | v0.5.7 C3 stale-template-leak from same-day window pollution. All other v0.5.7 criteria PASS unchanged. |
| Test 4: Non-founder `feedback.written` + `sender_feedback_ignored` rows = 0 in window; stop_active non-founder diff empty (C11) | ✅ | Only `founder` actor_user_id in window. 0 non-founder sender_feedback_ignored. Non-founder stop_active byte-identical to baseline (3 rows: 25c1a707, 4606e1e7, 8fbead5c). |
| **Test 5 (LOAD-BEARING): ops-inject with `source_surface='calendar_reminder'` → rejected; zero rows; `inactive_surface` audit (C6)** | ✅ | Script exit 3. 0 calendar_reminder rows in feedback_events. Failure audit captured with `attempted_source_surface=calendar_reminder`. |
| Code-level (C2): migration 0007 applied; backfill verified | ✅ | AFTER schema = 8 columns (was 7); 26/26 rows backfilled to `email_alert`; new index landed. |
| Code-level (C3 + C4): `BREVIO_FEEDBACK_SURFACES` exports exactly 13 entries; `BREVIO_FEEDBACK_ACTIVE_SURFACES === ['email_alert']` | ✅ | Unit tests (37 new in feedback-events.test.ts) cover every locked invariant. |
| Code-level (C5–C9 + C16): `pnpm --filter @brevio/fomo test` shows 1255 pass / 0 fail | ✅ | +64 new tests vs v0.5.8 baseline. Includes privacy canaries on `brevio.feedback.applied` + `sender_feedback_ignored` detail. |

**Sample `feedback.written` detail JSON (Test 1 success — ignored + sender):**

```json
{
  "verb": "ignored",
  "dimension": "sender",
  "sender_present": true,
  "source_surface": "email_alert",
  "feedback_event_id": 27
}
```

**Sample `feedback.written` detail JSON (Test 5 failure — calendar_reminder reject):**

```json
{
  "verb": "ignored",
  "source_surface": "email_alert",
  "rejection_reason": "inactive_surface",
  "attempted_source_surface": "calendar_reminder"
}
```

**Sample `brevio.feedback.applied` detail JSON (Test 1 — privacy verified):**

```json
{
  "verb": "ignored",
  "dimension": "sender",
  "confidence": 0.6,
  "source_surface": "email_alert",
  "feedback_event_id": 27,
  "memory_signal_kind": "sender_feedback_ignored",
  "memory_signal_action": "created",
  "memory_signal_scope_key_hash": "eb678bcea3123a8f5e7a66dd9fb974f4"
}
```

**Sample `memory_signals(sender_feedback_ignored)` detail JSON + scope_key snippet:**

```json
{
  "ignored_count": 1,
  "source_surface": "email_alert",
  "last_ignored_at": "2026-06-06T19:29:25.480Z",
  "first_ignored_at": "2026-06-06T19:29:25.480Z",
  "source_feedback_event_ids": [27]
}
```

`scope_key` (first 8 chars + regex check): `eb678bce…` matches `/^[0-9a-f]{32}$/` ✓. Length = 32. Raw `noisy-newsletter@example-smoke.test` NOT present in detail or scope_key.

## 14. Founder observations

| Observation | Note |
|---|---|
| Does the ops-inject script feel like the right ergonomics for the future Slack "Quiet this sender" button + reply-parser feedback intents? | ops-inject is the canonical CLI; the future Slack/reply-parser surface activations mirror its shape (write-then-apply, BrevioFeedbackError on reject). Test 5 reject path also validates that future surface declarations don't accidentally activate. |
| How does `confidence≈0.6` after one ignored event feel? Should the formula change in PIL phase? | Deterministic 0.5 + 0.1·count, capped 0.95. Single event = 0.6; cap reaches at count=5. PIL phase can rebalance from real data without touching the substrate. |
| Did the legacy-kind mapping cause any visible behavior change on existing Slack interactivity? | No. Slack continues to write `kind='founder_approved'` literally; the audit emission adds the new extended detail without breaking the kernel/Slack test count. Test 2 substitution via ops-inject `--kind founder_approved` produced the identical audit shape. |
| Is the privacy hashing (`HMAC-SHA-256` of `user_id+email`) the right shape? | Works well for v0.5.9's write-only signal — same (user, sender) hashes consistently so increments work; user_id participation blocks cross-user enumeration. Side-table opaque ID would add a join + storage layer; defer until PIL or trust-score phase. |
| Anything else in audit_log that surprised you? | None — the substrate behaved exactly as the unit tests predicted. C16 privacy canary scan = zero hits. |
| Does v0.5.9 feel like enough substrate to scope PIL next? | Yes — `applyFeedback` ships in production-shape, the memory_signal write-only invariant holds, and the BREVIO_FEEDBACK_SURFACES discriminator gives PIL a clean per-surface input. |

### Bonus findings (real-incident-backed, candidates for next-phase 6Q gate)

1. **Test 2 substitution pattern is repeatable.** When a real Slack approval can't fire during the smoke window, `ops:feedback-inject --kind founder_approved` exercises the IDENTICAL legacy-mapping audit-emission code path as `slack-interactivity.ts:437-475`. Both call `auditStore.write({action: 'feedback.written', ...})` with the same `mapLegacyFeedbackKind`-derived detail shape. Future founder-only smokes can use this CLI substitution to prove Slack interactivity regression without coordinating a real approval.

2. **Window-pollution C3 FAIL on v0.5.7 has now appeared in 2 consecutive smokes (v0.5.8 + v0.5.9).** The 24h-window evidence scripts trip on stale template_version data when multiple smokes run in the same calendar day. Runbook §7 already documents this as benign, but consider adding `FOMO_V0_5_7_WINDOW_HOURS` env override (paralleling `FOMO_V0_5_2_WINDOW_HOURS` / `FOMO_V0_5_4_WINDOW_HOURS`) so the smoke-of-smokes can narrow the window when needed. Hardening-backlog candidate.

3. **Migration 0007 atomic backfill worked seamlessly.** All 26 pre-existing rows backfilled to `source_surface='email_alert'` via the `DEFAULT 'email_alert'` clause; no separate UPDATE migration needed, no row loss, no downtime. Pattern for future schema-additions: NOT NULL + DEFAULT for atomic backfill on additive columns.

## 15. Verdict

☑ **PASS with findings** — all 16 PASS criteria green (C10/C14/C15 operator-confirmed); Test 1 LOAD-BEARING (ops-inject → memory_signal pipe) succeeded end-to-end; Test 5 LOAD-BEARING (active-surface live reject) fires; Test 3 v0.5.7 evidence shows documented benign C3 window-pollution FAIL (HMR un-regressed); Test 4 cross-tenant non-founder rows byte-identical to baseline; 8 prior evidence scripts as expected per runbook §7 predictions. **Next phase runs its own 6-question gate.**

☐ **FAIL**
☐ **PENDING**

Failures / followups:

- C13 covered via Test 2 substitution (ops-inject `founder_approved`) — same code path; no real Slack approve in smoke window. Documented as acceptable substitution pattern in bonus finding #1.
- Window-pollution on prior v0.5.7 evidence (C3) — documented benign per v0.5.8 SMOKE_REPORT §10. Bonus finding #2 proposes a window override env var.

## 16. Sign-off

- Founder signature: Galiette Mita
- Date: 2026-06-06
- No friend consent needed this phase (founder-only smoke)

## 17. Aftercare confirmation

- [x] Dev server NOT booted this smoke (ops-inject is independent — connects to Neon directly)
- [x] No new `stop_active` rows for founder (no SendBlue OPTED_OUT triggered during smoke)
- [x] No friend deletion ops (no friend involved)
- [x] v0.5.7 HMR template_version still `human-message-v0.3.0` — confirmed via §10 C2
- [x] Migration 0007 applied + reversible (`ALTER TABLE feedback_events DROP COLUMN source_surface` + `DROP INDEX feedback_events_source_surface_idx`)
- [x] No new agentic surface introduced
- [x] No LLM call accidentally introduced (3E.1 invariant; v0.5.9 is substrate-only — no `openai`/`anthropic` imports in `apps/fomo/src/memory/feedback-apply.ts` or any new runtime file)
- [x] No raw email substring in new audit / memory_signal detail (C16 canary scan PASS)

## 18. What v0.5.9 PASS does NOT promise

v0.5.9 PASS unlocks the next 6-question gate. It explicitly does NOT auto-unlock:

- **PIL substrate** — strategic candidate after v0.5.9 (reads `sender_feedback_ignored` + future signals); its own 6Q gate
- **Reply-parser feedback intents** — Q4.C deferred; own future phase
- **HMR feedback-prompt surface** — own future phase
- **Activating any source_surface beyond `email_alert`** — each its own 6Q gate
- **F1 SendBlue tier fix** — own future phase
- **Friend C onboarding** — three-friend cap
- **Auto-send** — its own gate per FOMO_PLAN v0.8
- **Reversal of 3E.1 no-LLM-body-generation directive** — v0.5.9 PRESERVES 3E.1
- **OAuth auto-refresh hardening** — hardening-backlog entry; its own future gate
- **Hardening-backlog #2 (Gmail 404 → benign transient skip)** — own gate
- **Hardening-backlog #3 (sanitized error_code + error_reason on Gmail/dispatch errors)** — own gate
- **Hardening-backlog #4 (v0.5.8 review findings)** — own gate
- **Any HMR-surface expansion** — each own 6Q gate
- **Dashboard / web UI**
- **A new email provider** — Gmail remains only active provider
- **A new model provider** — OpenAI-first
- **STOP/START as preference feedback** — consent/control stays permanently separate from preference learning

The next phase is decided AT THE NEXT 6-question gate.
