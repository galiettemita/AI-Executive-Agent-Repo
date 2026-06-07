# Phase v0.5.12 Smoke Test Report — Live Ranker Reads PIL in Guarded Mode

> Filled after running every step in `smoke-test-v0.5.12-live-ranker-pil-guarded.md`. Commit as `docs/SMOKE_REPORT_v0.5.12.md` once **`VERDICT: PASS`** on all 14 criteria AND BB1–BB8 all PASS in `pil-live.eval.ts` AND §5 Path A kill-switch-off contract verified AND §7 Path C two-call hybrid + cap + audit chain verified AND §8 Path D legacy-placeholder-ignored verified AND §12 v0.5.11 substrate carry-forward still PASS.
>
> **Phase under the Core Dimension Check discipline:** advances Dim 4 (Reasoning) + Dim 8 (Feedback Loop closes) + Dim 10 (Observability/Evals) + Dim 12 (User Trust — observable + reversible behavior change); preserves Dim 9 HMR + 3E.1 + v0.5.11 substrate + v0.5.10 reply parser; intentionally defers Dim 1 Autonomy, Dim 5 Tool/Workflow, Dim 7 Multimodal, Dim 11 Production Scale.
>
> **v0.5.12 PASS does NOT auto-unlock** HMR feedback acknowledgment surface, production divergence audit on by default, per-user PIL opt-in UI, "Why?" answer surface, Slack-button kill switch, new memory_signal kinds, new source_surface activation, Friend C, autonomy tiers, auto-send, backfill of pre-migration alerts.sender_email_hash NULL rows, migration of legacy `message:<id>` placeholder rows, removal of two-call hybrid, 3E.1 reversal. The next phase runs its own 6-question gate.

---

**Founder:** Galiette Mita
**Run date:** _<YYYY-MM-DD HH:MM TZ>_
**Branch:** `phase-v0.5.12-live-ranker-pil-guarded`
**Scaffolding commit SHA:** _<sha>_
**Runtime commit SHA(s):** _<sha(s)>_
**SMOKE_START_TS:** _<UTC ISO timestamp>_
**Kill-switch timeline:**
  - OFF window: _<ts_start>_ → _<ts_flip>_  (Path A)
  - ON  window: _<ts_flip>_  → _<ts_end>_   (Paths B/C/D/E)
**Smoke window override:** `FOMO_V0_5_12_WINDOW_HOURS=24` (default)
**Tunable env vars at smoke time:** `FOMO_PIL_SCORE_CAP=_<float>_`, `FOMO_PIL_LIVE_ENABLED=true/false (per window)`, `FOMO_PIL_DIVERGENCE_AUDIT_ENABLED=_<bool>_`

---

## 1. Prerequisites confirmed

- [ ] PR #51 (v0.5.11 PIL substrate + shadow ranker) on `main` with VERDICT: PASS (merge `762439c3`)
- [ ] No friend involvement (three-friend cap)
- [ ] §1 baseline snapshot captured BEFORE smoke start (`/tmp/v0.5.12-baseline.txt`)
- [ ] `BREVIO_SENDER_HASH_KEY` set (≥32 bytes; carry-forward; LOAD-BEARING for read-time HMAC)
- [ ] `OPENAI_API_KEY` set (TWO ranker calls per PIL-influenced rank + offline eval)
- [ ] ≥1 canonical-HMAC PIL row present in `memory_signals` from v0.5.11 carry-forward (or synthetic seed)
- [ ] Migration 0008 (alerts.sender_email_hash) already applied (v0.5.11 carry-forward)
- [ ] ngrok up + reachable (or signed-curl substitute documented in §16)
- [ ] `FOMO_PIL_LIVE_ENABLED=false` at start (Path A)

## 2. Env additions (redacted)

| Var | Set? | Value | Notes |
|---|---|---|---|
| `FOMO_PIL_LIVE_ENABLED` | ☐ | `_<true/false per window>_` | Q5.A — default false (kill-switch off → bit-identical v0.5.11) |
| `FOMO_PIL_SCORE_CAP` | ☐ | `_<float>_` | Q2.A — default 0.15; bounds [0.05, 0.25] |
| `FOMO_PIL_DIVERGENCE_AUDIT_ENABLED` | ☐ | `false` (default) | Q4.A — OFF this phase |
| `FOMO_V0_5_12_BASELINE_CONFIRMED` | ☐ | `true` | set after §1 capture |
| `FOMO_V0_5_12_WINDOW_HOURS` | ☐ | default 24 | |

All v0.5.11 substrate env (FOMO_PIL_K_THRESHOLD / FOMO_PIL_SCORE_DELTA / FOMO_PIL_RECENCY_*) unchanged.

## 3. PASS criteria (14)

| # | Criterion | Evidence | Got |
|---|---|---|---|
| C1 | Kill switch off → bit-identical v0.5.11 behavior (no PIL prompt block, no `brevio.rank.pil_applied` audit, single ranker call) | _<Path A rank_id + 0-audit + prompt_version=ranker-v0.2.0>_ | ☐ |
| C2 | Kill switch on + matching canonical PIL row → PIL context included + audit fires with all 9 locked fields | _<Path C sample JSON>_ | ☐ |
| C3 | **Legacy `scope_key='message:<id>'` row produces null PIL context (LOAD-BEARING)** — no influence, no audit | _<Path D rank_id + 0-audit + BB6 eval result>_ | ☐ |
| C4 | Cap is enforced against a baseline no-PIL call (two-call hybrid; cap is REAL only if baseline runs) | _<Path C audit + math check: score_after_cap = baseline + clamp(raw_delta)>_ | ☐ |
| C5 | `brevio.rank.pil_applied` fires only when PIL context is non-null (no audit on Path B null-context rank) | _<Path B 0-audit verification>_ | ☐ |
| C6 | Privacy canary — no raw sender_email/subject/body/snippet/header in audit detail OR ranker prompt | _<canary scan + prompt assembly unit-test fixture>_ | ☐ |
| C7 | **Cross-user contamination — DB layer (LOAD-BEARING)** — user A signal cannot leak to user B's rank | _<Path E inverse query + BB4 eval result>_ | ☐ |
| C8 | **Suppressed sender with high-intrinsic importance can still surface (BB1 LOAD-BEARING)** — model overrides prior; rank.reason mentions both | _<BB1 eval output line>_ | ☐ |
| C9 | Old/stale PIL signals decay to zero (BB3) — Q3.C | _<BB3 eval output line + decay factor>_ | ☐ |
| C10 | One false-positive correction does NOT materially change live ranker output (BB5) — Q3.A guardrail 4 | _<BB5 eval output line: \|Δ\| ≤ 0.05>_ | ☐ |
| C11 | **PIL cannot bypass score cap via prompt/model behavior (BB8 LOAD-BEARING)** | _<BB8 eval: pil_score_delta_was_capped=true; \|delta\| ≤ FOMO_PIL_SCORE_CAP>_ | ☐ |
| C12 | v0.5.11 substrate remains unchanged — `applyPilAggregation` writes still happen, `brevio.signal.aggregated` still fires; PIL kinds + audit registry intact | _<smoke-evidence:v0.5.11 verdict + registry check>_ | ☐ |
| C13 | v0.5.7 HMR + v0.5.10 reply parser remain unchanged — `human-message-v0.3.0`, `reply-parser-v0.2.0` | _<grep + carry-forward smoke>_ | ☐ |
| C14 | 3E.1 invariant remains preserved — no LLM in body composition; only `rank.reason` is model-generated | _<grep on renderFounderText + human-message-renderer>_ | ☐ |

## 4. `smoke-evidence:v0.5.1` (substrate) — **VERDICT: _<…>_**

```
_<paste full output>_
```

## 5–11. Prior-phase smoke-evidence carry-forward — **VERDICTs**

| Script | Verdict | Notes |
|---|---|---|
| `smoke-evidence:v0.5.2` | _<…>_ | (`FOMO_V0_5_2_WINDOW_HOURS=168`) |
| `smoke-evidence:v0.5.3` | _<…>_ | |
| `smoke-evidence:v0.5.4` | _<…>_ | (`FOMO_V0_5_4_WINDOW_HOURS=168`) |
| `smoke-evidence:v0.5.5` | _<…>_ | |
| `smoke-evidence:v0.5.6` | _<…>_ | |
| `smoke-evidence:v0.5.7` | _<…>_ | HMR regression check |
| `smoke-evidence:v0.5.8` | _<…>_ | |
| `smoke-evidence:v0.5.9` | _<…>_ | Feedback substrate regression |
| `smoke-evidence:v0.5.10` | _<…>_ | Reply parser regression |
| `smoke-evidence:v0.5.11` | _<…>_ | **PIL substrate carry-forward — LOAD-BEARING for C12** |

> Operator note: v0.5.11 substrate UNCHANGED is the C12 contract. Any v0.5.12 regression here is a blocker.

## 12. `smoke-evidence:v0.5.12` output — **VERDICT: _<…>_**

```
_<paste full output>_
```

## 13. Path F — `eval:pil-live` output — **VERDICT: _<…>_**

```
_<paste full output of `pnpm --filter @brevio/fomo run eval:pil-live`>_
```

**8 LOAD-BEARING becomes-blind fixtures — each MUST be PASS:**

| ID | Setup | Required result | Got |
|---|---|---|---|
| BB1 | sender_suppressed=true 3d old + "URGENT: from CEO" email | label=important, score ≥ 0.7, rank.reason mentions prior + override | ☐ |
| BB2 | sender_importance.score=-0.3 (no suppression) + normal-strength email | Live score within ±`FOMO_PIL_SCORE_CAP` of baseline | ☐ |
| BB3 | sender_importance.score=+0.3 but 200d old + weak signals | Live ≈ baseline (\|Δ\| ≤ 0.05); decay → effective score 0 | ☐ |
| BB4 | Cross-user hash-collision sim (DB row at founder's actual scope under userA's id) | `buildLivePilContext('founder', H)` reads founder row only; userA row ignored | ☐ |
| BB5 | Single false_positive (1 event, score=-0.1) + mid-strength email | Live within noise floor (\|Δ\| ≤ 0.05) of baseline | ☐ |
| BB6 | Legacy `message:<id>` row only; NO canonical HMAC row | `pil_context: null`; bit-identical to baseline; NO audit | ☐ |
| BB7 | `FOMO_PIL_LIVE_ENABLED=false` + PIL rows present + strong-signal email | `pil_context: null`; baseline-only call; no audit | ☐ |
| BB8 | sender_importance.score=+1.0 (theoretical max) | `pil_score_delta_was_capped=true`; \|delta\| ≤ `FOMO_PIL_SCORE_CAP` | ☐ |

## 14. Sample audit JSON

**Sample `brevio.rank.pil_applied` audit detail (Path C — typical PIL-influenced rank):**

```json
_<paste verbatim — all 9 locked fields: rank_result_id, pil_signal_kinds_present, score_before_pil_cap, score_after_pil_cap, pil_score_delta, pil_score_delta_was_capped, model_mentioned_pil_in_reason, source_surface, scope_key_hash>_
```

**Sample `brevio.rank.pil_applied` audit detail (Path C — cap-enforced case, was_capped=true):**

```json
_<paste verbatim — score_before_pil_cap shows model's PIL-context score outside cap; score_after_pil_cap = baseline + ±FOMO_PIL_SCORE_CAP; pil_score_delta_was_capped=true>_
```

**Sample rank_results row showing v0.5.12 PIL-block prompt_version:**

```
rank_result_id=_<id>_  prompt_version=ranker-v0.3.0  label=_<…>_  score=_<…>_  reason_head=_<first 80 chars>_
```

**Kill-switch-off rank_results row (Path A, bit-identical v0.5.11):**

```
rank_result_id=_<id>_  prompt_version=ranker-v0.2.0  label=_<…>_  score=_<…>_  reason_head=_<first 80 chars>_
```

## 15. Operator-confirmed smoke evidence

| Check | Confirmed? | Notes |
|---|---|---|
| **Path A (LOAD-BEARING C1): kill switch off → bit-identical v0.5.11** | ☐ | _<rank_result_id, prompt_version=ranker-v0.2.0, 0 brevio.rank.pil_applied audits>_ |
| Path A: single ranker call per rank (NOT two-call hybrid) | ☐ | _<cost_records or runtime log evidence>_ |
| Path B: kill switch on, no canonical PIL row → no audit, pil_context null | ☐ | _<rank_id + 0-audit query>_ |
| **Path C (LOAD-BEARING C2 + C4 + C5): kill switch on, canonical PIL row → two-call hybrid + audit with all 9 fields** | ☐ | _<scope_key_hash, sample JSON, rank_results.score = audit.score_after_pil_cap>_ |
| Path C: score_after_pil_cap = baseline_score + clamp(pil_score - baseline_score, ±cap) | ☐ | _<math check from sample row>_ |
| Path C: model_mentioned_pil_in_reason bool computed via regex on rank.reason text (text itself NOT in audit) | ☐ | _<sample showing bool only, no raw reason>_ |
| **Path D (LOAD-BEARING C3): legacy message:<id> row only → null PIL context, no audit** | ☐ | _<rank_id + 0-audit query + BB6 eval>_ |
| **Path E (LOAD-BEARING C7): cross-user contamination test** | ☐ | _<userA row at scope X; founder rank for same email → 0 founder audit at userA's scope; BB4 eval>_ |
| **Path F BB1 (LOAD-BEARING C8): suppressed sender + URGENT email surfaces** | ☐ | _<BB1 eval line>_ |
| Path F BB2: not-yet-suppressed score within cap | ☐ | _<BB2 eval line>_ |
| **Path F BB3 (LOAD-BEARING C9): 200d-old signal decays to zero** | ☐ | _<BB3 eval line>_ |
| Path F BB4: cross-user read isolation | ☐ | _<BB4 eval line>_ |
| **Path F BB5 (LOAD-BEARING C10): one false_positive within noise floor** | ☐ | _<BB5 eval line>_ |
| **Path F BB6 (LOAD-BEARING C3 in-vitro): legacy placeholder → null** | ☐ | _<BB6 eval line>_ |
| **Path F BB7 (LOAD-BEARING C1 in-vitro): kill switch off → null** | ☐ | _<BB7 eval line>_ |
| **Path F BB8 (LOAD-BEARING C11): cap not bypassable** | ☐ | _<BB8 eval line: pil_score_delta_was_capped=true>_ |
| §11 Live ranker invariant: kill-switch gate present BEFORE buildLivePilContext call site | ☐ | _<grep>_ |
| §12 v0.5.9 + v0.5.10 + v0.5.11 evidence scripts still PASS or match documented benign shapes | ☐ | _<3 verdicts>_ |
| Code-level: `buildLivePilContext` exported from `pil-live-context.ts` or `pil-context.ts`; `pil-live.eval.ts` present with 11 carry-forward + 8 BB fixtures | ☐ | _<grep outputs>_ |
| Unit-test sanity: all new tests green (two-call hybrid, cap math, read-side filter, kill switch, audit detail 9 fields, cross-user) | ☐ | _<test count + pass count>_ |

## 16. Operator notes / honest substitutions

If any path was substituted, document below. **Do not oversell substitutes as full live proof** — same rule as v0.5.10 / v0.5.11.

If the live eval reports model-nondeterministic flakiness on shift magnitude (but DIRECTION is consistent), that's acceptable per Q4.A (eval asserts direction not magnitude). Document flaky fixtures by ID.

**Kill-switch timeline (LOAD-BEARING for C1 audit-row-count interpretation):**

```
OFF window:  _<ts_start>_ → _<ts_flip>_
ON  window:  _<ts_flip>_  → _<ts_end>_
Rank_result_ids observed in OFF window: [_<…>_]
Rank_result_ids observed in ON window:  [_<…>_]
brevio.rank.pil_applied audits in OFF window: _<expect 0>_
brevio.rank.pil_applied audits in ON window:  _<expect ≥ 1 if Path C ran>_
```

## 17. Founder observations

| Observation | Note |
|---|---|
| Does the two-call hybrid feel like the right shape, or would post-hoc-only (Q1.B) have been better as a first step? | _<…>_ |
| Is `FOMO_PIL_SCORE_CAP=0.15` the right value, or should it be tighter / looser based on observed shift magnitudes? | _<…>_ |
| Did the model mention PIL prior in `rank.reason` often enough to feel transparent (≥40% target on bounded-influence cases)? | _<…>_ |
| Did any BB fixture surface a real edge case worth promoting to a hardening item? | _<…>_ |
| Does v0.5.12 feel like the right shape for the v0.5.13+ next phase (HMR feedback acknowledgment surface, divergence audit production-on, per-user opt-in, "Why?" surface) to build on? | _<…>_ |
| Should `FOMO_PIL_LIVE_ENABLED` stay TRUE in production after smoke PASS, or stay false until further validation? | _<…>_ |

### Bonus findings (real-incident-backed, candidates for next-phase 6Q gate)

1. _<…>_
2. _<…>_

## 18. Verdict

☐ **PASS** — all 14 criteria green; BB1–BB8 all PASS in `eval:pil-live`; §5 Path A kill-switch-off bit-identical contract verified; §7 Path C two-call hybrid + cap + audit with all 9 fields verified; §8 Path D legacy-placeholder-ignored verified; §9 Path E cross-user contamination fails closed; §12 v0.5.9 + v0.5.10 + v0.5.11 evidence still PASS; privacy canary clean.

☐ **FAIL** — list below. Founder lock: if BB1 fails (suppressed sender + URGENT email does NOT surface), v0.5.12 does NOT ship — Brevio going binary-blind is a forbidden failure mode.

☐ **PENDING** — runtime commit(s) not yet on branch; re-run after runtime lands.

Failures / followups:

- _…_

## 19. Sign-off

- Founder signature: Galiette Mita
- Date: _<YYYY-MM-DD>_
- No friend consent needed this phase (founder-only smoke)

## 20. Aftercare confirmation

- [ ] Killed dev server + ngrok (if used)
- [ ] No friend deletion ops (no friend involved)
- [ ] v0.5.7 HMR template_version still `human-message-v0.3.0` (no renderer edits this phase)
- [ ] v0.5.9 substrate unchanged: BREVIO_FEEDBACK_SURFACES=13, ACTIVE=`['email_alert']`
- [ ] v0.5.10 reply-parser unchanged: `PROMPT_VERSION='reply-parser-v0.2.0'`, 8 intents
- [ ] v0.5.11 substrate write path unchanged: `applyPilAggregation` + `brevio.signal.aggregated` audit still fire on natural-reply feedback
- [ ] No LLM call introduced into renderer (3E.1 invariant)
- [ ] No raw private content in any new audit detail OR ranker PIL prompt block (C6 canary PASS)
- [ ] Read-side filter enforced: `scope_key ~ '^[a-f0-9]{32}$'` AND `user_id = userId` (legacy `message:<id>` rows ignored)
- [ ] Kill switch contract verified: off → bit-identical v0.5.11
- [ ] Production divergence audit (`FOMO_PIL_DIVERGENCE_AUDIT_ENABLED`) remains OFF by default
- [ ] No new active `source_surface` activated beyond `email_alert`
- [ ] No new memory_signal kinds beyond v0.5.11's two
- [ ] Final state: `FOMO_PIL_LIVE_ENABLED=_<true/false per founder decision>_` documented in §17

## 21. What v0.5.12 PASS does NOT promise

- **HMR Feedback Acknowledgment / Feedback Prompt Surface** — Q5.A defer from v0.5.10 still holds
- **Production divergence audit on by default** (`FOMO_PIL_DIVERGENCE_AUDIT_ENABLED=true`) — own future gate
- **Per-user PIL opt-in / opt-out UI**
- **"Why?" answer surface** explaining PIL influence to the user
- **Slack-button kill switch / Slack PIL-state surface**
- **Activating PIL for any sender_importance.score beyond the bounded [-1, +1] substrate range**
- **Backfill of pre-migration `alerts.sender_email_hash` NULL rows** — own future gate
- **Migration / cleanup of v0.5.10 legacy `message:<id>` placeholder rows** — own hardening phase
- **New memory_signal kinds beyond `sender_importance` + `sender_suppressed`** (e.g. `topic_importance`, `commercial_kept`, `conditional_rule`)
- **Activating any source_surface beyond `email_alert`** — each its own 6Q gate
- **F1 SendBlue tier fix / real SendBlue inbound proof**
- **Friend C onboarding** — three-friend cap
- **Autonomy tiers / auto-send / new tools / new modalities / production scale**
- **3E.1 reversal**
- **Removal of the two-call hybrid** (trusting prompt-only cap) — own future phase
- **Brevio proposing rules to the user** (silent learning + observable behavior change only this phase)
- **User-facing preference inspection / editing surface**
- **Storing reply text in any column / detail field**
- **STOP/START as preference feedback**

The next phase is decided AT THE NEXT 6-question gate.
