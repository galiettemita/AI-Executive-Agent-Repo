# Phase v0.5.8 Smoke Test Report — Gmail INBOX Event Reliability Hardening

> Filled after running every step in `smoke-test-v0.5.8-gmail-inbox-reliability.md`.
> Commit as `docs/SMOKE_REPORT_v0.5.8.md` once **`VERDICT: PASS`** on all 14 criteria AND §6 Test 1 Gmail-to-self synthetic produced a rank within ≤3 cycles AND Test 3 confirms `smoke-evidence:v0.5.7` still PASSES on this branch.
>
> **Scaffolding-vs-runtime note:** if this report is filled before the runtime implementation commit lands, `smoke-evidence:v0.5.8` will print `VERDICT: PENDING` (not PASS). That is expected at scaffolding time. The report can only legitimately reach `VERDICT: PASS` after both the SCAFFOLDING commit and the RUNTIME commit are on this branch AND the §6 test sequence has been run end-to-end.
>
> **First phase under the Core Dimension Check discipline:** advances Dim 2 (Proactivity) + Dim 10 (Observability/Reliability); preserves HMR (Dim 9), PIL/Feedback substrates, autonomy/memory/reasoning/tool/multimodal/scale/trust dimensions; intentionally defers Dim 8 (Feedback + Learn/Grow Loop) to the next gate. See [scope memory](../.claude/projects/-Users-galiettemita-Downloads-Executive-AI-Agent-backend/memory/project_v05-8-scope.md).
>
> **v0.5.8 PASS does NOT auto-unlock** v0.5.9 Feedback substrate, F1 SendBlue tier fix, Friend C, PIL substrate, auto-send, 3E.1 reversal, OAuth auto-refresh hardening, any HMR-surface expansion, or any v0.5.7 §11 bonus finding. The next phase runs its own 6-question gate.

---

**Founder:** Galiette Mita
**Run date:** _<YYYY-MM-DD HH:MM TZ>_
**Branch:** `phase-v0.5.8-gmail-inbox-reliability`
**Scaffolding commit SHA:** _<sha>_
**Runtime commit SHA:** _<sha>_
**Smoke window override (if any):** `FOMO_V0_5_8_WINDOW_HOURS=24` (default)
**SMOKE_START_TS:** _<UTC ISO timestamp>_

---

## 1. Prerequisites confirmed

- [ ] PR #46 (v0.5.7 HMR) on `main` with VERDICT: PASS
- [ ] PR #47 (docs: brevio permanent product layers + 12 core dimensions) on `main`
- [ ] No friend involvement (three-friend cap holds)
- [ ] §1 baseline snapshots captured BEFORE smoke start (all three: `gmail.poll.cycle`, `rank_results` count, `stop_active`)
- [ ] Founder Gmail web UI accessible for Gmail-to-self send (Test 1 — LOAD-BEARING)
- [ ] (Optional) iCloud (or non-Gmail) account accessible for external regression (Test 2)
- [ ] ngrok NOT required — v0.5.8 does not exercise SendBlue inbound

## 2. Env additions (redacted)

| Var | Set? | Notes |
|---|---|---|
| `FOMO_V0_5_8_BASELINE_CONFIRMED` | ☐ | set `true` after §1 capture |
| `FOMO_V0_5_8_WINDOW_HOURS` | ☐ | default 24 |

All other v0.5.4 / v0.5.5 / v0.5.6 / v0.5.7 env vars unchanged.

## 3. PASS criteria (14 — Gmail INBOX Event Reliability)

| # | Criterion | Evidence | Got |
|---|---|---|---|
| C1 | `fomo.gmail.poll.event_observed` registered in `FOMO_AUDIT_ACTIONS` | _<preflight + smoke-evidence output>_ | ☐ |
| C2 | `fomo.gmail.poll.event_skipped` registered in `FOMO_AUDIT_ACTIONS` | _<preflight + smoke-evidence output>_ | ☐ |
| C3 | Poller `historyTypes` query parameter contains `'labelAdded'` | _<grep output of apps/fomo/src/adapters/gmail/client.ts>_ | ☐ |
| C4 | Unit test: external `messageAdded` path → dispatch (no regression) | _<test name + run output>_ | ☐ |
| C5 | Unit test: Gmail-to-self `labelAdded:INBOX`-only path → dispatch | _<test name + run output>_ | ☐ |
| C6 | Unit test: routed / forwarded `labelAdded:INBOX` → dispatch | _<test name + run output>_ | ☐ |
| C7 | Unit test: duplicate `messageAdded`+`labelAdded` same cycle → exactly ONE dispatch (Q3.A dedupe) | _<test name + run output>_ | ☐ |
| C8 | Unit test: `labelAdded` with NON-INBOX label → no dispatch | _<test name + run output>_ | ☐ |
| C9 | Unit test: malformed `labelAdded` (missing `addedLabels`) → `event_skipped` audit + skip | _<test name + run output>_ | ☐ |
| C10 | **Live smoke (Path A): Gmail-to-self synthetic → rank within ≤3 poll cycles (vs v0.5.7 NEVER)** | _<§6 Test 1 timestamps + rank_results row>_ | ☐ |
| C11 | Live smoke regression: external email still ranks via `messageAdded` path | _<§6 Test 2 cycle counter `m_added_only` total>_ | ☐ |
| C12 | `fomo.gmail.poll.event_observed` populated with `event_types_seen` containing `'labelAdded'` for ≥1 message; sanitized canary scan finds zero forbidden substrings | _<smoke-evidence output + sample row JSON>_ | ☐ |
| C13 | Cycle counter `messages_observed_via_labelAdded_only` ≥ 1 in smoke window | _<smoke-evidence output + sample cycle row>_ | ☐ |
| C14 | Cross-tenant isolation + HMR regression: `smoke-evidence:v0.5.7` still PASSES on this branch (no v0.5.7 regression); only founder polled in smoke window | _<§6 Test 3 + Test 4 outputs>_ | ☐ |

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

> Operator note: v0.5.8 is a founder-only smoke — no `/onboard/callback` runs. If v0.5.3 reports Item #1 FAIL with no contact_registered rows, that's expected blocked-substrate, NOT a v0.5.8 regression. Same shape as prior SMOKE_REPORTs.

## 7. `smoke-evidence:v0.5.4` output (`FOMO_V0_5_4_WINDOW_HOURS=168`) — **VERDICT: _<…>_**

```
_<paste full output>_
```

> Operator note: if v0.5.4 reports C13/C14 window-slide FAIL shape that prior SMOKE_REPORTs documented, that's NOT a v0.5.8-caused regression. Confirm rows are not from the v0.5.8 smoke window.

## 8. `smoke-evidence:v0.5.5` output — **VERDICT: _<…>_**

```
_<paste full output>_
```

> Operator note: if v0.5.5 reports C2/C3/C11 SendBlue OPTED_OUT blocked-external FAIL — same shape PR #43 documented — that's NOT a v0.5.8-caused regression. F1 SendBlue tier fix is its own future-phase candidate, NOT in v0.5.8 scope.

## 9. `smoke-evidence:v0.5.6` output — **VERDICT: _<…>_**

```
_<paste full output>_
```

## 10. `smoke-evidence:v0.5.7` output (HMR regression check) — **VERDICT: _<…>_**

```
_<paste full output>_
```

> Operator note (LOAD-BEARING for C14): if v0.5.7 reports a NEW failure that PR #46's SMOKE_REPORT did not have, **v0.5.8 has regressed HMR — STOP and fix before merging.** Acceptable: identical shape to PR #46 PASS (or PR #46's documented blocked-external shapes).

## 11. `smoke-evidence:v0.5.8` output (Gmail INBOX reliability proof) — **VERDICT: _<…>_**

```
_<paste full output>_
```

## 12. Operator-confirmed smoke evidence

| Check | Confirmed? | Notes |
|---|---|---|
| **Test 1 (LOAD-BEARING): Gmail-to-self synthetic email surfaced ≤3 cycles after send** | ☐ | _<SMOKE_START_TS, rank_results.created_at, delta in seconds>_ |
| Test 1: ≥1 `fomo.gmail.poll.event_observed` row whose `event_types_seen` contains `"labelAdded"` (C12) | ☐ | _<sample row JSON>_ |
| Test 1: `event_observed` detail contains ONLY structural fields (`event_types_seen`, `inbox_label_present`, `is_dedupe_drop`, `message_id`) — no subject / sender / body / raw label names | ☐ | _<canary scan summary>_ |
| Test 1: Cycle counter `messages_observed_via_labelAdded_only` ≥ 1 in window (C13 KEY METRIC) | ☐ | _<window total + sample cycle row>_ |
| Test 1: `messages_observed_via_both == messages_dedupe_drops` (Q3.A invariant) | ☐ | _<window totals>_ |
| Test 1: `messages_observed` continues to count post-dedupe unique observations (carry-forward invariant) | ☐ | _<sample cycle row>_ |
| Test 1: Slack card body still renders via `human-message-v0.3.0` template (HMR un-regressed) | ☐ | _<sample fomo.send.attempted detail.template_version>_ |
| Test 2: External (icloud → gmail) email ranks via `messageAdded` path; `m_added_only` ≥ 1 in window (C11) | ☐ | _<window total + sample cycle row>_ |
| Test 3: `pnpm smoke-evidence:v0.5.7` reports PASS (or identical to PR #46 shape) — HMR un-regressed (C14) | ☐ | _<v0.5.7 verdict on this branch>_ |
| Test 4: `stop_active` baseline-vs-post NON-FOUNDER diff is empty (v0.5.8 must NOT touch non-founder rows) | ☐ | _<diff output>_ |
| Test 4: Only `actor_user_id='founder'` in `fomo.send.attempted` + `fomo.gmail.poll.event_observed` for smoke window | ☐ | _<group-by output>_ |
| Code-level (C3): `grep -n "historyTypes:" apps/fomo/src/adapters/gmail/client.ts` shows the comma-list filter | ☐ | _<grep output line>_ |
| Code-level (C4–C9): `pnpm --filter @brevio/fomo test` shows 6 new tests green per the §3 list | ☐ | _<test run summary>_ |

**Sample `fomo.gmail.poll.event_observed` detail JSON (paste 1–3 representative rows verbatim):**

```json
_<paste verbatim>_
```

**Sample `gmail.poll.cycle` detail JSON showing the 4 new counters (paste 1–2 cycles verbatim):**

```json
_<paste verbatim>_
```

## 13. Founder observations

| Observation | Note |
|---|---|
| Compared to v0.5.7, did the Gmail-to-self synthetic actually arrive within ≤3 cycles? Felt like real-time? | _<…>_ |
| Across the smoke window, what fraction of observations came from `labelAdded_only` vs `messageAdded_only` vs `both`? | _<window distribution>_ |
| Any `event_observed` row whose `inbox_label_present=false` AND `event_types_seen` contains `labelAdded`? (Should be zero by Q2.A — surface if seen) | _<…>_ |
| Any `fomo.gmail.poll.event_skipped` rows? If yes, was Gmail genuinely sending malformed events, or is the runtime falsely flagging valid events? | _<…>_ |
| Any unexpected `messages_dedupe_drops > messages_observed_via_both` violations? | _<…>_ |
| Anything else in audit_log that surprised you? | _<…>_ |
| Does v0.5.8 feel like enough hardening to proceed to v0.5.9 (Feedback substrate), or is there a residual proactivity gap to gate first? | _<…>_ |

### Bonus findings (real-incident-backed, candidates for next-phase 6Q gate)

1. _<…>_
2. _<…>_

## 14. Verdict

☐ **PASS** — all 14 criteria green; §6 Test 1 Gmail-to-self synthetic ranked ≤3 cycles; Test 3 confirms `smoke-evidence:v0.5.7` still PASSES on this branch; Test 4 cross-tenant non-founder diff empty; all 8 evidence scripts as expected (v0.5.3/4/5 may legitimately FAIL per documented shapes; operator confirmed identical shape). **Next phase runs its own 6-question gate with the binding three principle-gate questions + Core Dimension Check + per-phase Q1–Q6.**

☐ **FAIL** — list below.

☐ **PENDING** — runtime commit not yet on branch; re-run after runtime lands.

Failures / followups:

- _…_

## 15. Sign-off

- Founder signature: Galiette Mita
- Date: _<YYYY-MM-DD>_
- No friend consent needed this phase (founder-only smoke)

## 16. Aftercare confirmation

- [ ] If Test 1 left a fresh `stop_active` row for founder from outbound failure, no action (v0.5.3 drift detector behavior — not a v0.5.8 concern)
- [ ] No friend deletion ops (no friend involved)
- [ ] v0.5.7 HMR template_version still `human-message-v0.3.0` — confirmed via §10 Test 3
- [ ] No new schema / migration introduced (Q4.A invariant)
- [ ] No LLM call accidentally introduced anywhere (3E.1 invariant; v0.5.8 is poller-layer only — confirmed by code review)
- [ ] Dev server (Terminal 1) stopped after the smoke

## 17. What v0.5.8 PASS does NOT promise

v0.5.8 PASS unlocks the next 6-question gate. It explicitly does NOT auto-unlock:

- **v0.5.9 Feedback + Learn/Grow Loop substrate** — strategic next-phase candidate per founder direction; its own 6Q gate
- **F1 SendBlue tier fix / un-flag** — its own future-phase candidate
- **Friend C onboarding** — three-friend cap; Friend C is OPTIONAL
- **Personalized Importance Learning substrate** — separate phase per [docs/personalized-importance-learning.md](personalized-importance-learning.md)
- **Auto-send** — its own gate per FOMO_PLAN v0.8
- **Reversal of 3E.1 no-LLM-body-generation directive** — v0.5.8 PRESERVES 3E.1 by design (poller-layer only)
- **OAuth auto-refresh-after-expiry hardening** — [hardening-backlog](../.claude/projects/-Users-galiettemita-Downloads-Executive-AI-Agent-backend/memory/project_hardening-backlog.md) candidate entry #2; its own future gate
- **Any HMR-surface expansion** (calendar / drafts / tasks / etc.) — each own 6Q gate
- **The four v0.5.7 §11 bonus findings** — each own gate
- **Short-body length policy resolution** — own future gate per v0.5.6 PASS finding
- **Runbook drift-detector amendment** — own future gate per v0.5.6 PASS finding
- **Dashboard / web UI**
- **A new email provider** — Gmail remains only active provider per [FOMO_DESIGN.md §6.2](../FOMO_DESIGN.md)
- **A new model provider** — OpenAI-first per [FOMO_DESIGN.md §18](../FOMO_DESIGN.md)

The next phase is decided AT THE NEXT 6-question gate.
