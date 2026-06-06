# Phase v0.5.9 Smoke Test Report — Feedback + Learn/Grow Loop substrate (Brevio-wide)

> Filled after running every step in `smoke-test-v0.5.9-feedback-substrate.md`.
> Commit as `docs/SMOKE_REPORT_v0.5.9.md` once **`VERDICT: PASS`** on all 16 criteria AND §6 Test 1 ops-inject produced a `sender_feedback_ignored` memory_signal upsert with `brevio.feedback.applied` audit AND Test 5 active-surface live reject fires AND Test 3 confirms `smoke-evidence:v0.5.7` still PASSES on this branch.
>
> **Scaffolding-vs-runtime note:** if this report is filled before the runtime implementation commit lands, `smoke-evidence:v0.5.9` will print `VERDICT: PENDING` (not PASS). That is expected at scaffolding time. The report can only legitimately reach `VERDICT: PASS` after both the SCAFFOLDING commit and the RUNTIME commit are on this branch AND migration 0007 is applied AND the §6 test sequence has been run end-to-end.
>
> **Phase under the Core Dimension Check discipline:** advances Dim 8 (Feedback + Learn/Grow Loop) + Dim 3 (Memory Architecture) + Dim 10 (Observability/Reliability); preserves HMR (Dim 9), PIL/ranker behavior (Dim 9 + Dim 4 intent), autonomy/tool/multimodal/scale/trust dimensions; intentionally defers Dim 1 (Autonomy), Dim 4 (Reasoning), Dim 5 (Tools), Dim 7 (Multimodal), Dim 11 (Scale). See [scope memory](../.claude/projects/-Users-galiettemita-Downloads-Executive-AI-Agent-backend/memory/project_v05-9-scope.md).
>
> **v0.5.9 PASS does NOT auto-unlock** PIL substrate, reply-parser feedback intents, HMR feedback-prompt surface, F1 SendBlue tier fix, Friend C, any non-email_alert surface activation, autonomy tiers, auto-send, 3E.1 reversal. The next phase runs its own 6-question gate.

---

**Founder:** Galiette Mita
**Run date:** _<YYYY-MM-DD HH:MM TZ>_
**Branch:** `phase-v0.5.9-feedback-learn-grow`
**Scaffolding commit SHA:** _<sha>_
**Runtime commit SHA:** _<sha>_
**Smoke window override (if any):** `FOMO_V0_5_9_WINDOW_HOURS=24` (default)
**SMOKE_START_TS:** _<UTC ISO timestamp>_

---

## 1. Prerequisites confirmed

- [ ] PR #48 (v0.5.8 Gmail INBOX hardening) on `main` with VERDICT: PASS with findings
- [ ] No friend involvement (three-friend cap holds)
- [ ] §1 baseline snapshots captured BEFORE smoke start (all four: feedback_events count, sender_feedback_ignored count = 0, brevio.feedback.applied count = 0, stop_active rows)
- [ ] Migration 0007_feedback_events_source_surface.sql applied to live Neon DB
- [ ] `BREVIO_SENDER_HASH_KEY` env var set (32 bytes, NEW key, NOT reused)
- [ ] ops-inject script present (lands in runtime commit; `pnpm --filter @brevio/fomo run ops:feedback-inject` resolves)
- [ ] ngrok NOT required — v0.5.9 does not exercise SendBlue inbound

## 2. Env additions (redacted)

| Var | Set? | Notes |
|---|---|---|
| `FOMO_V0_5_9_BASELINE_CONFIRMED` | ☐ | set `true` after §1 capture |
| `FOMO_V0_5_9_WINDOW_HOURS` | ☐ | default 24 |
| `BREVIO_SENDER_HASH_KEY` | ☐ | NEW: 32-byte HMAC key for scope_key derivation. Generate via `openssl rand -base64 32`. NEVER reuse from BREVIO_TOKEN_KEK / BREVIO_PHONE_HASH_KEY (separate hash domain). |

All other v0.5.4 / v0.5.5 / v0.5.6 / v0.5.7 / v0.5.8 env vars unchanged.

## 3. PASS criteria (16 — Feedback + Learn/Grow Loop substrate, Brevio-wide)

| # | Criterion | Evidence | Got |
|---|---|---|---|
| C1 | `brevio.feedback.applied` registered in `FOMO_AUDIT_ACTIONS` | _<preflight + smoke-evidence output>_ | ☐ |
| C2 | `feedback_events.source_surface` column exists in Neon (NOT NULL DEFAULT email_alert); backfill verified | _<psql \\d output + GROUP BY source_surface>_ | ☐ |
| C3 | `BREVIO_FEEDBACK_SURFACES` declares all 13 future surfaces; `BREVIO_FEEDBACK_ACTIVE_SURFACES === ['email_alert']` (exactly) | _<grep + smoke-evidence>_ | ☐ |
| C4 | `BREVIO_FEEDBACK_EVENT_KINDS` declares the locked generic set; `mapLegacyFeedbackKind` covers 10 mappable legacy kinds (`stop` preserved as-is, NOT mapped) | _<unit test run output>_ | ☐ |
| C5 | Unit + live test: write `source_surface='email_alert'` → SUCCESS | _<unit test + Test 1 query>_ | ☐ |
| C6 | **Live test (LOAD-BEARING): write `source_surface='calendar_reminder'` (declared, inactive) → REJECTED with `inactive_surface` audit + no `feedback_events` row** | _<Test 5 output>_ | ☐ |
| C7 | Unit test: write `source_surface='not_a_real_surface'` → REJECTED with `unknown_surface` audit + no row | _<unit test run output>_ | ☐ |
| C8 | All 11 legacy `FEEDBACK_EVENT_KINDS` callers still work via mapping helper (kernel integration test + Slack interactivity test green; `feedback.written` detail carries `source_surface`, `verb`, `dimension`, `role`, `legacy_kind`) | _<kernel test + Test 2 query>_ | ☐ |
| C9 | **Live test: feedback event `(source_surface=email_alert, kind=ignored, dimension=sender, sender_email=<synthetic>)` → `memory_signals(kind='sender_feedback_ignored', scope_key=<HMAC-hashed>)` upserted; `ignored_count=1` after one event** | _<Test 1 Query 3>_ | ☐ |
| C10 | Unit + live test: Reversibility — DELETE `sender_feedback_ignored` row → next feedback event creates fresh row with `ignored_count=1` (not resumed) | _<Test 1 reversibility sub-step>_ | ☐ |
| C11 | Cross-tenant: feedback writes for user A do NOT touch user B's `memory_signals` or `feedback_events` (carry-forward [[multitenant-design-principles]]) | _<Test 4 queries>_ | ☐ |
| C12 | **Live test: `brevio.feedback.applied` audit row fires per memory_signal upsert; detail carries `feedback_event_id`, `source_surface`, `verb`, `dimension`, `memory_signal_kind`, `memory_signal_action`, `confidence`** | _<Test 1 Query 4>_ | ☐ |
| C13 | Live test: founder approves existing Slack card → `feedback.written` audit row carries new `source_surface='email_alert'` + `verb='approved'` + `role='founder'` + `legacy_kind='founder_approved'` detail fields | _<Test 2 query>_ | ☐ |
| C14 | HMR regression: `smoke-evidence:v0.5.7` still PASSES on this branch (no v0.5.9 regression on renderer/ranker/HMR audit fields) | _<v0.5.7 verdict on this branch>_ | ☐ |
| C15 | All prior smoke-evidence scripts (v0.5.1–v0.5.8) still PASS or match documented benign shapes | _<8 prior verdicts>_ | ☐ |
| C16 | Privacy canary: zero forbidden substrings (Subject:, From:, body fragments, @gmail.com / @icloud.com / @hotmail.com / @yahoo.com) in any new audit detail or new memory_signal detail across smoke window | _<smoke-evidence C16 scan>_ | ☐ |

## 4. `smoke-evidence:v0.5.1` output (substrate) — **VERDICT: _<…>_**

```
_<paste full output>_
```

## 5. `smoke-evidence:v0.5.2` output (`FOMO_V0_5_2_WINDOW_HOURS=168` if needed) — **VERDICT: _<…>_**

```
_<paste full output>_
```

## 6. `smoke-evidence:v0.5.3` output — **VERDICT: _<…>_**

```
_<paste full output>_
```

> Operator note: v0.5.9 is a founder-only smoke — no `/onboard/callback` runs. If v0.5.3 reports Item #1 FAIL, that's expected blocked-substrate, NOT a v0.5.9 regression.

## 7. `smoke-evidence:v0.5.4` output (`FOMO_V0_5_4_WINDOW_HOURS=168`) — **VERDICT: _<…>_**

```
_<paste full output>_
```

> Operator note: if v0.5.4 reports C13/C14 window-slide FAIL shape that prior SMOKE_REPORTs documented, that's NOT a v0.5.9-caused regression.

## 8. `smoke-evidence:v0.5.5` output — **VERDICT: _<…>_**

```
_<paste full output>_
```

> Operator note: if v0.5.5 reports C2/C3/C11 SendBlue OPTED_OUT blocked-external FAIL — same shape PR #43 documented — that's NOT a v0.5.9-caused regression.

## 9. `smoke-evidence:v0.5.6` output — **VERDICT: _<…>_**

```
_<paste full output>_
```

## 10. `smoke-evidence:v0.5.7` output (HMR regression check) — **VERDICT: _<…>_**

```
_<paste full output>_
```

> Operator note (LOAD-BEARING for C14): if v0.5.7 reports a NEW failure that PR #46's SMOKE_REPORT did not have, **v0.5.9 has regressed HMR — STOP and fix before merging.** Acceptable: identical shape to PR #46 PASS, OR the documented benign FAIL pattern from v0.5.8 SMOKE_REPORT §10 (window-pollution from multiple smokes in same 24h).

## 11. `smoke-evidence:v0.5.8` output — **VERDICT: _<…>_**

```
_<paste full output>_
```

> Operator note: v0.5.8 C14 may FAIL due to v0.5.5 STOP-suppressed polling producing non-founder event_observed rows (preserved-by-design; see v0.5.8 SMOKE_REPORT §11). Not a v0.5.9 regression.

## 12. `smoke-evidence:v0.5.9` output (feedback substrate proof) — **VERDICT: _<…>_**

```
_<paste full output>_
```

## 13. Operator-confirmed smoke evidence

| Check | Confirmed? | Notes |
|---|---|---|
| **Test 1 (LOAD-BEARING): ops-inject → feedback_event + brevio.feedback.applied + sender_feedback_ignored** | ☐ | _<SMOKE_START_TS, audit row IDs, memory_signal scope_key sample (first 8 hex chars), confidence>_ |
| Test 1: `feedback.written` audit detail carries `source_surface=email_alert`, `verb=ignored`, `dimension=sender`, `role=user` (C8) | ☐ | _<sample row JSON>_ |
| Test 1: `memory_signals(sender_feedback_ignored)` scope_key is HMAC-hashed hex (NOT plain email) (C9 + privacy guardrail) | ☐ | _<scope_key character length + regex /^[0-9a-f]{32}$/ check>_ |
| Test 1: `brevio.feedback.applied` audit detail contains structural-only fields (no subject/sender/body) (C12) | ☐ | _<sample row JSON + canary scan summary>_ |
| Test 1: Reversibility — DELETE → fresh row with `ignored_count=1` (C10) | ☐ | _<delete output + post-inject ignored_count>_ |
| Test 2: Slack approval writes `feedback.written` with `verb=approved`, `role=founder`, `legacy_kind=founder_approved` (C13) | ☐ | _<sample row JSON>_ |
| Test 3: `pnpm smoke-evidence:v0.5.7` reports PASS (or identical to PR #46 / v0.5.8 documented benign shape) — HMR un-regressed (C14) | ☐ | _<v0.5.7 verdict on this branch>_ |
| Test 4: Non-founder `feedback.written` + `sender_feedback_ignored` rows = 0 in window; stop_active non-founder diff empty (C11) | ☐ | _<query outputs>_ |
| **Test 5 (LOAD-BEARING): ops-inject with `source_surface='calendar_reminder'` → rejected with `inactive_surface` audit; zero rows in `feedback_events` with that surface (C6)** | ☐ | _<exit code + failure audit row + DB count>_ |
| Code-level (C2): migration 0007 applied; `\\d feedback_events` shows `source_surface text NOT NULL DEFAULT 'email_alert'`; all existing rows backfilled | ☐ | _<psql output>_ |
| Code-level (C3 + C4): `BREVIO_FEEDBACK_SURFACES` exports exactly 13 entries; `BREVIO_FEEDBACK_ACTIVE_SURFACES === ['email_alert']`; `mapLegacyFeedbackKind` covers 10 of 11 legacy kinds (`stop` preserved) | ☐ | _<grep + test run summary>_ |
| Code-level (C5–C9): `pnpm --filter @brevio/fomo test` shows new unit tests green per §3 list | ☐ | _<test run summary>_ |

**Sample `feedback.written` detail JSON (paste 1–2 representative rows verbatim):**

```json
_<paste verbatim — should include source_surface, verb, dimension, role, optional legacy_kind>_
```

**Sample `brevio.feedback.applied` detail JSON (paste 1 representative row verbatim):**

```json
_<paste verbatim — should include feedback_event_id, source_surface, verb, dimension, memory_signal_kind, memory_signal_action, confidence; NO raw sender_email>_
```

**Sample `memory_signals(sender_feedback_ignored)` detail JSON + scope_key snippet (paste verbatim):**

```json
_<paste verbatim — detail has ignored_count, first/last_ignored_at, source_feedback_event_ids[], source_surface>_
```

`scope_key` (first 8 chars + regex check): _<e.g., `5d3b9a1f...` matches /^[0-9a-f]{32}$/ ✓>_

## 14. Founder observations

| Observation | Note |
|---|---|
| Does the ops-inject script feel like the right ergonomics for the future Slack "Quiet this sender" button + reply-parser feedback intents? | _<…>_ |
| How does `confidence≈0.6` after one ignored event feel? Should the formula change in PIL phase? | _<…>_ |
| Did the legacy-kind mapping cause any visible behavior change on existing Slack interactivity? | _<…>_ |
| Is the privacy hashing (`HMAC-SHA-256` of `user_id+email`) the right shape, or should we move to a fully-random opaque ID per (user, sender) stored in a side-table? | _<…>_ |
| Anything else in audit_log that surprised you? | _<…>_ |
| Does v0.5.9 feel like enough substrate to scope PIL next (consumer side), or is there a residual capture gap to close first (e.g. activating one more surface)? | _<…>_ |

### Bonus findings (real-incident-backed, candidates for next-phase 6Q gate)

1. _<…>_
2. _<…>_

## 15. Verdict

☐ **PASS** — all 16 criteria green; §6 Test 1 produced the full feedback → memory_signal → audit chain end-to-end; Test 5 active-surface live reject fires (LOAD-BEARING "not trapped in email" proof); Test 3 confirms HMR un-regressed; Test 4 cross-tenant non-founder rows untouched; all 9 evidence scripts as expected (v0.5.3/4/5/7/8 may legitimately FAIL per documented shapes; operator confirmed identical shape). **Next phase runs its own 6-question gate.**

☐ **FAIL** — list below.

☐ **PENDING** — runtime commit not yet on branch OR migration 0007 not applied; re-run after both land.

Failures / followups:

- _…_

## 16. Sign-off

- Founder signature: Galiette Mita
- Date: _<YYYY-MM-DD>_
- No friend consent needed this phase (founder-only smoke)

## 17. Aftercare confirmation

- [ ] Dev server (Terminal 1) stopped after the smoke
- [ ] If Test 2 left a fresh `stop_active` row for founder from any outbound failure, no action (v0.5.3 drift detector behavior — not a v0.5.9 concern)
- [ ] No friend deletion ops (no friend involved)
- [ ] v0.5.7 HMR template_version still `human-message-v0.3.0` — confirmed via §10 Test 3
- [ ] Migration 0007 applied; reversible if needed (ALTER TABLE feedback_events DROP COLUMN source_surface)
- [ ] No new agentic surface introduced
- [ ] No LLM call accidentally introduced anywhere (3E.1 invariant; v0.5.9 is substrate-only — confirmed by code review)
- [ ] No raw email substring in new audit / memory_signal detail (C16 canary scan PASS)

## 18. What v0.5.9 PASS does NOT promise

v0.5.9 PASS unlocks the next 6-question gate. It explicitly does NOT auto-unlock:

- **PIL substrate** — strategic candidate after v0.5.9 (reads `sender_feedback_ignored` + future signals); its own 6Q gate
- **Reply-parser feedback intents** — Q4.C deferred; own future phase
- **HMR feedback-prompt surface** ("Was this the kind of thing you want me to catch?") — own future phase
- **Activating any source_surface beyond `email_alert`** — each its own 6Q gate
- **F1 SendBlue tier fix** — own future phase
- **Friend C onboarding** — three-friend cap; Friend C is OPTIONAL
- **Auto-send** — its own gate per FOMO_PLAN v0.8
- **Reversal of 3E.1 no-LLM-body-generation directive** — v0.5.9 PRESERVES 3E.1 by design (substrate-only)
- **OAuth auto-refresh-after-expiry hardening** — hardening-backlog entry; its own future gate
- **Hardening-backlog #2 (Gmail 404 → benign transient skip)** — own gate
- **Hardening-backlog #3 (sanitized error_code + error_reason on Gmail/dispatch errors)** — own gate
- **Hardening-backlog #4 (v0.5.8 review findings)** — own gate
- **Any HMR-surface expansion** (calendar / drafts / tasks / etc.) — each own 6Q gate
- **Dashboard / web UI**
- **A new email provider** — Gmail remains only active provider per [FOMO_DESIGN.md §6.2](../FOMO_DESIGN.md)
- **A new model provider** — OpenAI-first per [FOMO_DESIGN.md §18](../FOMO_DESIGN.md)
- **STOP/START as preference feedback** — consent/control stays permanently separate from preference learning

The next phase is decided AT THE NEXT 6-question gate.
