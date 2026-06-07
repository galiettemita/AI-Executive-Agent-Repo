# Phase v0.5.10 Smoke Test Report — Reply-parser feedback intents

> **VERDICT: PASS** — all 16 criteria green (8 PASS by smoke-evidence, 8 operator-confirmed); §6 Test 1a LOAD-BEARING (natural reply "ignore this sender" → deterministic-allowlist match → routing module → feedback_event + feedback.written audit) succeeded; §6 Test 1b (consumer arm via `ops:feedback-inject` substitute) confirmed `sender_feedback_ignored` upsert + `brevio.feedback.applied` audit fire with HMAC-hashed `scope_key`; §6 Test 2 ("this mattered") wrote positive-signal `feedback_event` with NO memory_signal write; §6 Test 3 ("got it") forced unclear with NO feedback_event written; §6 Test 4 (STOP) stayed on the existing deterministic compliance path and wrote NO v0.5.10 feedback.written audit; §6 Test 5 cross-tenant isolation byte-clean; `smoke-evidence:v0.5.9` (Feedback substrate regression) still PASSES; privacy canary clean across 8 new audit rows + 1 new memory_signal row.
>
> **Phase under the Core Dimension Check discipline:** advances Dim 8 (Feedback + Learn/Grow Loop) + Dim 4 (Agent Core + Reasoning) + Dim 10 (Observability/Reliability); preserves HMR (Dim 9), PIL/ranker behavior, autonomy/tool/multimodal/scale/trust dimensions; intentionally defers Dim 1, Dim 5, Dim 7, Dim 11. See [scope memory](../.claude/projects/-Users-galiettemita-Downloads-Executive-AI-Agent-backend/memory/project_v05-10-scope.md).
>
> **v0.5.10 PASS does NOT auto-unlock** PIL substrate, HMR Feedback Acknowledgment surface, positive-signal `memory_signal` kinds, any non-`email_alert` surface activation, F1 SendBlue tier fix, Friend C, autonomy tiers, auto-send, 3E.1 reversal. The next phase runs its own 6-question gate.

---

**Founder:** Galiette Mita
**Run date:** 2026-06-07 00:41 UTC
**Branch:** `phase-v0.5.10-reply-parser-feedback`
**Scaffolding commit SHA:** `e5e6a397`
**Runtime commit SHA:** `fb242415`
**Smoke window override:** none (default `FOMO_V0_5_10_WINDOW_HOURS=24`)
**SMOKE_START_TS:** `2026-06-07T00:41:09Z`

---

## 1. Prerequisites confirmed

- [x] PR #49 (v0.5.9 Feedback substrate) on `main` with VERDICT: PASS — `smoke-evidence:v0.5.9` still PASSES on this branch (see §12)
- [x] No friend involvement (three-friend cap holds)
- [x] §1 baseline snapshots captured BEFORE smoke start: 30 feedback_events, 1 existing `sender_feedback_ignored`, 16 inbound_replies, 3 non-founder `stop_active` (background tenant state), 0 rows with `intent_source` field
- [x] `BREVIO_SENDER_HASH_KEY` env var still set from v0.5.9 (reused this phase)
- [x] No new env vars needed
- [x] ngrok N/A — Path A used signed-curl substitute (header `sb-signing-secret`)

## 2. Env additions (redacted)

| Var | Set? | Notes |
|---|---|---|
| `FOMO_V0_5_10_BASELINE_CONFIRMED` | ☑ | set to `true` after §1 capture |
| `FOMO_V0_5_10_WINDOW_HOURS` | ☐ | default 24 used |

All other v0.5.x env vars unchanged.

## 3. PASS criteria (16 — Reply-parser feedback intents)

| # | Criterion | Evidence | Got |
|---|---|---|---|
| C1 | `feedback.written` audit detail carries the 10 locked fields on reply-parser-routed rows | smoke-evidence: 2 reply-parser-routed rows; sample carries `intent_source=reply_parser_deterministic`, `parser_intent=ignore_sender`, `parser_confidence=1`, `source_surface=email_alert`, `verb=ignored`, `dimension=sender`, `role=user`, `legacy_kind=null`, `feedback_event_id=31`, `inbound_reply_id=18`, `sender_present=false` | ☑ |
| C2 | Every reply-parser-routed `feedback_events` row has `source_surface='email_alert'` | smoke-evidence: 2/2 rows checked, all `email_alert` | ☑ |
| C3 | `reply-parser PROMPT_VERSION === 'reply-parser-v0.2.0'`; validator accepts the 2 new intents | smoke-evidence: current = `reply-parser-v0.2.0` | ☑ |
| C4 | All Q3.C explicit-feedback-phrase allowlist phrases classify via `parseReplyDeterministic` with confidence=1.0 (LLM never invoked) | smoke-evidence: 2 deterministic-source rows each at `parser_confidence=1.0`; both Test 1a + Test 2 audit rows show `intent_source=reply_parser_deterministic` despite LLM-disabled-stub on `OPENAI_API_KEY` absence | ☑ |
| C5 | ≤3-word safe rule: 2-word non-allowlist reply → forced unclear; NO feedback_event written | smoke-evidence + Test 3 live: "got it" → response `intent=unclear`, feedback_events count UNCHANGED from 3 (no new event), `fomo.sendblue.reply_unclear` audit fired at 00:46:05 | ☑ |
| C6 | Live: `ignore_sender` → `applyFeedback` → `sender_feedback_ignored` memory_signal upserted + `brevio.feedback.applied` audit fires | Test 1b (ops-inject with explicit sender substitute, per documented v0.1 substrate gap): `memory_signals.id=19` created (scope_key=`d910452f90985e27e8740882a0323625`, `ignored_count=1`, `confidence=0.6`); `brevio.feedback.applied` audit fired at 00:45:21 | ☑ |
| C7 | Live: `this_mattered` → `feedback_event(verb=approved, dimension=importance, role=user, value=confirmed_important)`; NO memory_signal write | Test 2 live: `feedback_events.id=33`, detail `{role:user, value:confirmed_important, dimension:importance}`; memory_signals count UNCHANGED | ☑ |
| C8 | `more_like_this` mapping covered | Unit-test suite (`feedback-routing.test.ts`); intent absent from live smoke window (acceptable per smoke-evidence C8 note "OPTIONAL during smoke"); routing module `INTENT_MAPPING` table verified at `apps/fomo/src/reply-parser/feedback-routing.ts:32-39` | ☑ |
| C9 | `false_positive` mapping covered | Unit-test suite (`feedback-routing.test.ts`); intent absent from live smoke window (acceptable per smoke-evidence C9 note "OPTIONAL"); routing module `INTENT_MAPPING` table verified | ☑ |
| C10 | `unclear` → routing returns `{kind:'unclear_no_op'}` writes nothing | Test 3 live: response `intent=unclear`, no feedback.written audit row appeared for Test 3 inbound_reply, no new feedback_event row written. `fomo.sendblue.reply_unclear` audit fired in lieu | ☑ |
| C11 | Idempotency carry-forward (duplicate webhook → one feedback_event) | Substrate: `inbound_replies.provider_message_id UNIQUE` constraint intact (verified `\d inbound_replies`); v0.5.5 dedup test still green in unit suite | ☑ |
| C12 | Cross-tenant: only founder rows in window; non-founder rows byte-identical to baseline | Test 5 queries: 0 non-founder `feedback.written` rows, 0 non-founder `sender_feedback_ignored` rows, 0 non-founder `brevio.feedback.applied` rows. Background non-founder audit traffic = oauth.refresh_failed (37) + gmail.poll.event_observed (9) + policy.decided (9) + tool.invoked (9) + oauth.refreshed (2) + poll.skipped_stop_active (1) — none v0.5.10-related | ☑ |
| C13 | Privacy canary: NO raw reply text / subject / body / snippet / headers / sender_email substrings in new audit detail | smoke-evidence C13: scanned 8 `feedback.written` + `brevio.feedback.applied` rows, checked 11 forbidden substrings, ZERO hits. Sample `brevio.feedback.applied` detail (Test 1b) carries `memory_signal_scope_key_hash` only, never the raw `smoke-noisy-newsletter+v0.5.10-test1b@example.com` | ☑ |
| C14 | Live smoke Path A (LOAD-BEARING) | Test 1a live via signed-curl: `MSG_ID=sb-smoke-v0.5.10-test1a-1780792924`; producer arm fired (feedback_events.id=31 + feedback.written audit with all 10 locked fields). Consumer arm correctly `no_match`'d because the auto-matched `aa51b11f-...-d1a010` alert lacks `sender_email` (v0.1 substrate gap from 3D.1 privacy design). Test 1b proved the consumer arm fires when `sender_email` IS present (see C6) | ☑ |
| C15 | Live smoke `this_mattered` → positive-signal feedback_event; NO memory_signal change; NO `brevio.feedback.applied` audit | Test 2 live: response `intent=this_mattered`, feedback_events.id=33 written with positive shape, memory_signals count UNCHANGED, `brevio.feedback.applied` count stayed at 1 (only Test 1b's) | ☑ |
| C16 | Carry-forward: `smoke-evidence:v0.5.7` + `smoke-evidence:v0.5.9` still PASS or match documented benign shapes; STOP did NOT write a v0.5.10 feedback.written audit | v0.5.9 PASS (§12); v0.5.7 FAIL matches documented benign stale-template-leak shape (§10, see operator note). STOP regression: Test 4 wrote a legacy `kind=stop` `feedback_events` row (pre-existing substrate behavior — 7 historical `kind=stop` rows pre-date this branch) but NO v0.5.10 `feedback.written` audit (verified by `feedback_event_id=34` absent from feedback.written audits) — STOP stayed on deterministic compliance path as required | ☑ |

## 4. `smoke-evidence:v0.5.1` (substrate) — **VERDICT: PASS**

```
VERDICT: PASS  (operator must additionally confirm friend-safe Slack card was rendered visually + clean-stop refused /onboard with the switch off)
```

## 5. `smoke-evidence:v0.5.2` (`FOMO_V0_5_2_WINDOW_HOURS=168`) — **VERDICT: PASS**

```
2 founder approved→sent transition(s) in the smoke window
Leak-canary scan — no forbidden substrings in persisted detail
scanned 2000 audit + 8 memory + 67 transition rows; zero hits across 3 canary substring(s)
VERDICT: PASS
```

## 6. `smoke-evidence:v0.5.3` — **VERDICT: FAIL (documented blocked-external)**

```
Leak-canary scan: no raw secrets / connection strings in audit detail (PASS)
scanned 1997 audit rows; zero hits
VERDICT: FAIL — 1 required criterion(criteria) failed.
```

> Operator note: v0.5.10 is a founder-only smoke — no `/onboard/callback`. Item #1 FAIL is the documented blocked-substrate pattern, not a v0.5.10 regression.

## 7. `smoke-evidence:v0.5.4` (`FOMO_V0_5_4_WINDOW_HOURS=168`) — **VERDICT: FAIL (documented blocked-external)**

```
C16 (NEW): v0.5.3 hardening still functional (registry intact + contact lifecycle fired)
7/7 hardening audits registered; sendblue_contact_status kind registered; 2 contact-lifecycle audit row(s) fired during Friend B onboarding.
VERDICT: FAIL — 2 required v0.5.4 criterion(criteria) failed.
```

> Operator note: 2-criterion FAIL matches the documented prior-phase shape (no Friend B fresh onboarding this smoke). v0.5.3 hardening still intact (C16 PASS).

## 8. `smoke-evidence:v0.5.5` — **VERDICT: FAIL (documented benign — no founder STOP confirmation in window)**

```
No stop_confirmation_sent row for actor_user_id='founder'.
```

> Operator note: Test 4 fired `STOP` but `fomo.sendblue.stop_confirmation_failed` fired instead of `stop_confirmation_sent` — the SendBlue dev-env path can't deliver to the founder phone in this smoke configuration. This matches the documented SendBlue OPTED_OUT blocked-external shape from PR #43 / SMOKE_REPORT_v0.5.6.md. The deterministic compliance write itself (`fomo.sendblue.stop_recorded`) DID fire. **Not a v0.5.10 regression.**

## 9. `smoke-evidence:v0.5.6` — **VERDICT: PASS**

```
2 founder send row(s); all on bumped template_version.
VERDICT: PASS
```

## 10. `smoke-evidence:v0.5.7` (HMR regression check) — **VERDICT: FAIL (documented benign — window-pollution stale template leak)**

```
STALE TEMPLATE LEAK — 1/2 rows still on 'founder-text-v0.2.0'. Either outbound path bypassed renderHumanMessage, or the bump did not land.
VERDICT: FAIL — at least one criterion failed.
```

> Operator note: identical shape to the documented benign window-pollution pattern in v0.5.8/v0.5.9 SMOKE_REPORTs (§10). The 1 stale `founder-text-v0.2.0` row is a pre-v0.5.7 send still inside the 24h window. **v0.5.10 source has no renderer edits** — confirmed by `git diff main -- apps/fomo/src/render apps/fomo/src/human-message-renderer.ts` showing zero touches outside `reply-parser/`, `routes/sendblue-inbound.ts`, `dispatch/internal-executors.ts`, `memory/feedback-events.ts`, `routes/slack-interactivity.ts`, `scripts/ops-feedback-inject.ts`, `index.ts`. Not a v0.5.10 regression.

## 11. `smoke-evidence:v0.5.8` — **VERDICT: FAIL (documented benign — non-founder polling in window)**

```
KEY METRIC (labelAdded_only) reflects messages v0.5.7 would have missed: 1
CROSS-TENANT VIOLATION — non-founder gmail.poll.event_observed rows: 11.
VERDICT: FAIL — at least one criterion failed.
```

> Operator note: non-founder `gmail.poll.event_observed` rows are background tenant polling (Friend B / Morris / synthetic test users) — same shape as documented in v0.5.9 SMOKE_REPORT §11. The v0.5.8 reliability metric itself (labelAdded_only counter active) is functioning. **Not a v0.5.10 regression.**

## 12. `smoke-evidence:v0.5.9` (Feedback substrate regression check) — **VERDICT: PASS** (LOAD-BEARING for C16)

```
13 PASS, 3 operator-confirmed
surfaces=13 (13 expected); active=[email_alert]
required=6/6; opened=deferred
7 feedback.written success rows; 1 inactive_surface reject row
sender_feedback_ignored: 2 rows; sample ignored_count=1 source_surface=email_alert confidence=0.6
brevio.feedback.applied: 4 rows; sample kind=sender_feedback_ignored action=created surface=email_alert
Slack interactivity: 1 row from approval path
Privacy canary: 7 forbidden substrings; zero hits
VERDICT: PASS
```

> v0.5.9 substrate UNCHANGED: 13 surfaces / `email_alert` active / 6 required generic kinds / `sender_feedback_ignored` memory_signal / privacy canary all intact.

## 13. `smoke-evidence:v0.5.10` output — **VERDICT: PASS**

```
[✓] C1: feedback.written detail carries 10 locked fields (per Q6.A-modified)
      2 reply-parser-routed row(s). sample fields present: intent_source, parser_intent,
      parser_confidence, source_surface, verb, feedback_event_id; intent_source=reply_parser_deterministic,
      parser_intent=ignore_sender, parser_confidence=1
[✓] C2: every reply-parser-routed feedback_events row has source_surface=email_alert
      2/2 rows checked; all source_surface=email_alert
[✓] C3: reply-parser PROMPT_VERSION === 'reply-parser-v0.2.0'
      current: 'reply-parser-v0.2.0'
[✓] C4: Q3.C explicit-feedback-phrase allowlist routes through parseReplyDeterministic
      2 deterministic-source row(s); each has parser_confidence=1.0 by construction
[!] C5: ≤3-word safe rule — operator + unit-test confirmed; Test 3 live confirms absence of feedback_event
[✓] C6: ignore_sender intent → brevio.feedback.applied fires for sender_feedback_ignored
      1 ignore_sender feedback.written row(s); 4 brevio.feedback.applied row(s) for sender_feedback_ignored
[✓] C7: this_mattered → feedback_event(verb=approved, dimension=importance, role=user)
      1 row(s); sample: verb=approved dimension=importance role=user
[!] C8: more_like_this — OPTIONAL during smoke; unit-test verified
[!] C9: false_positive — OPTIONAL during smoke; unit-test verified
[!] C10: unclear no_op — operator + unit-test confirmed
[!] C11: idempotency — substrate carry-forward (inbound_replies UNIQUE + v0.5.5 dedup test)
[✓] C12: cross-tenant — only founder feedback_events + sender_feedback_ignored writes in smoke window
      0 non-founder feedback.written rows; 0 non-founder sender_feedback_ignored rows
[✓] C13: privacy canary scan — zero forbidden substrings
      scanned 8 feedback.written + brevio.feedback.applied rows; checked 11 forbidden substring(s)
[!] C14: Live smoke Path A — OPERATOR-CONFIRMED in §6 Test 1
[!] C15: Live smoke Test 2 — OPERATOR-CONFIRMED in §6 Test 2
[!] C16: smoke-evidence:v0.5.7 + smoke-evidence:v0.5.9 carry-forward + STOP regression

VERDICT: PASS  (8 PASS, 8 operator-confirmed)
```

## 14. Operator-confirmed smoke evidence

| Check | Confirmed? | Notes |
|---|---|---|
| **Test 1a (LOAD-BEARING): founder texts "ignore this sender" → producer arm + audit chain** | ☑ | `MSG_ID=sb-smoke-v0.5.10-test1a-1780792924` @ 00:42:04Z. `feedback_events.id=31` written; `feedback.written` audit detail carries `intent_source=reply_parser_deterministic`, `parser_intent=ignore_sender`, `parser_confidence=1`, `inbound_reply_id=18`, `feedback_event_id=31` |
| Test 1a: `feedback.written` audit carries all 10 locked fields | ☑ | See sample JSON below — all 10 fields present + 2 extras (`legacy_kind`, `feedback_event_id`) |
| Test 1b (consumer arm substitute via `ops:feedback-inject` — same pattern as v0.5.9 Test 2): explicit `sender_email` → `applyFeedback` fires | ☑ | `feedback_events.id=32`; `memory_signals.id=19` created (`sender_feedback_ignored`, `scope_key=d910452f90985e27e8740882a0323625`, `ignored_count=1`, `confidence=0.6`); `brevio.feedback.applied` audit fired @ 00:45:21Z |
| Test 1b: `brevio.feedback.applied` audit detail contains NO raw email / sender substring | ☑ | Canary scan: detail carries `memory_signal_scope_key_hash` (32-hex HMAC) only, never `smoke-noisy-newsletter+v0.5.10-test1b@example.com` |
| Test 1b: memory_signal `scope_key` is HMAC-hashed hex (NOT plain email) — carry-forward v0.5.9 | ☑ | `d910452f90985e27e8740882a0323625` matches `/^[0-9a-f]{32}$/` ✓ |
| Test 2: founder texts "this mattered" → positive-signal feedback_event; NO memory_signal write; NO applied audit | ☑ | `MSG_ID=sb-smoke-v0.5.10-test2-1780793137` @ 00:45:37Z. `feedback_events.id=33` (verb=approved, dimension=importance, value=confirmed_important). memory_signals count UNCHANGED (Q4.A defer holds). `brevio.feedback.applied` count UNCHANGED at 1 |
| Test 3: ≤3-word safe rule fires; NO feedback_event for "got it" | ☑ | `MSG_ID=sb-smoke-v0.5.10-test3-1780793162` @ 00:46:05Z. Response `intent=unclear`. feedback_events count UNCHANGED at 3 after Test 3. `fomo.sendblue.reply_unclear` audit fired |
| Test 4 (STOP regression): STOP did NOT write a v0.5.10 feedback.written audit; existing deterministic compliance fired | ☑ | `MSG_ID=sb-smoke-v0.5.10-test4-1780793227` @ 00:47:08Z. Response `intent=stop, source=deterministic`. `fomo.sendblue.stop_recorded` audit fired. Legacy `kind=stop` feedback_event row written by pre-existing applyStop path (7 historical kind=stop rows pre-date this branch — substrate carry-forward, NOT a v0.5.10 write). NO `feedback.written` audit with `feedback_event_id=34`. `fomo.sendblue.stop_confirmation_failed` fired (SendBlue dev-env block — documented benign per §8) |
| Test 5 (cross-tenant): zero non-founder writes; non-founder `feedback_events` + `memory_signals` + `feedback.written` + `brevio.feedback.applied` all empty | ☑ | 0 non-founder feedback_events; 0 non-founder memory_signals; non-founder audit traffic = OAuth/polling/policy only |
| (Path A fallback used?): signed-curl substitute (header `sb-signing-secret`) used in lieu of ngrok+SendBlue inbound webhook | ☑ | All 4 webhook tests used `curl -X POST /sendblue/inbound -H "sb-signing-secret: $SENDBLUE_WEBHOOK_SECRET"`. Runbook's openssl HMAC example is stale (auth is plain header equality; flagged in §15 Bonus Findings) |
| Code-level: PROMPT_VERSION === 'reply-parser-v0.2.0'; `feedback-routing.ts` exports `routeReplyFeedback`; allowlist absorbed into `deterministic.ts` | ☑ | `apps/fomo/src/reply-parser/prompt.ts` line 1: `export const PROMPT_VERSION = 'reply-parser-v0.2.0'`; `feedback-routing.ts:routeReplyFeedback` present; SOFT_ALLOWLIST (26 phrases) in `deterministic.ts` |
| Unit-test sanity: all new tests green (allowlist phrases, ≤3-word safe rule, 8 routing arms, idempotency, cross-tenant, privacy canary) | ☑ | Pre-runtime push CI green; `git log fb242415` runtime commit landed with green checks |

**Sample `feedback.written` detail JSON (Test 1a — `ignore_sender` deterministic):**

```json
{
    "role": "user",
    "verb": "ignored",
    "dimension": "sender",
    "legacy_kind": null,
    "intent_source": "reply_parser_deterministic",
    "parser_intent": "ignore_sender",
    "sender_present": false,
    "source_surface": "email_alert",
    "inbound_reply_id": 18,
    "feedback_event_id": 31,
    "parser_confidence": 1
}
```

**Sample `feedback.written` detail JSON (Test 2 — `this_mattered`):**

```json
{
    "role": "user",
    "verb": "approved",
    "dimension": "importance",
    "legacy_kind": null,
    "intent_source": "reply_parser_deterministic",
    "parser_intent": "this_mattered",
    "sender_present": false,
    "source_surface": "email_alert",
    "inbound_reply_id": 19,
    "feedback_event_id": 33,
    "parser_confidence": 1
}
```

**Sample `brevio.feedback.applied` detail JSON (Test 1b — consumer arm proof):**

```json
{
    "verb": "ignored",
    "dimension": "sender",
    "confidence": 0.6,
    "source_surface": "email_alert",
    "feedback_event_id": 32,
    "memory_signal_kind": "sender_feedback_ignored",
    "memory_signal_action": "created",
    "memory_signal_scope_key_hash": "d910452f90985e27e8740882a0323625"
}
```

**Sample `memory_signals(sender_feedback_ignored)` detail JSON + scope_key (Test 1b):**

```json
{
    "ignored_count": 1,
    "source_surface": "email_alert",
    "last_ignored_at": "2026-06-07T00:45:21.450Z",
    "first_ignored_at": "2026-06-07T00:45:21.450Z",
    "source_feedback_event_ids": [32]
}
```

`scope_key`: `d910452f90985e27e8740882a0323625` (32 hex chars, matches `/^[0-9a-f]{32}$/` ✓)

## 15. Founder observations

| Observation | Note |
|---|---|
| Does "ignore this sender" → silent action feel right, or does the lack of acknowledgment feel weird? | Q5.A (no HMR feedback prompt / acknowledgment) deferred per scope lock. Silent is acceptable for v0.5.10; HMR Feedback Acknowledgment surface is its own future phase. The deterministic-allowlist path returning `200 ok` with the JSON body is the only "ack" the founder sees. |
| How does the LLM classifier behave for natural variations not in the allowlist? | Live smoke did not exercise non-allowlist phrases (Tests 1–4 all used canonical allowlist phrases by founder direction). Unit-test fixtures cover natural variations. |
| Did the ≤3-word safe rule produce any false-negatives (real feedback intent silently dropped)? | Not observed in Test 3 ("got it" was correctly ambiguous). Pass 4 forces unclear ONLY when reply ≤3 words AND not deterministic match AND classifier picked non-unclear — narrow window. |
| Is `dimension='pattern'` the right name for `more_like_this`, or should it be `'sender_or_topic'` per the founder Q2 note? | Locked as `dimension='pattern'` for v0.5.10. Founder Q2 note logged in [scope memory](../.claude/projects/-Users-galiettemita-Downloads-Executive-AI-Agent-backend/memory/project_v05-10-scope.md) as a future rename candidate for PIL substrate phase. |
| Anything else in audit_log that surprised you? | (a) The legacy `fomo.sendblue.reply_parsed` audit's `intent_source` field still hardcodes `'classifier'` even when the parser actually went deterministic (allowlist match). The new v0.5.10 `feedback.written` audit's `intent_source` is correct (`reply_parser_deterministic`). Minor: legacy audit field is misleading. (b) Test 1a's response body shows `source:"classifier"` for the same reason — `RouteOutcome.source` on the soft-feedback path is hardcoded in `applyIgnoreSender`. The DB audit is the source of truth. |
| Does v0.5.10 feel like the right shape for the future PIL / HMR-acknowledgment phases to build on? | Yes — the routing module is a single policy chokepoint, the 10-field audit is queryable, the Q4.A defer (only `ignore_sender` fires `applyFeedback`) keeps future positive-signal additions scoped to one switch in `INTENT_MAPPING.fires_apply_feedback`. |

### Bonus findings (real-incident-backed, candidates for next-phase 6Q gate)

1. **v0.1 substrate gap — `alerts` and `rank_results` don't carry `sender_email`** (3D.1 privacy design). Test 1a's producer arm fired but consumer arm `no_match`'d because the auto-matched alert lacked `sender_email`. For `ignore_sender` to bind a real `memory_signal` from a SendBlue natural reply (not an ops-inject), the sender_email needs to flow from the rank step into the alert. Candidate for PIL substrate phase: thread `sender_email` from `rank_results` → `alerts` so the reply-parser path can hash it.
2. **Legacy `fomo.sendblue.reply_parsed` audit `intent_source` field is hardcoded `'classifier'`** even when the parser went deterministic. Minor — the v0.5.10 `feedback.written` audit reports correctly. Candidate: align the legacy audit with the new convention, or remove the misleading field.
3. **SendBlue webhook auth is plain header equality (`sb-signing-secret`), NOT HMAC.** Runbook had an openssl HMAC example that doesn't match the actual verifier (`apps/fomo/src/routes/sendblue-inbound.ts` checks header equality). Updated curl commands worked. Candidate: update runbook §6 sample command.
4. **Cosmetic: `RouteOutcome.source` field returned in the JSON response body hardcodes `'classifier'` for all soft intents** in `applyIgnoreSender` / `applyThisMattered` / `applyMoreLikeThis` / `applyFalsePositive` etc., even on allowlist matches. Internal DB audit is correct; only the cosmetic HTTP response is misleading. Candidate: thread `result.source` through `RouteOutcome.source` for symmetry.

## 16. Verdict

**☑ PASS** — all 16 criteria green; §6 Test 1a + 1b (LOAD-BEARING `ignore_sender` chain: producer arm via natural reply + consumer arm via ops-inject substitute per documented v0.1 sender_email gap) succeeded; §6 Test 2 (positive intent) wrote correct positive shape with no memory_signal write; §6 Test 3 (≤3-word safe rule) forced unclear with no feedback_event; §6 Test 4 (STOP regression) stayed on deterministic compliance path with no v0.5.10 `feedback.written` audit; §6 Test 5 cross-tenant non-founder rows byte-identical to baseline; privacy canary clean (8 audits + 1 memory_signal, 11 forbidden substrings, 0 hits); `smoke-evidence:v0.5.9` PASSES (substrate untouched); all carry-forward FAILs match documented benign shapes from prior SMOKE_REPORTs.

☐ FAIL
☐ PENDING

**Next phase runs its own 6-question gate.**

## 17. Sign-off

- Founder signature: Galiette Mita
- Date: 2026-06-07
- No friend consent needed this phase (founder-only smoke)

## 18. Aftercare confirmation

- [x] Killed Terminal 1 dev server (background task `bavropfdj` stopped)
- [x] Terminal 2 ngrok N/A (signed-curl substitute used)
- [x] Test 4 STOP confirmation send failed (documented benign per §8) — founder consent table remains empty (founder consent is permissive-by-default in dev; no manual START needed)
- [x] No friend deletion ops (no friend involved)
- [x] v0.5.7 HMR template_version still `human-message-v0.3.0` (no renderer edits this phase — verified by git diff)
- [x] v0.5.9 substrate unchanged: BREVIO_FEEDBACK_SURFACES (13) + ACTIVE (`['email_alert']`) + `sender_feedback_ignored` memory_signal kind intact (verified by §12 PASS)
- [x] No LLM call introduced into renderer (3E.1 invariant — no edits to `apps/fomo/src/render/*` or `human-message-renderer.ts`)
- [x] No raw reply text / subject / body / snippet / headers / sender_email in any new audit detail (C13 canary PASS — 0/11 hits across 8 rows)

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
- **Threading `sender_email` from `rank_results` → `alerts`** (bonus finding #1; required before `ignore_sender` via natural reply can bind a real memory_signal — own future phase, candidate for PIL substrate)

The next phase is decided AT THE NEXT 6-question gate.
