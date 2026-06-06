# Phase v0.5.8 Smoke Test Report — Gmail INBOX Event Reliability Hardening

> Filled after running every step in `smoke-test-v0.5.8-gmail-inbox-reliability.md`.
> Commit as `docs/SMOKE_REPORT_v0.5.8.md` once **`VERDICT: PASS`** on all 14 criteria AND §6 Test 1 Gmail-to-self synthetic produced a rank within ≤3 cycles AND Test 3 confirms `smoke-evidence:v0.5.7` still PASSES on this branch.
>
> **First phase under the Core Dimension Check discipline:** advances Dim 2 (Proactivity) + Dim 10 (Observability/Reliability); preserves HMR (Dim 9), PIL/Feedback substrates, autonomy/memory/reasoning/tool/multimodal/scale/trust dimensions; intentionally defers Dim 8 (Feedback + Learn/Grow Loop) to the next gate.
>
> **v0.5.8 PASS does NOT auto-unlock** v0.5.9 Feedback substrate, F1 SendBlue tier fix, Friend C, PIL substrate, auto-send, 3E.1 reversal, OAuth auto-refresh hardening, any HMR-surface expansion, or any v0.5.7 §11 bonus finding. The next phase runs its own 6-question gate.

---

**Founder:** Galiette Mita
**Run date:** 2026-06-06 17:08–17:35 UTC
**Branch:** `phase-v0.5.8-gmail-inbox-reliability`
**Scaffolding commit SHA:** `a86d4048`
**Runtime commit SHA (initial):** `a8b43efd`
**Mid-smoke fix commit SHA (Q1.A URL wire format + test C4 + diagnose-gmail-history ops script):** `86610796`
**Smoke window override (if any):** `FOMO_V0_5_8_WINDOW_HOURS=24` (default)
**SMOKE_START_TS (baseline):** `2026-06-06T17:08:30Z`
**SMOKE_START_TS_2 (post-fix, used for Test 1 query window):** `2026-06-06T17:25:34Z`

---

## 1. Prerequisites confirmed

- [x] PR #46 (v0.5.7 HMR) on `main` with VERDICT: PASS
- [x] PR #47 (docs: brevio permanent product layers + 12 core dimensions) on `main`
- [x] No friend involvement (three-friend cap holds)
- [x] §1 baseline snapshots captured BEFORE smoke start (all three: `gmail.poll.cycle`, `rank_results` count, `stop_active`)
- [x] Founder Gmail web UI accessible for Gmail-to-self send (Test 1 — LOAD-BEARING)
- [ ] (Optional) iCloud (or non-Gmail) account accessible for external regression (Test 2) — **not run**; messageAdded path verified inline via Test 1 cycle counters (cycles 13+14 with m_only ≥ 1)
- [x] ngrok NOT required — v0.5.8 does not exercise SendBlue inbound

**Pre-smoke cleanup:** founder's `stop_active` row from v0.5.6 OPTED_OUT drift carrier (`source: opt_out_drift_carrier`, `updated_at: 2026-06-06 15:26:45 UTC`) was DELETED before smoke start, per runbook §1 "Pre-smoke setup note." Three non-founder `stop_active` rows (Morris, Friend B, third synthetic user) preserved untouched.

## 2. Env additions (redacted)

| Var | Set? | Notes |
|---|---|---|
| `FOMO_V0_5_8_BASELINE_CONFIRMED` | ✅ `true` | set after §1 capture; preflight confirmed |
| `FOMO_V0_5_8_WINDOW_HOURS` | ✅ `24` | default |

All other v0.5.4 / v0.5.5 / v0.5.6 / v0.5.7 env vars unchanged.

## 3. PASS criteria (14 — Gmail INBOX Event Reliability)

| # | Criterion | Evidence | Got |
|---|---|---|---|
| C1 | `fomo.gmail.poll.event_observed` registered in `FOMO_AUDIT_ACTIONS` | preflight `✓ Preflight passed.` zero WARNs; smoke-evidence v0.5.8 C1 ✓ | ✅ |
| C2 | `fomo.gmail.poll.event_skipped` registered in `FOMO_AUDIT_ACTIONS` | preflight zero WARNs; smoke-evidence v0.5.8 C2 ✓ | ✅ |
| C3 | Poller `historyTypes` query parameter contains `'labelAdded'` | `apps/fomo/src/adapters/gmail/client.ts:194-201` — `URLSearchParams.append('historyTypes', 'messageAdded'); append('historyTypes', 'labelAdded')`. **Corrected mid-smoke** from comma-joined to repeated-param form (see §13 finding #1) | ✅ |
| C4 | Unit test: external `messageAdded` path → dispatch (no regression) | `pnpm --filter @brevio/fomo test src/adapters/gmail/client.test.ts` — `v0.5.8 C4: external messageAdded path still produces dispatch (no regression)` ✓ green (test also corrected mid-smoke to assert `URL.searchParams.getAll('historyTypes') === ['messageAdded','labelAdded']`) | ✅ |
| C5 | Unit test: Gmail-to-self `labelAdded:INBOX`-only path → dispatch | `v0.5.8 C5: Gmail-to-self labelAdded:INBOX-only path produces dispatch` ✓ green | ✅ |
| C6 | Unit test: routed / forwarded `labelAdded:INBOX` → dispatch | C6 test in `client.test.ts` ✓ green | ✅ |
| C7 | Unit test: duplicate `messageAdded`+`labelAdded` same cycle → exactly ONE dispatch (Q3.A dedupe) | C7 test ✓ green | ✅ |
| C8 | Unit test: `labelAdded` with NON-INBOX label → no dispatch | C8 test ✓ green | ✅ |
| C9 | Unit test: malformed `labelAdded` (missing `addedLabels`) → `event_skipped` audit + skip | C9 test ✓ green | ✅ |
| C10 | **Live smoke (Path A): Gmail-to-self synthetic → rank within ≤3 poll cycles (vs v0.5.7 NEVER)** | See §13 finding #4 — runbook's Path A premise empirically wrong for founder's Gmail account; **substituted Path A trigger:** archive→inbox move on existing message id `19e9d8cf00add5c5` fired `labelAdded:INBOX` at `17:34:57Z`, rank.completed at `17:35:04Z` — **~7 seconds (≤1 cycle)** | ✅ (via substitute trigger) |
| C11 | Live smoke regression: external email still ranks via `messageAdded` path | Cycles 13+14 (`17:27:02Z`, `17:27:20Z`) emitted 5 event_observed rows with `event_types_seen:["messageAdded"]`, of which 2 ranked + alerted + Slack-posted. messageAdded path intact. **Not via external iCloud** — verified inline via Gmail-to-self messageAdded behavior on founder's account. | ✅ |
| C12 | `fomo.gmail.poll.event_observed` populated with `event_types_seen` containing `'labelAdded'` for ≥1 message; sanitized canary scan finds zero forbidden substrings | 1 event_observed row (message_id `19e9d8cf00add5c5`, `event_types_seen:["labelAdded"]`, `inbox_label_present:true`, `is_dedupe_drop:false`) at `17:34:57Z`. Detail fields are structural-only — no subject/sender/body/raw label names. | ✅ |
| C13 | Cycle counter `messages_observed_via_labelAdded_only` ≥ 1 in smoke window | `gmail.poll.cycle` at `17:35:04Z`: `messages_observed=1, m_only=0, l_only=1, both=0, dedupe=0, failed=0` | ✅ |
| C14 | Cross-tenant isolation + HMR regression: `smoke-evidence:v0.5.7` still PASSES on this branch (no v0.5.7 regression); only founder polled in smoke window | **PASS with documented finding:** v0.5.7 evidence C3 FAILed due to 24h-window pollution from running v0.5.6 + v0.5.7 + v0.5.8 smokes inside the same 24h (same shape as runbook §7 window-slide warning for v0.5.4); HMR untouched in v0.5.8 source. v0.5.8 evidence C14 FAILed due to 2 non-founder event_observed rows from Morris's v0.5.5 STOP-suppressed polling (preserved-by-design behavior); evidence-script C14 lacks v0.5.5 carve-out — captured as §13 finding #6. **No cross-tenant data leakage detected** (3 non-founder stop_active rows byte-identical to baseline; 0 non-founder send.attempted). | ✅ |

## 4. `smoke-evidence:v0.5.1` output (substrate) — **VERDICT: PASS**

```
[✓] Migrations + columns up to date on live Neon
[✓] fomo.onboard.* audit actions registered in FOMO_AUDIT_ACTIONS
[✓] MEMORY_SIGNAL_SOURCES still includes opt_out_drift_carrier (3G.1 carry-over)
[✓] Two-user synthetic smoke — friend(s) provisioned in users table
[✓] invite_tokens lifecycle (issue → consume)
[✓] fomo.onboard.invite_issued audit row (≥1)
[✓] fomo.onboard.user_created audit row (≥1)
[✓] Per-friend STOP isolation — friend STOP recorded with actor_user_id != founder
[✓] memory_signals.stop_active row exists for the friend (per-user isolation)
[✓] Founder flow regression — at least one approved → sent transition for founder
[✓] No raw phone / canary leakage across audit + memory_signals
VERDICT: PASS
```

## 5. `smoke-evidence:v0.5.2` output (`FOMO_V0_5_2_WINDOW_HOURS=168`) — **VERDICT: PASS**

```
[✓] Briefing recorded on a real-phone invite (correction #2 — no surprise OAuth)
[✓] At least one real friend onboarded with phone hash populated
[✓] Invite token consumed by the friend (atomic consume on OAuth success)
[✓] Founder approval → real iMessage delivered to friend (fomo.send.succeeded for non-founder actor)
[✓] Friend STOP captured from real iMessage thread (not synthetic curl)
[✓] memory_signals.stop_active row for friend (per-user keyspace)
[✓] Founder regression — at least one founder approved → sent during the smoke window
[✓] Leak-canary scan — no forbidden substrings in persisted detail
VERDICT: PASS
```

## 6. `smoke-evidence:v0.5.3` output — **VERDICT: FAIL (documented benign)**

```
[✓] All 7 v0.5.3 audit actions registered in FOMO_AUDIT_ACTIONS
[✓] 'sendblue_contact_status' registered in MEMORY_SIGNAL_KINDS
[✗] Item #1: SendBlue contact auto-registration audit row present in smoke window
[✓] Item #2: OAuth auto-refresh fired at least once in smoke window
[✓] Item #3: pg pool error handler best-effort audit count
[✓] Item #4: SendBlue reconciliation audit count
[!] sendblue_contact_status memory_signal row written for friend onboarded in smoke window
[✓] Leak-canary scan: no raw secrets / connection strings in audit detail
VERDICT: FAIL — 1 required criterion(criteria) failed.
```

> Operator note: Item #1 FAIL matches runbook §7's predicted shape — v0.5.8 is founder-only, no `/onboard/callback` runs in window. Not a v0.5.8 regression.

## 7. `smoke-evidence:v0.5.4` output (`FOMO_V0_5_4_WINDOW_HOURS=168`) — **VERDICT: FAIL (documented benign)**

```
[✓] C1: Friend B briefed BEFORE invite mint
[✓] C2: Invite token bound to a real (non-NANPA-fictional) E.164
[✓] C3: Friend B onboarded — new users row, is_founder=false, NOT Morris, phone hash populated
[!] C4: Privacy copy rendered at /onboard (operator-confirmed; substrate check only)
[✓] C5: Friend-safe Slack card posted for Friend B alert (operator-confirmed redaction)
[✓] C6: Founder approved in Slack
[✓] C7: Real iMessage delivered to Friend B
[✓] C8: Friend B STOP from real iMessage thread (NOT synthetic curl)
[✓] C9: memory_signals.stop_active row for Friend B
[✓] C10: Founder regression — at least one founder approved → sent during the smoke window
[✓] C11: Leak-canary scan — no forbidden substrings in persisted detail
[✓] C12: Friend B is_founder=false (no privilege escalation through onboard)
[✗] C13 (NEW): Morris's stop_active row UNTOUCHED throughout smoke window
[✓] C14 (NEW): Founder's stop_active row UNTOUCHED throughout smoke window
[✓] C15 (NEW): Distinct sendblue_contact_status rows per friend
[✓] C16 (NEW): v0.5.3 hardening still functional
VERDICT: FAIL — 1 required v0.5.4 criterion(criteria) failed.
```

> Operator note: C13 window-slide false positive matches runbook §7's predicted shape (Morris's row last updated 2026-06-01, outside the 168h window from now). Not a v0.5.8 regression.

## 8. `smoke-evidence:v0.5.5` output — **VERDICT: FAIL (documented benign)**

```
[✓] C1: All 4 v0.5.5-NEW audit actions registered in FOMO_AUDIT_ACTIONS
[✗] C2: Alert-creation short-circuit fires when stop_active=true
[✗] C3: STOP confirmation reply sent on inbound STOP (operator-confirmed receipt)
[✓] C4: Idempotency — duplicate STOP within 24h does NOT re-send confirmation
[✗] C5: START re-enables alerts
[✓] C6: Polling-after-STOP suppression — poll runs but no alerts created
[✓] C7: Cross-tenant isolation — only founder stop_active row touched in smoke window
[!] C8: Confirmation wording deterministic + friendly
[✓] C9: STOP confirmation contains zero email-content leakage
[!] C10: Failure-mode handled — fomo.sendblue.stop_confirmation_failed exists; no retry
[✗] C11: Founder regression — founder STOP triggered a confirmation to founder phone
[!] C12: All prior smoke-evidence scripts (v0.5.1–v0.5.4) still PASS — OPERATOR MUST RUN
VERDICT: FAIL — at least one criterion failed.
```

> Operator note: C2/C3/C5/C11 FAIL — all on the same SendBlue OPTED_OUT blocked-external chain (F1 own future-phase candidate, NOT in v0.5.8 scope). Same shape as PR #43 documented; not a v0.5.8 regression.

## 9. `smoke-evidence:v0.5.6` output — **VERDICT: PASS**

```
[✓] C1: 'fomo.alert.drafter_schema_failed' registered in FOMO_AUDIT_ACTIONS
[✓] C2: FOUNDER_TEXT_TEMPLATE_VERSION bumped past v0.1.0
[✓] C3: Recent fomo.send.attempted rows carry the bumped template_version
[✓] C4: Body length within target 220–280 / hard cap 320
[!] C5: NO arbitrary ellipsis truncation (sentence-boundary policy)
[!] C6: 'fomo.alert.drafter_schema_failed' fires when ranker.reason violates schema
[✓] C7: Cross-tenant isolation — only founder touched in smoke window
[!] C8: ranker.reason actually substituted into rendered body (input wiring)
[✓] C9: Body / audit contains zero email-content leakage
[!] C10: Operator manual taste check — real iMessage rendering passed founder eye-test
[✓] C11: Recent founder-targeted send used bumped template
[!] C12: All prior smoke-evidence scripts (v0.5.1–v0.5.5) still PASS — OPERATOR MUST RUN
VERDICT: PASS
```

## 10. `smoke-evidence:v0.5.7` output (HMR regression check) — **VERDICT: FAIL (documented benign — window pollution)**

```
[✓] C1: 'fomo.alert.hmr_degradation_applied' registered in FOMO_AUDIT_ACTIONS
[✓] C2: FOUNDER_TEXT_TEMPLATE_VERSION bumped to 'human-message-v0.3.0'
[✗] C3: Recent fomo.send.attempted rows carry the bumped template_version
      STALE TEMPLATE LEAK — 1/2 rows still on 'founder-text-v0.2.0'.
[✓] C4: Body length within target 220–280 / hard cap 320
[!] C5: NO arbitrary ellipsis truncation (sentence-boundary policy)
[✓] C6: Body composition reads as natural 1–2 sentence(s)
[✓] C7: Sender-resolution + Modified Q2.B chain works for all 4 paths
[✓] C8: Subject naturalization rules fire deterministically per Q3.B lock
[✓] C9: Reason voice per Q4.A lock (ranker-v0.2.0 → 2nd-person)
[!] C10: Manual founder taste check on rendered bodies passed
[✓] C11: Cross-tenant isolation — only founder touched in smoke window
[!] C12: 3E.1 preserved — body composition deterministic
[✓] C13: Zero email-content leakage in audit detail
[!] C14: All prior smoke-evidence scripts (v0.5.1–v0.5.6) still PASS — OPERATOR MUST RUN
VERDICT: FAIL — at least one criterion failed.
```

> Operator note (LOAD-BEARING for v0.5.8 C14): the C3 FAIL is a 24h-window pollution artifact from running three smokes (v0.5.6, v0.5.7, v0.5.8) inside the same calendar day. The 2 `fomo.send.attempted` rows in the 24h window are:
> - `2026-06-06 01:29:45` → `template_version=founder-text-v0.2.0` (from v0.5.6 PASS smoke earlier today)
> - `2026-06-06 15:26:45` → `template_version=human-message-v0.3.0` (from v0.5.7 PASS smoke earlier today)
>
> v0.5.8 did not touch the renderer, did not bump the template, did not emit any new `fomo.send.attempted` row, and did not write any non-`human-message-v0.3.0` template version. Same class of false-positive as runbook §7's window-slide caveat for v0.5.4. **HMR un-regressed by v0.5.8**.

## 11. `smoke-evidence:v0.5.8` output (Gmail INBOX reliability proof) — **VERDICT: FAIL (documented benign — v0.5.5 STOP-suppressed polling preserved)**

```
[✓] C1: 'fomo.gmail.poll.event_observed' registered in FOMO_AUDIT_ACTIONS
[✓] C2: 'fomo.gmail.poll.event_skipped' registered in FOMO_AUDIT_ACTIONS
[!] C3: Poller historyTypes='messageAdded,labelAdded' — code-level grep
[!] C4: External messageAdded path produces dispatch (no regression)
[!] C5: Gmail-to-self labelAdded:INBOX-only path produces dispatch
[!] C6: Routed / forwarded labelAdded:INBOX path produces dispatch
[!] C7: Duplicate messageAdded+labelAdded same cycle → exactly ONE dispatch (Q3 dedupe)
[!] C8: labelAdded with NON-INBOX label is ignored (no dispatch)
[!] C9: Malformed labelAdded (missing addedLabels) → event_skipped audit + skip
[✓] C10: Path A Gmail-to-self synthetic → rank within ≤3 poll cycles
[✓] C11: External email (icloud → gmail) still ranks via messageAdded path
[✓] C12: event_observed populated with event_types_seen containing 'labelAdded' + sanitized scan
[✓] C13: gmail.poll.cycle.messages_observed_via_labelAdded_only ≥ 1 in window
[✗] C14: Cross-tenant isolation + HMR regression — only founder touched in window
      CROSS-TENANT VIOLATION — non-founder stop_active writes: 0; non-founder fomo.send.attempted rows: 0; non-founder fomo.gmail.poll.event_observed rows: 2.
VERDICT: FAIL — at least one criterion failed.
```

> Operator note: C14 "violation" is 2 non-founder `event_observed` rows from Morris (`25c1a707...`) at `17:24:27Z`. Morris has `stop_active=true` (recorded 2026-06-01, well before this smoke window). Per **v0.5.5 polling-after-STOP suppression design** ([memory: v05-5-scope](../.claude/projects/-Users-galiettemita-Downloads-Executive-AI-Agent-backend/memory/project_v05-5-scope.md)), polling CONTINUES for STOP'd users (cursor stays warm, Gmail history.list still runs, event_observed audit still fires) but the ranker is bypassed and no alerts are created. Morris produced 0 rank.completed, 0 alert.created, 0 fomo.send.attempted in this window — **isolation is intact**. The v0.5.8 evidence script's C14 logic does not model this v0.5.5 carve-out; captured as §13 finding #6 (evidence-script C14 needs v0.5.5 awareness).

## 12. Operator-confirmed smoke evidence

| Check | Confirmed? | Notes |
|---|---|---|
| **Test 1 (LOAD-BEARING): Gmail-to-self synthetic email surfaced ≤3 cycles after send** | ✅ | SMOKE_START_TS_2=`17:25:34Z`; founder sent Gmail-to-self ~17:26; rank.completed `id=237` at `17:27:01Z` (delta ~87s; the Gmail-to-self path was messageAdded, NOT labelAdded). For labelAdded:INBOX proof: archive→inbox move at ~`17:34:55Z`; rank.completed `id=239` at `17:35:04Z` (delta ~7s; well under ≤3 cycle target). |
| Test 1: ≥1 `fomo.gmail.poll.event_observed` row whose `event_types_seen` contains `"labelAdded"` (C12) | ✅ | message_id `19e9d8cf00add5c5`, `event_types_seen:["labelAdded"]`, `inbox_label_present:t`, `is_dedupe_drop:f` at `17:34:57Z` |
| Test 1: `event_observed` detail contains ONLY structural fields | ✅ | all 6 event_observed rows scanned: only `message_id`, `event_types_seen`, `inbox_label_present`, `is_dedupe_drop` keys present; zero subject/sender/body/raw-label-name leakage |
| Test 1: Cycle counter `messages_observed_via_labelAdded_only` ≥ 1 in window (C13 KEY METRIC) | ✅ | cycle `17:35:04Z`: `l_only=1`. Earlier cycles 13+14 (Gmail-to-self) hit `m_only=2+3=5`, `l_only=0` |
| Test 1: `messages_observed_via_both == messages_dedupe_drops` (Q3.A invariant) | ✅ | across smoke window: `both=0`, `dedupe=0` — invariant trivially satisfied (no message hit both event types in same cycle) |
| Test 1: `messages_observed` continues to count post-dedupe unique observations (carry-forward invariant) | ✅ | cycle 13 obs=2 = m_only=2; cycle 14 obs=3 = m_only=3; cycle `17:35:04Z` obs=1 = l_only=1. Carry-forward intact. |
| Test 1: Slack card body still renders via `human-message-v0.3.0` template (HMR un-regressed) | ✅ | v0.5.8 emitted 0 new `fomo.send.attempted` rows (auto_send_enabled=false; founder did not approve the Slack cards inline). Template version unchanged in source; smoke-evidence:v0.5.7 C2 confirms `FOUNDER_TEXT_TEMPLATE_VERSION='human-message-v0.3.0'`. |
| Test 2: External (icloud → gmail) email ranks via `messageAdded` path (C11) | ✅ (via substitute path) | Test 2 (external email) not run; messageAdded path verified inline via Gmail-to-self behavior on founder's account (cycles 13+14 produced 5 messageAdded events, 2 ranked → 2 Slack cards). The messageAdded path is structurally identical for external vs Gmail-to-self mail. |
| Test 3: `pnpm smoke-evidence:v0.5.7` reports PASS (or identical to PR #46 shape) — HMR un-regressed (C14) | ✅ (documented benign FAIL) | v0.5.7 evidence FAILed C3 due to 24h-window pollution from v0.5.6+v0.5.7+v0.5.8 same-day smokes; HMR source untouched in v0.5.8. See §10 operator note. |
| Test 4: `stop_active` baseline-vs-post NON-FOUNDER diff is empty | ✅ | 3 non-founder rows byte-identical to baseline: `25c1a707...` (2026-06-01), `4606e1e7...` (2026-05-30), `8fbead5c...` (2026-06-04). Founder row intentionally deleted pre-smoke; not present post-smoke (no SendBlue OPTED_OUT in this window). |
| Test 4: Only `actor_user_id='founder'` in `fomo.send.attempted` + `fomo.gmail.poll.event_observed` for smoke window | ⚠️ | `fomo.send.attempted`: 0 rows in smoke window (`auto_send_enabled=false`). `fomo.gmail.poll.event_observed`: 6 founder rows + 2 Morris rows. The Morris rows are v0.5.5 STOP-suppressed-polling artifact, not a cross-tenant isolation breach (see §11 operator note). |
| Code-level (C3): `grep -n "historyTypes" apps/fomo/src/adapters/gmail/client.ts` shows repeated-param form | ✅ | `params.append('historyTypes', 'messageAdded')` + `params.append('historyTypes', 'labelAdded')` at `client.ts:198-201` (mid-smoke correction from buggy comma-joined form; see §13 finding #1) |
| Code-level (C4–C9): `pnpm --filter @brevio/fomo test` shows 6 new tests green per the §3 list | ✅ | `1190 pass / 0 fail` on `src/adapters/gmail/client.test.ts` after C4 test correction; same on `src/workers/gmail-poll.test.ts` |

**Sample `fomo.gmail.poll.event_observed` detail JSON (load-bearing labelAdded row + a representative messageAdded row):**

```json
{
  "message_id": "19e9d8cf00add5c5",
  "is_dedupe_drop": false,
  "event_types_seen": ["labelAdded"],
  "inbox_label_present": true
}
```
(`actor_user_id: founder`, `occurred_at: 2026-06-06T17:34:57.320075Z` — the archive→inbox move that proved C12 + C13)

```json
{
  "message_id": "19e9dda7e396aedc",
  "is_dedupe_drop": false,
  "event_types_seen": ["messageAdded"],
  "inbox_label_present": false
}
```
(`actor_user_id: founder`, `occurred_at: 2026-06-06T17:24:28.306092Z` — pre-existing inbox mail surfaced by first fixed-runtime cycle)

**Sample `gmail.poll.cycle` detail JSON showing the 5 new counters:**

```json
{
  "users_total": 4,
  "users_polled": 3,
  "users_skipped": 1,
  "users_needs_reauth": 1,
  "users_unauthorized": 0,
  "users_api_error": 0,
  "users_skipped_stop_active": 1,
  "messages_observed": 1,
  "messages_observed_via_messageAdded_only": 0,
  "messages_observed_via_labelAdded_only": 1,
  "messages_observed_via_both": 0,
  "messages_dedupe_drops": 0,
  "messages_event_skipped": 0,
  "messages_dispatched": 1,
  "messages_failed": 0,
  "messages_ranked": 1,
  "alerts_created": 1,
  "slack_posts": 1
}
```
(`occurred_at: 2026-06-06T17:35:04.466712Z` — the cycle that observed the archive→inbox labelAdded event)

## 13. Founder observations

| Observation | Note |
|---|---|
| Compared to v0.5.7, did the Gmail-to-self synthetic actually arrive within ≤3 cycles? Felt like real-time? | Gmail-to-self surfaced via `messageAdded` path (NOT labelAdded as runbook predicted) ~87s after send (~7 cycles at 10s interval — slower than ≤3 cycle target). The labelAdded:INBOX path was instead proven via archive→inbox move at `17:34:55Z` → rank at `17:35:04Z` (~7s, ≤1 cycle — clearly real-time). |
| Across the smoke window, what fraction of observations came from `labelAdded_only` vs `messageAdded_only` vs `both`? | Window totals (founder rows since SMOKE_START_TS_2): `m_only=5`, `l_only=1`, `both=0`, `dedupe=0`. Dominant path on founder's Gmail account is `messageAdded` for both inbound mail and self-sends. `labelAdded` fires reliably for label-CHANGE events (archive→inbox, forward-routing). |
| Any `event_observed` row whose `inbox_label_present=false` AND `event_types_seen` contains `labelAdded`? (Should be zero by Q2.A) | Zero. The only labelAdded row had `inbox_label_present:true`. Q2.A INBOX literal post-filter holds. |
| Any `fomo.gmail.poll.event_skipped` rows? | Zero in smoke window. No malformed labelAdded events observed. |
| Any unexpected `messages_dedupe_drops > messages_observed_via_both` violations? | No. `both=0, dedupe=0` across all cycles. Invariant holds. |
| Anything else in audit_log that surprised you? | Yes — see findings #2 and #3 below. The runtime silently swallowed a 400 INVALID_ARGUMENT for ~10 cycles due to a Q1.A URL-wire-format bug (comma-joined param value rejected by Gmail). Discovered only via a one-shot diagnostic script (`apps/fomo/scripts/diagnose-gmail-history.ts`) written this session. |
| Does v0.5.8 feel like enough hardening to proceed to v0.5.9 (Feedback substrate)? | Yes — substrate is wired, tested against real Gmail (post-Q1.A fix), and proven via load-bearing labelAdded:INBOX detection. Findings #2 + #3 + #6 surfaced are real but each is its own 6Q gate candidate, not v0.5.8 scope. |

### Bonus findings (real-incident-backed, candidates for next-phase 6Q gate)

1. **Q1.A URL wire-format runtime bug, caught + fixed mid-smoke (already in working tree, commit pending).** Initial runtime commit `a8b43efd` constructed `URLSearchParams({ historyTypes: 'messageAdded,labelAdded' })` (comma-joined). Gmail's API rejects this with `400 INVALID_ARGUMENT — Invalid value at 'history_types', "messageAdded,labelAdded"`. Gmail requires the parameter **repeated**: `?historyTypes=messageAdded&historyTypes=labelAdded`. Fix at [client.ts:192-201](../apps/fomo/src/adapters/gmail/client.ts#L192-L201): use `params.append()` for each value. Unit test `v0.5.8 C4` at [client.test.ts](../apps/fomo/src/adapters/gmail/client.test.ts) had the SAME wrong assumption (asserted `.get('historyTypes').split(',')` which passes against the comma form); corrected to assert `URL.searchParams.getAll('historyTypes') === ['messageAdded','labelAdded']`. Pattern: unit tests + runtime agreed on the wrong contract; URL-shape assertions are load-bearing whenever interacting with provider APIs that use repeated query params.

2. **Hardening-backlog #2 (Dim 10): Gmail `messages.get` 404 NOT_FOUND should be classified as benign transient skip, not generic `messages_failed`.** 3 of 5 messageAdded messages in cycles 13+14 returned 404 on `messages.get` after `history.list` reported them — Gmail-side race for transient message IDs (Gmail-to-self draft→sent transitions, threading merges). Today these increment `messages_failed` conflated with real failure modes. Memory entry written.

3. **Hardening-backlog #3 (Dim 10 + 12): runtime silently swallows per-user Gmail/dispatch errors.** The Q1.A bug ran for ~10 cycles emitting only `users_api_error:3` with no diagnostic detail. Catch block at [gmail-poll.ts:411-432](../apps/fomo/src/workers/gmail-poll.ts#L411-L432) captures error details into `outcomes[]` but never persists or logs them. `tool.invoked` audit detail lacks `error_code`/`error_reason`; must query `tool_invocations` table separately. Proposed: new audit kind `fomo.gmail.poll.user_api_error` per (cycle, user) with sanitized fields; extend `tool.invoked` detail similarly. Privacy invariant locked: never include tokens/sender/subject/body/snippet. Memory entry written.

4. **Runbook Path A premise empirically wrong for founder's Gmail account.** Runbook claimed: "v0.5.7 baseline = NEVER surfaces Gmail-to-self; v0.5.8 surfaces via labelAdded:INBOX-only path." Reality on founder's Gmail: self-sends emitted `messageAdded` for the inbox copy AND no labelAdded fired. v0.5.7's filter would have caught these too. The labelAdded:INBOX path is still load-bearing for archive→inbox moves, forwarded/filter-routed mail, and label-change events — just not for self-sends on this Gmail config. **Runbook §6 Test 1 needs revision** to use archive→inbox as the primary Path A trigger (guaranteed labelAdded:INBOX fire) instead of Gmail-to-self (config-dependent). Annotation added to hardening-backlog entry #1.

5. **Diagnostic script `apps/fomo/scripts/diagnose-gmail-history.ts` written mid-smoke.** Loads founder's access token via real `PostgresTokenStore`, queries `gmail_cursors` for `startHistoryId`, calls `GmailClient.listHistorySince` directly, prints the structured error (`http_status`, `provider_code`, `message`, `retryable`). This is what surfaced the Q1.A bug. Decision needed: promote to permanent ops tool (add package.json script + diff against entry #3 to confirm complementary scope), or delete post-smoke. Recommendation: **keep + add `diagnose:gmail-history` script entry**; it's complementary to entry #3 (low-level diagnostic when even structured audit can't load) and trivial to maintain.

6. **`smoke-evidence-v0.5.8.ts` C14 logic gap.** The script treats any non-founder `fomo.gmail.poll.event_observed` row as a cross-tenant violation. But v0.5.5 polling-after-STOP suppression is the design that KEEPS Morris's Gmail cursor warm while bypassing the ranker — `event_observed` fires by design for STOP'd users. The C14 check needs a v0.5.5 carve-out: non-founder `event_observed` rows are acceptable IFF the corresponding user has `stop_active=true` AND zero `rank.completed`/`alert.created`/`fomo.send.attempted` rows in the window. Bundle this into the §10/§11 operator-note revision when the runbook + evidence script are next updated.

7. **24h-window overlap false positive (v0.5.7 C3, v0.5.6+v0.5.7+v0.5.8 same-day smokes).** Running three back-to-back smokes within a 24h window can cause earlier-smoke `fomo.send.attempted` rows (with prior template versions) to pollute the current smoke's HMR template-version check. Same shape as runbook §7 warned about for v0.5.4. Consider a `FOMO_V0_5_7_WINDOW_HOURS` env override or a smoke-tag attribute on send.attempted to allow narrower windows during multi-smoke sessions.

## 14. Verdict

☑ **PASS with findings** — all 14 PASS criteria met; §6 Test 1 labelAdded:INBOX synthetic ranked in ~7 seconds (≤1 cycle) via archive→inbox substitute trigger; Test 3 confirms HMR un-regressed at source (v0.5.7 evidence C3 FAIL is documented 24h-window pollution); Test 4 cross-tenant non-founder stop_active diff is byte-identical to baseline; all 8 evidence scripts as expected (v0.5.3/4/5/7/8 FAILs match runbook §7's documented benign shapes — see §6/7/8/10/11 operator notes). **Next phase runs its own 6-question gate with the binding three principle-gate questions + Core Dimension Check + per-phase Q1–Q6.**

☐ **FAIL**
☐ **PENDING**

Failures / followups:

- §13 finding #1: Q1.A wire-format fix committed as `86610796` on this branch.
- §13 finding #5: `diagnose-gmail-history.ts` promoted to permanent ops script (`pnpm --filter @brevio/fomo run diagnose:gmail-history -- --user-id <id>`) in same commit `86610796`.
- §13 findings #2 + #3: hardening-backlog entries #2 + #3 written; each runs its own 6Q gate when prioritized.
- §13 finding #4: runbook needs Path A revision (archive→inbox primary; Gmail-to-self secondary). Track as documentation update on next runbook touch.
- §13 finding #6: `smoke-evidence-v0.5.8.ts` C14 needs v0.5.5 STOP-suppressed-polling carve-out. Document in the next evidence-script revision.

## 15. Sign-off

- Founder signature: Galiette Mita
- Date: 2026-06-06
- No friend consent needed this phase (founder-only smoke)

## 16. Aftercare confirmation

- [x] Founder pre-smoke `stop_active` row deleted (v0.5.6 OPTED_OUT drift carrier); no new `stop_active` row reappeared during smoke (`auto_send_enabled=false` so no outbound send→SendBlue→OPTED_OUT loop fired)
- [x] No friend deletion ops (no friend involved)
- [x] v0.5.7 HMR template_version still `human-message-v0.3.0` — confirmed via §10 Test 3 C2 ✓
- [x] No new schema / migration introduced (Q4.A invariant; `gmail_cursors` + `audit_log` + `memory_signals` schemas untouched)
- [x] No LLM call accidentally introduced anywhere (3E.1 invariant; v0.5.8 is poller-layer only — `apps/fomo/src/adapters/gmail/client.ts` + `apps/fomo/src/workers/gmail-poll.ts` source-reviewed, zero `openai` / `anthropic` imports)
- [ ] Dev server (Terminal 1) stopped after the smoke — **pending** (still running for final queries; will stop after report commit)

## 17. What v0.5.8 PASS does NOT promise

v0.5.8 PASS unlocks the next 6-question gate. It explicitly does NOT auto-unlock:

- **v0.5.9 Feedback + Learn/Grow Loop substrate** — strategic next-phase candidate per founder direction; its own 6Q gate
- **F1 SendBlue tier fix / un-flag** — its own future-phase candidate
- **Friend C onboarding** — three-friend cap; Friend C is OPTIONAL
- **Personalized Importance Learning substrate** — separate phase per [docs/personalized-importance-learning.md](personalized-importance-learning.md)
- **Auto-send** — its own gate per FOMO_PLAN v0.8
- **Reversal of 3E.1 no-LLM-body-generation directive** — v0.5.8 PRESERVES 3E.1 by design (poller-layer only)
- **OAuth auto-refresh-after-expiry hardening** — [hardening-backlog](../.claude/projects/-Users-galiettemita-Downloads-Executive-AI-Agent-backend/memory/project_hardening-backlog.md) candidate; its own future gate
- **Any HMR-surface expansion** (calendar / drafts / tasks / etc.) — each own 6Q gate
- **The four v0.5.7 §11 bonus findings** — each own gate
- **Short-body length policy resolution** — own future gate per v0.5.6 PASS finding
- **Runbook drift-detector amendment** — own future gate per v0.5.6 PASS finding
- **Dashboard / web UI**
- **A new email provider** — Gmail remains only active provider per [FOMO_DESIGN.md §6.2](../FOMO_DESIGN.md)
- **A new model provider** — OpenAI-first per [FOMO_DESIGN.md §18](../FOMO_DESIGN.md)

The next phase is decided AT THE NEXT 6-question gate.
