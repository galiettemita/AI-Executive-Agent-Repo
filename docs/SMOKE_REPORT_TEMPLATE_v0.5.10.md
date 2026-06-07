# Phase v0.5.10 Smoke Test Report — Reply-parser feedback intents

> Filled after running every step in `smoke-test-v0.5.10-reply-parser-feedback.md`. Commit as `docs/SMOKE_REPORT_v0.5.10.md` once **`VERDICT: PASS`** on all 16 criteria AND §6 Test 1 (LOAD-BEARING ignore_sender natural reply) produced the full end-to-end chain AND §6 Test 2 (positive intent this_mattered) wrote the correct feedback_event shape without firing a memory_signal AND `smoke-evidence:v0.5.9` still PASSES on this branch.
>
> **Phase under the Core Dimension Check discipline:** advances Dim 8 (Feedback + Learn/Grow Loop) + Dim 4 (Agent Core + Reasoning) + Dim 10 (Observability/Reliability); preserves HMR (Dim 9), PIL/ranker behavior, autonomy/tool/multimodal/scale/trust dimensions; intentionally defers Dim 1 (Autonomy), Dim 5 (Tools), Dim 7 (Multimodal), Dim 11 (Scale). See [scope memory](../.claude/projects/-Users-galiettemita-Downloads-Executive-AI-Agent-backend/memory/project_v05-10-scope.md).
>
> **v0.5.10 PASS does NOT auto-unlock** PIL substrate, HMR Feedback Acknowledgment surface, positive-signal memory_signal kinds, any non-email_alert surface activation, F1 SendBlue tier fix, Friend C, autonomy tiers, auto-send, 3E.1 reversal. The next phase runs its own 6-question gate.

---

**Founder:** Galiette Mita
**Run date:** _<YYYY-MM-DD HH:MM TZ>_
**Branch:** `phase-v0.5.10-reply-parser-feedback`
**Scaffolding commit SHA:** _<sha>_
**Runtime commit SHA:** _<sha>_
**Smoke window override (if any):** `FOMO_V0_5_10_WINDOW_HOURS=24` (default)
**SMOKE_START_TS:** _<UTC ISO timestamp>_

---

## 1. Prerequisites confirmed

- [ ] PR #49 (v0.5.9 Feedback substrate) on `main` with VERDICT: PASS
- [ ] No friend involvement (three-friend cap holds)
- [ ] §1 baseline snapshots captured BEFORE smoke start (feedback_events count, sender_feedback_ignored rows for founder, inbound_replies count, stop_active rows, feedback.written rows with intent_source = 0)
- [ ] `BREVIO_SENDER_HASH_KEY` env var still set from v0.5.9 (reused this phase)
- [ ] No new env vars needed
- [ ] ngrok up + reachable (`FOMO_FRIEND_BETA_BASE_URL` points to the tunnel)

## 2. Env additions (redacted)

| Var | Set? | Notes |
|---|---|---|
| `FOMO_V0_5_10_BASELINE_CONFIRMED` | ☐ | set `true` after §1 capture |
| `FOMO_V0_5_10_WINDOW_HOURS` | ☐ | default 24 |

All other v0.5.x env vars unchanged.

## 3. PASS criteria (16 — Reply-parser feedback intents)

| # | Criterion | Evidence | Got |
|---|---|---|---|
| C1 | `feedback.written` audit detail carries the 10 locked fields (per Q6.A-modified) on reply-parser-routed rows | _<sample row JSON + smoke-evidence C1>_ | ☐ |
| C2 | Every reply-parser-routed `feedback_events` row has `source_surface='email_alert'` | _<DB query>_ | ☐ |
| C3 | `reply-parser PROMPT_VERSION === 'reply-parser-v0.2.0'`; validator accepts the 2 new intents | _<grep + smoke-evidence>_ | ☐ |
| C4 | All Q3.C explicit-feedback-phrase allowlist phrases classify via `parseReplyDeterministic` with `confidence=1.0` and expected intent (LLM never invoked) | _<unit-test output + sample smoke audit row with intent_source=reply_parser_deterministic>_ | ☐ |
| C5 | ≤3-word safe rule: 2-word non-allowlist reply → forced `unclear`; NO feedback_event written | _<unit-test output + Test 3 query>_ | ☐ |
| C6 | **Live test: `ignore_sender` natural reply → `applyFeedback` → `sender_feedback_ignored` memory_signal upserted + `brevio.feedback.applied` audit fires** (carry-forward v0.5.9 consumer arm) | _<Test 1 queries>_ | ☐ |
| C7 | Live test: `this_mattered` natural reply → `feedback_event(verb=approved, dimension=importance, role=user, value=confirmed_important)`; NO memory_signal write | _<Test 2 queries>_ | ☐ |
| C8 | Live test (OR unit test): `more_like_this` → `feedback_event(verb=approved, dimension=pattern, value=more_like_this)`; NO memory_signal write | _<sample row OR test output>_ | ☐ |
| C9 | Live test (OR unit test): `false_positive` (negative correction phrases) → `feedback_event(verb=corrected, dimension=ranker_label, previous_label=important, corrected_label=not_important)` | _<sample row OR test output>_ | ☐ |
| C10 | `unclear` intent → routing module returns `{kind: 'unclear_no_op'}` and writes nothing | _<unit-test output>_ | ☐ |
| C11 | Idempotency carry-forward: duplicate SendBlue webhook (same `provider_message_id`) → ONE feedback_event, ONE `applyFeedback` | _<unit-test output; v0.5.5 inbound_replies UNIQUE constraint intact>_ | ☐ |
| C12 | Cross-tenant: only founder rows in window; non-founder stop_active byte-identical to baseline | _<Test 5 queries + diff>_ | ☐ |
| C13 | Privacy canary: `feedback.written` detail + `brevio.feedback.applied` detail contain NO raw reply text / subject / body / snippet / headers / sender_email substrings | _<smoke-evidence C13 + sample canary scan>_ | ☐ |
| C14 | **Live smoke Path A (LOAD-BEARING)**: founder texts "ignore this sender" → full chain (deterministic match → routing → feedback_event + applyFeedback → sender_feedback_ignored upsert + brevio.feedback.applied audit) | _<Test 1 timestamps, IDs, sample JSON>_ | ☐ |
| C15 | Live smoke: founder texts "this mattered" → positive-signal feedback_event; NO memory_signal change; NO `brevio.feedback.applied` audit | _<Test 2 queries>_ | ☐ |
| C16 | Carry-forward: `smoke-evidence:v0.5.7` + `smoke-evidence:v0.5.9` still PASS or match documented benign shapes; STOP regression: STOP did NOT write a v0.5.10 feedback_event | _<v0.5.7 + v0.5.9 verdicts + Test 4 query>_ | ☐ |

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

> Operator note: v0.5.10 is a founder-only smoke — no `/onboard/callback`. If v0.5.3 reports Item #1 FAIL, that's expected blocked-substrate.

## 7. `smoke-evidence:v0.5.4` (`FOMO_V0_5_4_WINDOW_HOURS=168`) — **VERDICT: _<…>_**

```
_<paste full output>_
```

## 8. `smoke-evidence:v0.5.5` — **VERDICT: _<…>_**

```
_<paste full output>_
```

> Operator note: v0.5.5 C2/C3/C5/C11 may FAIL on SendBlue OPTED_OUT blocked-external — same shape as PR #43. Not a v0.5.10 regression.

## 9. `smoke-evidence:v0.5.6` — **VERDICT: _<…>_**

```
_<paste full output>_
```

## 10. `smoke-evidence:v0.5.7` (HMR regression check) — **VERDICT: _<…>_**

```
_<paste full output>_
```

> Operator note: same-day multi-smoke window pollution may produce C3 stale-template-leak FAIL (documented benign per v0.5.8/v0.5.9 SMOKE_REPORT §10). HMR untouched if v0.5.10 source has no renderer edits.

## 11. `smoke-evidence:v0.5.8` — **VERDICT: _<…>_**

```
_<paste full output>_
```

## 12. `smoke-evidence:v0.5.9` (Feedback substrate regression check) — **VERDICT: _<…>_**

```
_<paste full output>_
```

> Operator note (LOAD-BEARING for C16): v0.5.9 evidence MUST still PASS — BREVIO_FEEDBACK_SURFACES (13) + ACTIVE (`['email_alert']`) + sender_feedback_ignored memory_signal kind + privacy canary all intact.

## 13. `smoke-evidence:v0.5.10` output — **VERDICT: _<…>_**

```
_<paste full output>_
```

## 14. Operator-confirmed smoke evidence

| Check | Confirmed? | Notes |
|---|---|---|
| **Test 1 (LOAD-BEARING): founder texts "ignore this sender" → full chain end-to-end** | ☐ | _<SMOKE_START_TS, feedback_event IDs, scope_key_hash sample, applyFeedback audit row, memory_signal ignored_count>_ |
| Test 1: `feedback.written` audit carries all 10 locked fields | ☐ | _<sample row JSON>_ |
| Test 1: `brevio.feedback.applied` audit detail contains NO raw email / sender substring | ☐ | _<canary scan + sample JSON>_ |
| Test 1: memory_signal scope_key is HMAC-hashed hex (NOT plain email) — carry-forward v0.5.9 | ☐ | _<scope_key length + regex check>_ |
| Test 2: founder texts "this mattered" → positive-signal feedback_event; NO memory_signal write; NO applied audit | ☐ | _<sample JSON + count queries>_ |
| Test 3: ≤3-word safe rule fires; NO feedback_event for "got it" | ☐ | _<count query + reply_parsed audit>_ |
| Test 4 (STOP regression): STOP did NOT write a v0.5.10 feedback_event; existing deterministic compliance fired | ☐ | _<count query + stop_active row>_ |
| Test 5 (cross-tenant): zero non-founder writes; stop_active non-founder diff empty | ☐ | _<diff output>_ |
| (Path A fallback used?): if SendBlue inbound webhook didn't fire, signed-curl substitute was used — note the curl command + outcome | ☐ | _<yes / no + notes>_ |
| Code-level: PROMPT_VERSION === 'reply-parser-v0.2.0'; feedback-routing.ts exports routeReplyFeedback; allowlist absorbed into deterministic.ts | ☐ | _<grep outputs>_ |
| Unit-test sanity: all new tests green (allowlist phrases, ≤3-word safe rule, 8 routing arms, idempotency, cross-tenant, privacy canary) | ☐ | _<test count + pass count>_ |

**Sample `feedback.written` detail JSON (Test 1 — ignore_sender deterministic):**

```json
_<paste verbatim — should include intent_source='reply_parser_deterministic', parser_intent='ignore_sender', parser_confidence=1.0, source_surface='email_alert', verb='ignored', dimension='sender', role='user', feedback_event_id, inbound_reply_id>_
```

**Sample `feedback.written` detail JSON (Test 2 — this_mattered):**

```json
_<paste verbatim — should include intent_source='reply_parser_deterministic', parser_intent='this_mattered', verb='approved', dimension='importance', role='user'>_
```

**Sample `brevio.feedback.applied` detail JSON (Test 1):**

```json
_<paste verbatim — should include feedback_event_id, memory_signal_kind='sender_feedback_ignored', memory_signal_action, memory_signal_scope_key_hash (NEVER raw sender_email)>_
```

**Sample `memory_signals(sender_feedback_ignored)` detail JSON + scope_key snippet:**

```json
_<paste verbatim — detail has ignored_count, first/last_ignored_at, source_feedback_event_ids, source_surface='email_alert'>_
```

`scope_key` (first 8 chars + regex check): _<e.g. `eb678bce…` matches /^[0-9a-f]{32}$/ ✓>_

## 15. Founder observations

| Observation | Note |
|---|---|
| Does "ignore this sender" → silent action feel right, or does the lack of acknowledgment feel weird? | _<…>_ |
| How does the LLM classifier behave for natural variations not in the allowlist (e.g. "actually that was super useful")? | _<…>_ |
| Did the ≤3-word safe rule produce any false-negatives (real feedback intent silently dropped)? | _<…>_ |
| Is `dimension='pattern'` the right name for `more_like_this`, or should it be `'sender_or_topic'` per the founder Q2 note? | _<…>_ |
| Anything else in audit_log that surprised you? | _<…>_ |
| Does v0.5.10 feel like the right shape for the future PIL / HMR-acknowledgment phases to build on? | _<…>_ |

### Bonus findings (real-incident-backed, candidates for next-phase 6Q gate)

1. _<…>_
2. _<…>_

## 16. Verdict

☐ **PASS** — all 16 criteria green; §6 Test 1 LOAD-BEARING (ignore_sender → full chain) and Test 2 (positive intent) succeeded; cross-tenant non-founder rows byte-identical; privacy canary clean. **Next phase runs its own 6-question gate.**

☐ **FAIL** — list below.

☐ **PENDING** — runtime commit not yet on branch; re-run after runtime lands.

Failures / followups:

- _…_

## 17. Sign-off

- Founder signature: Galiette Mita
- Date: _<YYYY-MM-DD>_
- No friend consent needed this phase (founder-only smoke)

## 18. Aftercare confirmation

- [ ] Kill Terminal 1 dev server + Terminal 2 ngrok
- [ ] If Test 4 (STOP) left founder in `stop_active=true`, founder texted START before this step. Verify outbound substrate restored.
- [ ] No friend deletion ops (no friend involved)
- [ ] v0.5.7 HMR template_version still `human-message-v0.3.0`
- [ ] v0.5.9 substrate unchanged: BREVIO_FEEDBACK_SURFACES (13) + ACTIVE (['email_alert']) + sender_feedback_ignored memory_signal kind intact
- [ ] No LLM call accidentally introduced into renderer (3E.1 invariant)
- [ ] No raw reply text / subject / body / snippet / headers / sender_email in any new audit detail (C13 canary PASS)

## 19. What v0.5.10 PASS does NOT promise

- **PIL substrate** — own future phase
- **HMR Feedback Acknowledgment / Feedback Prompt Surface** — own future phase per Q5.A defer
- **Positive-signal memory_signal kinds** (`sender_feedback_positive` / `alert_corrected_positive`) — own future phase per Q4.A defer
- **Activating any source_surface beyond `email_alert`** — each its own 6Q gate
- **F1 SendBlue tier fix**
- **Friend C onboarding** — three-friend cap
- **Autonomy tiers / auto-send / new tools / new modalities / production scale**
- **3E.1 reversal**
- **Per-intent confidence calibration**
- **Storing reply text in any column / detail field**
- **STOP/START as preference feedback**

The next phase is decided AT THE NEXT 6-question gate.
