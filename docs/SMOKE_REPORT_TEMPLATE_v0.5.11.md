# Phase v0.5.11 Smoke Test Report — PIL Substrate + Shadow Ranker Context/Eval

> Filled after running every step in `smoke-test-v0.5.11-pil-substrate-shadow.md`. Commit as `docs/SMOKE_REPORT_v0.5.11.md` once **`VERDICT: PASS`** on all 18 criteria AND §6 Path A (LOAD-BEARING ignore_sender on a v0.5.11-rank-time alert) produced the full producer + consumer chain AND §10 Path E (cross-user contamination, LOAD-BEARING) returned 0 rows AND §9 Path D (shadow eval) printed `VERDICT: PASS` AND `smoke-evidence:v0.5.9` + `smoke-evidence:v0.5.10` still PASS.
>
> **Phase under the Core Dimension Check discipline:** advances Dim 3 (Memory) + Dim 4 (Reasoning) + Dim 8 (Feedback Loop) + Dim 10 (Observability); preserves Dim 9 HMR + 3E.1 + v0.5.10 reply parser + v0.5.9 substrate; intentionally defers Dim 5, Dim 7, Dim 11; live ranker reading PIL context = own v0.5.12 6Q gate.
>
> **v0.5.11 PASS does NOT auto-unlock** live ranker reading PIL context, HMR Feedback Acknowledgment, positive-signal memory_signal kinds beyond the two registered here, non-`email_alert` source_surface, F1 SendBlue tier fix, Friend C, autonomy tiers, auto-send, 3E.1 reversal. The next phase runs its own 6-question gate.

---

**Founder:** Galiette Mita
**Run date:** _<YYYY-MM-DD HH:MM TZ>_
**Branch:** `phase-v0.5.11-pil-substrate-shadow`
**Scaffolding commit SHA:** _<sha>_
**Runtime commit SHA(s):** _<sha(s)>_
**Migration 0008 applied to live Neon at:** _<UTC ISO timestamp>_
**SMOKE_START_TS:** _<UTC ISO timestamp>_
**Smoke window override (if any):** `FOMO_V0_5_11_WINDOW_HOURS=24` (default)
**Tunable env vars at smoke time:** k=_<n>_, δ=_<float>_, full_days=_<n>_, decay_days=_<n>_

---

## 1. Prerequisites confirmed

- [ ] PR #50 (v0.5.10 Reply-parser feedback intents) on `main` with VERDICT: PASS
- [ ] No friend involvement (three-friend cap holds)
- [ ] §1 baseline snapshots captured BEFORE smoke start
- [ ] `BREVIO_SENDER_HASH_KEY` set (≥32 bytes; carry-forward from v0.5.9; LOAD-BEARING for alerts.sender_email_hash + new memory_signal scope_key + shadow context lookup)
- [ ] `OPENAI_API_KEY` set (ranker + shadow eval)
- [ ] Migration 0008 (alerts.sender_email_hash) applied to live Neon BEFORE boot (verified via `information_schema.columns`)
- [ ] Four new tunable env vars parse + within bounds (see preflight output)
- [ ] ngrok up + reachable (or signed-curl substitute documented in §16)

## 2. Env additions (redacted)

| Var | Set? | Value | Notes |
|---|---|---|---|
| `FOMO_PIL_K_THRESHOLD` | ☐ | _<n>_ | default 3; minimum n_negative_events to flip sender_suppressed |
| `FOMO_PIL_SCORE_DELTA` | ☐ | _<float>_ | default 0.1; per-event score shift on sender_importance |
| `FOMO_PIL_RECENCY_FULL_DAYS` | ☐ | _<n>_ | default 90; full-weight window |
| `FOMO_PIL_RECENCY_DECAY_DAYS` | ☐ | _<n>_ | default 90; linear decay tail (0 = hard cliff) |
| `FOMO_V0_5_11_BASELINE_CONFIRMED` | ☐ | `true` | set after §1 capture |
| `FOMO_V0_5_11_WINDOW_HOURS` | ☐ | default 24 | |

All other v0.5.x env vars unchanged.

## 3. PASS criteria (18 — PIL substrate + shadow ranker context/eval)

| # | Criterion | Evidence | Got |
|---|---|---|---|
| C1 | Aggregation pipe wires positive intents (this_mattered +δ, more_like_this +2δ) → `sender_importance` upserts | _<DB query + sample row JSON>_ | ☐ |
| C2 | `false_positive` → `sender_importance.score −δ`; `sender_suppressed=true` flips ONLY when `n_negative_events ≥ FOMO_PIL_K_THRESHOLD` | _<Path C queries + unit test output>_ | ☐ |
| C3 | `ignore_sender` writes v0.5.9 `sender_feedback_ignored` AND new `sender_suppressed=true` (explicit single-event flip) | _<Path A queries; both kinds at same scope_key>_ | ☐ |
| C4 | `alerts.sender_email_hash` column present (migration 0008) + populated forward by rank-step write | _<information_schema + alerts row sample>_ | ☐ |
| C5 | **Natural-reply consumer arm binds (LOAD-BEARING)** — `ignore_sender` reply on a v0.5.11-rank-time alert creates `sender_suppressed` with `scope_key = alert.sender_email_hash` (closes v0.5.10 §15 bonus finding #1) | _<Path A queries; scope_key match>_ | ☐ |
| C6 | Per-user HMAC `scope_key` (32-hex) on new memory_signal kinds — prevents raw sender leakage | _<regex check + sample>_ | ☐ |
| C7 | One correction does NOT flip — single `false_positive` event lowers score but `sender_suppressed` UNCHANGED | _<Path C.2 query + unit test>_ | ☐ |
| C8 | Linear recency decay (full 0–90d, linear 90–180d, zero 180d+) tested at ages [0d, 45d, 90d, 135d, 180d, 200d] | _<unit-test output + eval harness fixtures>_ | ☐ |
| C9 | **Cross-user contamination — DB layer (LOAD-BEARING)**: user A `ignore_sender` on hash(X) leaves user B's `memory_signals` for that scope_key = 0 rows | _<Path E query>_ | ☐ |
| C10 | `buildPilContext(userId, senderEmailHash, deps)` exported from `apps/fomo/src/ranker/pil-context.ts` (pure projection; no model call) | _<grep + smoke-evidence>_ | ☐ |
| C11 | Shadow ranker eval harness present + ≥10 synthetic fixtures + shift-DIRECTION assertions; harness PASS | _<eval output tail>_ | ☐ |
| C12 | **Cross-user contamination — shadow eval (LOAD-BEARING)**: user B baseline score == user B + (user A signal injected) score | _<eval output: cross_user fixture>_ | ☐ |
| C13 | **Live ranker behavior UNCHANGED** — PROMPT_VERSION still `ranker-v0.2.0`; production call site passes `pil_context: null` unconditionally; `rank_results` schema unchanged | _<§11 grep + DB diff>_ | ☐ |
| C14 | No outbound alert behavior changes — `sender_suppressed=true` is NEVER read by live dispatch; pre-existing queued_for_review alert still dispatches after operator inserts suppression for its sender hash | _<integration test output>_ | ☐ |
| C15 | `brevio.signal.aggregated` audit kind registered + fires on every upsert + carries the 15 locked detail fields | _<smoke-evidence + sample JSON>_ | ☐ |
| C16 | Privacy canary: zero forbidden substrings (raw email patterns, subject keywords, body keywords) in any new memory_signal detail or audit detail | _<smoke-evidence C16 + sample scan>_ | ☐ |
| C17 | Cross-tenant carry-forward — only founder writes for new kinds; `smoke-evidence:v0.5.9` + `smoke-evidence:v0.5.10` still PASS or match documented benign shapes; v0.5.9 `sender_feedback_ignored` UNTOUCHED | _<v0.5.9 + v0.5.10 verdicts + memory_signals diff>_ | ☐ |
| C18 | Reversibility — delete `sender_importance` row → next aggregation event recreates (`memory_signal_action=created`); delete `sender_suppressed` row → suppression cleared until next flipping event | _<§6.4 query + audit row>_ | ☐ |

## 4. `smoke-evidence:v0.5.1` (substrate) — **VERDICT: _<…>_**

```
_<paste full output>_
```

## 5. `smoke-evidence:v0.5.2` (`FOMO_V0_5_2_WINDOW_HOURS=168`) — **VERDICT: _<…>_**

```
_<paste full output>_
```

## 6. `smoke-evidence:v0.5.3` — **VERDICT: _<…>_**

```
_<paste full output>_
```

## 7. `smoke-evidence:v0.5.4` (`FOMO_V0_5_4_WINDOW_HOURS=168`) — **VERDICT: _<…>_**

```
_<paste full output>_
```

## 8. `smoke-evidence:v0.5.5` — **VERDICT: _<…>_**

```
_<paste full output>_
```

## 9. `smoke-evidence:v0.5.6` — **VERDICT: _<…>_**

```
_<paste full output>_
```

## 10. `smoke-evidence:v0.5.7` (HMR regression check) — **VERDICT: _<…>_**

```
_<paste full output>_
```

> Operator note: same-day multi-smoke window pollution may produce C3 stale-template-leak FAIL (documented benign per v0.5.8/v0.5.9/v0.5.10 SMOKE_REPORT §10). HMR unchanged if v0.5.11 source has no renderer edits.

## 11. `smoke-evidence:v0.5.8` — **VERDICT: _<…>_**

```
_<paste full output>_
```

## 12. `smoke-evidence:v0.5.9` (Feedback substrate regression — LOAD-BEARING for C17) — **VERDICT: _<…>_**

```
_<paste full output>_
```

> Operator note: v0.5.9 substrate UNCHANGED — BREVIO_FEEDBACK_SURFACES=13, ACTIVE=['email_alert'], `sender_feedback_ignored` UNTOUCHED. Any v0.5.11 regression here is a blocker.

## 13. `smoke-evidence:v0.5.10` (Reply-parser regression — LOAD-BEARING for C17) — **VERDICT: _<…>_**

```
_<paste full output>_
```

> Operator note: reply-parser-v0.2.0 must remain the active PROMPT_VERSION; intent set must remain 8 (snooze, ignore, ignore_sender, why, false_positive, unclear, this_mattered, more_like_this).

## 14. `smoke-evidence:v0.5.11` output — **VERDICT: _<…>_**

```
_<paste full output>_
```

## 15. Shadow eval (Path D) output — **VERDICT: _<…>_**

```
_<paste full output of `pnpm --filter @brevio/fomo run eval:pil-shadow`>_
```

## 16. Operator-confirmed smoke evidence

| Check | Confirmed? | Notes |
|---|---|---|
| **Path A (LOAD-BEARING): founder texts "ignore this sender" → full producer + consumer chain on v0.5.11-rank-time alert** | ☐ | _<SMOKE_START_TS, alert_id, sender_email_hash, scope_key, sender_suppressed row id, sender_feedback_ignored row id, brevio.signal.aggregated audit row id>_ |
| Path A: `feedback.written` audit carries `sender_present=true` (was false in v0.5.10 due to bonus finding #1; now closed) | ☐ | _<sample JSON>_ |
| Path A: `brevio.signal.aggregated` detail carries all 15 locked fields | ☐ | _<sample JSON>_ |
| Path A: privacy canary clean on `brevio.signal.aggregated` + new memory_signal rows | ☐ | _<canary scan>_ |
| Path A: scope_key is 32-hex HMAC matching `alert.sender_email_hash` | ☐ | _<scope_key + regex>_ |
| Path A reversibility sub-step: delete row → re-fire aggregation → `memory_signal_action='created'` | ☐ | _<audit row JSON>_ |
| Path B.1 "this mattered" → `sender_importance.score=+δ`, `n_positive_events=1`, NO `sender_suppressed` write | ☐ | _<queries>_ |
| Path B.2 "more like this" → `sender_importance.score=+δ+2δ=+0.3`, `n_positive_events=2` | ☐ | _<queries>_ |
| Path C.1 3 consecutive `false_positive` → `sender_importance.score=-0.3`, `n_negative_events=3`, `sender_suppressed` flips on 3rd write (`suppression_flipped=true` in audit) | ☐ | _<queries + audit>_ |
| Path C.2 single positive on fresh synthetic sender → `score=+0.1`, NO `sender_suppressed` row | ☐ | _<queries>_ |
| **Path D shadow eval VERDICT: PASS** (per-fixture shift DIRECTION assertions) | ☐ | _<eval output tail>_ |
| **Path E cross-user contamination — DB layer (LOAD-BEARING)**: 0 rows for user B at user A's scope_key | ☐ | _<query>_ |
| Path E cross-user contamination — shadow eval row PASS (user B score unchanged by user A signal) | ☐ | _<eval output line>_ |
| §11 Live ranker invariant: `PROMPT_VERSION='ranker-v0.2.0'`; production call site passes `pil_context: null` (grep verified); `rank_results` schema unchanged | ☐ | _<grep + DB diff>_ |
| §12 v0.5.9 + v0.5.10 evidence scripts still PASS or match documented benign shapes | ☐ | _<v0.5.9 verdict + v0.5.10 verdict>_ |
| Code-level: `apps/fomo/src/memory/pil-aggregation.ts` exports `applyPilAggregation`; `apps/fomo/src/ranker/pil-context.ts` exports `buildPilContext`; `apps/fomo/src/eval/pil-shadow.eval.ts` present with ≥10 fixtures + cross-user fixture | ☐ | _<grep outputs>_ |
| Unit-test sanity: all new tests green (aggregation pipe, recency decay, scope_key HMAC, one-correction-no-flip, k-event-flip, cross-user isolation, audit detail 15 fields) | ☐ | _<test count + pass count>_ |

**Sample `sender_importance` memory_signal detail (Path B):**

```json
_<paste verbatim — should include score, n_positive_events, n_negative_events, last_updated, source_surface, source_feedback_event_ids>_
```

**Sample `sender_suppressed` memory_signal detail (Path A):**

```json
_<paste verbatim — should include suppressed:true, set_at, set_by='explicit_ignore_sender', source_surface, source_feedback_event_ids>_
```

**Sample `brevio.signal.aggregated` audit detail (Path A — suppression_flipped=true on first ignore_sender):**

```json
_<paste verbatim — all 15 locked fields: verb, dimension, feedback_event_id, source_surface, memory_signal_kind, memory_signal_action, memory_signal_scope_key_hash, score_before, score_after, score_delta, n_positive_events_before/after, n_negative_events_before/after, suppression_flipped, threshold_k_in_force>_
```

**Sample `alerts.sender_email_hash` row (proving rank-step thread):**

```
alert_id=_<uuid>_  sender_email_hash=_<32-hex>_  created_at=_<ts>_
```

**Cross-user contamination query result (Path E LOAD-BEARING):**

```
_<paste the 0-row query result that proves user B has no memory_signal at user A's scope_key>_
```

## 17. Founder observations

| Observation | Note |
|---|---|
| Does the recency decay model feel right (90/180 linear), or would exponential / shorter windows fit better? | _<…>_ |
| Does k=3 / δ=0.1 produce reasonable shift dynamics on synthetic fixtures? | _<…>_ |
| Was the shadow eval informative — did the shift directions match founder intuition on the synthetic fixtures? | _<…>_ |
| Anything in `brevio.signal.aggregated` audit detail that surprised you? | _<…>_ |
| Does v0.5.11 feel like the right shape for the v0.5.12 live ranker wiring to build on, or do field/structure changes need to happen first? | _<…>_ |
| Are the 4 tunable env vars well-named? Should any move to per-user config later? | _<…>_ |

### Bonus findings (real-incident-backed, candidates for next-phase 6Q gate)

1. _<…>_
2. _<…>_

## 18. Verdict

☐ **PASS** — all 18 criteria green; §6 Path A LOAD-BEARING chain succeeded; §10 Path E cross-user contamination test returned 0 rows at the DB layer AND shadow eval row passed; §9 shadow eval PASS; §11 live ranker invariant verified; §12 v0.5.9 + v0.5.10 evidence scripts still PASS or match documented benign shapes; privacy canary clean.

☐ **FAIL** — list below.

☐ **PENDING** — runtime commit(s) not yet on branch; re-run after runtime lands.

Failures / followups:

- _…_

## 19. Sign-off

- Founder signature: Galiette Mita
- Date: _<YYYY-MM-DD>_
- No friend consent needed this phase (founder-only smoke)

## 20. Aftercare confirmation

- [ ] Killed Terminal 1 dev server + Terminal 2 ngrok
- [ ] No friend deletion ops (no friend involved)
- [ ] v0.5.7 HMR template_version still `human-message-v0.3.0` (no renderer edits this phase — verified by `git diff main -- apps/fomo/src/render`)
- [ ] v0.5.9 substrate unchanged: BREVIO_FEEDBACK_SURFACES=13, ACTIVE=`['email_alert']`, `sender_feedback_ignored` rows untouched
- [ ] v0.5.10 reply-parser unchanged: `PROMPT_VERSION='reply-parser-v0.2.0'`, 8 intents
- [ ] No LLM call introduced into renderer (3E.1 invariant)
- [ ] No raw reply text / subject / body / snippet / headers / sender_email in any new audit or memory_signal detail (C16 canary PASS)
- [ ] Live ranker call site passes `pil_context: null` unconditionally (C13)
- [ ] No new active `source_surface` activated beyond `email_alert`

## 21. What v0.5.11 PASS does NOT promise

- **Live ranker reading PIL context** — own v0.5.12 6Q gate
- **HMR Feedback Acknowledgment / Feedback Prompt Surface** — own future phase (Q5.A defer from v0.5.10)
- **Positive-signal memory_signal kinds beyond `sender_importance` + `sender_suppressed`** (e.g. `topic_importance`, `commercial_kept`, `conditional_rule`) — each its own future gate per `[[personalized-importance-learning]]` §7.2
- **Activating any source_surface beyond `email_alert`** — each its own 6Q gate
- **F1 SendBlue tier fix / real SendBlue inbound proof**
- **Friend C onboarding** — three-friend cap
- **Autonomy tiers / auto-send / new tools / new modalities / production scale**
- **3E.1 reversal**
- **Brevio proposing rules to the user** (per doctrine §9.9 — silent learning only this phase)
- **"Why" answer surface** (per doctrine §14.5 — deferred)
- **User-facing preference inspection / editing surface** (per doctrine §14.6 — deferred)
- **Migration / rename of v0.5.9 `sender_feedback_ignored`** — own hardening item
- **Storing reply text in any column / detail field**
- **STOP/START as preference feedback**

The next phase is decided AT THE NEXT 6-question gate.
