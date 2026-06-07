# Phase v0.5.13 Smoke Test Report — Founder-only PIL Live Canary / Controlled Activation

> Filled after running every step in `smoke-test-v0.5.13-pil-live-canary.md`. Commit as `docs/SMOKE_REPORT_v0.5.13.md` once **`VERDICT: PASS`** on all 12 criteria AND Phases A/C/E/F all observed end-to-end (Phase B preflight ERROR confirmed) AND v0.5.12 BB1–BB8 still PASS deterministically (3/3) AND v0.5.9 + v0.5.10 + v0.5.11 + v0.5.12 carry-forward smoke-evidence still PASS.
>
> **Phase under the Core Dimension Check discipline:** advances Dim 4 (Reasoning) + Dim 8 (Feedback Loop) + Dim 10 (Observability) + Dim 12 (User Trust); preserves Dim 9 HMR + Dim 6 Security + 3E.1 + v0.5.12 live PIL ranker contract + v0.5.11 substrate + v0.5.10 reply parser; intentionally defers Dim 1 Autonomy, Dim 5 Tool/Workflow, Dim 7 Multimodal, Dim 11 Production Scale.
>
> **v0.5.13 PASS does NOT auto-unlock** HMR Feedback Acknowledgment surface, "Why?" answer surface, friend activation (allowlist stays founder-only this phase), per-user PIL opt-in UI for end users, Slack-button kill-switch surface, production divergence audit on by default, per-user PIL score-cap overrides, new memory_signal kinds, new source_surface activation beyond `email_alert`, Friend C onboarding, autonomy tiers, auto-send, removal of the two-call hybrid, 3E.1 reversal. The next phase runs its own 6-question gate.

---

**Founder:** Galiette Mita
**Run date:** _<YYYY-MM-DD HH:MM TZ>_
**Branch:** `phase-v0.5.13-pil-live-canary`
**Scaffolding commit SHA:** _<sha>_
**Runtime commit SHA(s):** _<sha(s)>_
**SMOKE_START_TS:** _<UTC ISO timestamp>_
**Phase + flip timeline (LOAD-BEARING):**
  - Phase A (global=off, list=any): _<ts_start>_ → _<ts_phaseB_start>_
  - Phase B (global=on, list=empty): _<ts_phaseB_start>_ → _<ts_phaseC_start>_
  - Phase C (global=on, list=founder): _<CANARY_OPEN_TS>_ → _<SOFT_ROLLBACK_TS>_
  - Phase E (global=on, list=empty): _<SOFT_ROLLBACK_TS>_ → _<HARD_ROLLBACK_TS>_
  - Phase F (global=off, list=any): _<HARD_ROLLBACK_TS>_ → _<ts_end>_
**Smoke window override:** `FOMO_V0_5_13_WINDOW_HOURS=24` (default)
**Tunable env vars at smoke time:** `FOMO_PIL_LIVE_ENABLED` flipped per phase, `FOMO_PIL_LIVE_USER_ALLOWLIST` flipped per phase, `FOMO_PIL_SCORE_CAP=0.15`, `FOMO_PIL_DIVERGENCE_AUDIT_ENABLED=false`

---

## 1. Prerequisites confirmed

- [ ] PR #52 (v0.5.12 live PIL ranker) on `main` with VERDICT: PASS (merge `9d7a2e1b`)
- [ ] No friend involvement (three-friend cap)
- [ ] §1 baseline snapshot captured BEFORE smoke start (`/tmp/v0.5.13-baseline.txt`)
- [ ] `BREVIO_SENDER_HASH_KEY` set (carry-forward; LOAD-BEARING)
- [ ] `OPENAI_API_KEY` set
- [ ] `FOMO_FOUNDER_USER_ID` set to the actual founder user_id (LOAD-BEARING for allowlist match)
- [ ] ≥1 canonical-HMAC PIL row present in `memory_signals` for founder (v0.5.12 carry-forward)
- [ ] Migration 0008 (alerts.sender_email_hash) applied (v0.5.11 carry-forward)
- [ ] `FOMO_PIL_LIVE_ENABLED=false` + `FOMO_PIL_LIVE_USER_ALLOWLIST=` empty at start (Phase A)

## 2. Env additions

| Var | Set? | Value | Notes |
|---|---|---|---|
| `FOMO_PIL_LIVE_USER_ALLOWLIST` | ☐ | _<empty / `<founder_user_id>` per phase>_ | NEW this phase; CSV; trim-only no-lowercase per founder correction #1 |
| `FOMO_PIL_LIVE_ENABLED` | ☐ | `false → true → false` per phase | Carry-forward; flipped during canary |
| `FOMO_PIL_SCORE_CAP` | ☐ | `0.15` (carry-forward default) | Unchanged this phase |
| `FOMO_PIL_DIVERGENCE_AUDIT_ENABLED` | ☐ | `false` (default; hard boundary) | OUT OF SCOPE this phase |
| `FOMO_V0_5_13_BASELINE_CONFIRMED` | ☐ | `true` | set after §1 capture |
| `FOMO_V0_5_13_WINDOW_HOURS` | ☐ | default 24 | smoke-evidence window scope |
| `FOMO_V0_5_13_CANARY_OPEN_TS` | ☐ | _<ISO ts>_ | LOAD-BEARING for smoke-evidence C2/C3/C4/C5/C6/C9 scoping |
| `FOMO_V0_5_13_SOFT_ROLLBACK_TS` | ☐ | _<ISO ts>_ | LOAD-BEARING for smoke-evidence C7 scoping |
| `FOMO_V0_5_13_HARD_ROLLBACK_TS` | ☐ | _<ISO ts>_ | LOAD-BEARING for smoke-evidence C8 scoping |

All v0.5.12 / v0.5.11 substrate env unchanged.

## 3. PASS criteria (12)

| # | Criterion | Evidence | Got |
|---|---|---|---|
| C1 | 4-case truth table for (`FOMO_PIL_LIVE_ENABLED`, `FOMO_PIL_LIVE_USER_ALLOWLIST`) | _<unit-test verdicts + Phase A/B/C/E observation>_ | ☐ |
| C2 | ≥1 founder `brevio.rank.pil_applied` audit in canary window (Phase C) | _<rank_result_id + audit sample JSON>_ | ☐ |
| C3 | **LOAD-BEARING: 0 non-founder `brevio.rank.pil_applied` audits in canary window** | _<DB query result + Phase D observation or operator-confirmed substitution note>_ | ☐ |
| C4 | `pil_score_delta` distribution surfaced; cap-bind rate < 25% | _<smoke-evidence n / |Δ| stats / cap rate>_ | ☐ |
| C5 | Privacy canary — 0 hits on PIL audit detail in canary window | _<canary scan over 16+ substrings>_ | ☐ |
| C6 | **LOAD-BEARING: 0 founder PIL audits cite a scope_key without a founder memory_signal row** | _<inverse SQL result>_ | ☐ |
| C7 | **LOAD-BEARING: Soft rollback (clear allowlist mid-canary) → next fresh rank `ranker-v0.2.0` + no new PIL audit** | _<post-soft-rollback rank ids + audit count>_ | ☐ |
| C8 | **LOAD-BEARING: Hard rollback (`FOMO_PIL_LIVE_ENABLED=false` mid-canary) → next fresh rank `ranker-v0.2.0` + no new PIL audit** | _<post-hard-rollback rank ids + audit count>_ | ☐ |
| C9 | Legacy `message:<id>` rows still ignored at read side (regression) | _<DB query: 0 PIL audits with scope_key LIKE 'message:%'>_ | ☐ |
| C10 | v0.5.11 substrate UNCHANGED — `applyPilAggregation` + `brevio.signal.aggregated` audit still fire | _<smoke-evidence:v0.5.11 verdict>_ | ☐ |
| C11 | v0.5.9 + v0.5.10 + v0.5.12 carry-forward smoke-evidence PASS | _<3 verdicts>_ | ☐ |
| C12 | v0.5.12 BB1-BB8 still PASS deterministically (3/3 runs) | _<3 eval verdicts>_ | ☐ |

## 4. `smoke-evidence:v0.5.9` — **VERDICT: _<…>_**

```
_<paste tail>_
```

## 5–7. Prior-phase smoke-evidence carry-forward — **VERDICTs**

| Script | Verdict | Notes |
|---|---|---|
| `smoke-evidence:v0.5.9` | _<…>_ | Feedback substrate regression |
| `smoke-evidence:v0.5.10` | _<…>_ | Reply parser regression |
| `smoke-evidence:v0.5.11` | _<…>_ | **PIL substrate carry-forward — LOAD-BEARING for C10** |
| `smoke-evidence:v0.5.12` | _<…>_ | **Live PIL ranker carry-forward** |

> Operator note: v0.5.11 substrate UNCHANGED is the C10 contract. v0.5.12 substrate UNCHANGED carries forward through C11. Any regression here is a blocker.

## 8. `smoke-evidence:v0.5.13` output — **VERDICT: _<…>_**

```
_<paste full output>_
```

## 9. `eval:pil-live` × 3 (C12 LOAD-BEARING regression) — **VERDICTs**

| Run | Verdict | Notes |
|---|---|---|
| 1 | _<PASS / FAIL>_ | _<19 fixtures: shadow 11/11 + BB 8/8>_ |
| 2 | _<PASS / FAIL>_ | _<...>_ |
| 3 | _<PASS / FAIL>_ | _<...>_ |

**8 LOAD-BEARING becomes-blind fixtures (BB1–BB8) — each MUST be PASS on all 3 runs:**

| ID | Required result | Got |
|---|---|---|
| BB1 | suppressed sender + URGENT/CEO email → model overrides; score ≥ 0.7 | ☐ |
| BB2 | sender_importance.score=-0.3 (no suppression) → live within ±cap of baseline | ☐ |
| BB3 | 200d-old signal → decay → effective score 0 | ☐ |
| BB4 | cross-user read isolation deterministic | ☐ |
| BB5 | 1 false_positive below k_threshold → null PIL context (architectural; v0.5.12 fix) | ☐ |
| BB6 | legacy `message:<id>` row → null PIL context | ☐ |
| BB7 | kill switch off → null context, single ranker call, no audit | ☐ |
| BB8 | sender_importance.score=+1.0 → `pil_score_delta_was_capped=true`, |Δ| ≤ cap | ☐ |

## 10. Sample audit JSON

**Sample `brevio.rank.pil_applied` audit detail (Phase C — typical PIL-influenced rank):**

```json
_<paste verbatim — all 9 locked fields>_
```

All 9 locked structural fields present (carry-forward from v0.5.12 contract): `rank_result_id`, `scope_key_hash`, `source_surface`, `pil_score_delta`, `score_after_pil_cap`, `score_before_pil_cap`, `pil_signal_kinds_present`, `pil_score_delta_was_capped`, `model_mentioned_pil_in_reason`.

**Sample `rank_results` row showing v0.5.13 PIL-block prompt_version (Phase C):**

```
rank_result_id=_<id>_  prompt_version=ranker-v0.3.0  user_id=_<founder>_  label=_<…>_  score=_<…>_
```

**Sample `rank_results` row Phase A (kill switch off, bit-identical v0.5.11):**

```
rank_result_id=_<id>_  prompt_version=ranker-v0.2.0  user_id=_<founder>_  label=_<…>_  score=_<…>_
```

**Sample `rank_results` row Phase E (soft rollback, allowlist cleared):**

```
rank_result_id=_<id>_  prompt_version=ranker-v0.2.0  user_id=_<founder>_  label=_<…>_  score=_<…>_
```

**Sample `rank_results` row Phase F (hard rollback, global off):**

```
rank_result_id=_<id>_  prompt_version=ranker-v0.2.0  user_id=_<founder>_  label=_<…>_  score=_<…>_
```

## 11. Operator-confirmed smoke evidence

| Check | Confirmed? | Notes |
|---|---|---|
| **Phase A (C1 case (a)): kill switch off, allowlist any → bit-identical v0.5.11** | ☐ | _<rank_result_id, prompt_version=ranker-v0.2.0, 0 PIL audits>_ |
| **Phase B (C1 case (b)): preflight ERRORS on global=on + empty allowlist** | ☐ | _<exact ERROR message from preflight stdout>_ |
| Phase B (optional): runtime fail-closed when preflight bypassed (`fomo.pil_live.allowlist_empty` WARN log) | ☐ | _<log line + 0 PIL audits in window>_ |
| **Phase C (C1 case (c) + C2 LOAD-BEARING): kill switch on + allowlist=founder → ≥1 PIL audit fires for founder, all 9 fields** | ☐ | _<rank_result_id, scope_key_hash, sample JSON>_ |
| Phase C: prompt_version=`ranker-v0.3.0`; rank_results.score = audit.score_after_pil_cap | ☐ | _<math check>_ |
| Phase C: `model_mentioned_pil_in_reason` bool computed via regex on rank.reason (text NOT in audit) | ☐ | _<sample showing bool only>_ |
| **Phase D (C3 LOAD-BEARING): 0 non-founder PIL audits in canary window** | ☐ | _<DB query result>_ |
| **Phase D (C6 LOAD-BEARING): cross-user contamination — 0 founder audits cite non-founder scope** | ☐ | _<inverse SQL result>_ |
| **Phase E (C7 LOAD-BEARING): soft rollback in vivo → ranker-v0.2.0 + 0 new PIL audit** | ☐ | _<post-soft-rollback rank ids + audit count>_ |
| **Phase F (C8 LOAD-BEARING): hard rollback in vivo → ranker-v0.2.0 + 0 new PIL audit** | ☐ | _<post-hard-rollback rank ids + audit count>_ |
| C4: pil_score_delta distribution + cap-bind rate < 25% | ☐ | _<n / abs-mean / p90 / cap-rate>_ |
| C5: privacy canary on PIL audit detail | ☐ | _<rows scanned / canary hits expected 0>_ |
| C9: legacy `message:<id>` rows still ignored at read side | ☐ | _<0 PIL audits cite `message:%` scope_key>_ |
| §11 worker-level allowlist gate present BEFORE `buildLivePilContext` call site | ☐ | _<grep / unit-test confirmation>_ |
| §12 v0.5.9 + v0.5.10 + v0.5.11 + v0.5.12 carry-forward all PASS | ☐ | _<4 verdicts>_ |
| Code-level: `KillSwitches.pil_live_user_allowlist` parsed from `FOMO_PIL_LIVE_USER_ALLOWLIST` (trim-only, no-lowercase) | ☐ | _<grep / unit-test confirmation>_ |
| Unit-test sanity: 4-case truth table covered (founder-in-list / not-in-list / empty list global on / global off) | ☐ | _<test count + pass count>_ |

## 12. Operator notes / honest substitutions

If any path was substituted, document below. **Do not oversell substitutes as full live proof** — same rule as v0.5.10 / v0.5.11 / v0.5.12.

If Phase D's cross-user check finds 0 non-founder users polled during the canary window (e.g. friend-beta accounts have `stop_active=true`), document that the C3 + C6 LOAD-BEARING checks are confirmed via the architectural mechanism (worker-level allowlist gate + HMAC user_id construction) plus unit tests rather than in-vivo observation.

If natural Gmail is dry during Phase C, the runbook §7.2 sanctioned synthetic-seed substitute (extend `_smoke-v0.5.12-seed-path-c.ts`) is acceptable. Disclose.

### Phase + flip timeline (LOAD-BEARING for C7/C8 interpretation)

```
Phase A (global=off, list=any):     <ts_start>  → <ts_phaseB_start>
Phase B (global=on, list=empty):    <ts_phaseB_start>  → <ts_phaseC_start>
Phase C (global=on, list=founder):  <CANARY_OPEN_TS>  → <SOFT_ROLLBACK_TS>
Phase E (global=on, list=empty):    <SOFT_ROLLBACK_TS>  → <HARD_ROLLBACK_TS>
Phase F (global=off, list=any):     <HARD_ROLLBACK_TS>  → <ts_end>

Rank_result_ids observed per phase:
  Phase A: [...]
  Phase C: [...]
  Phase E (post-soft):  [...]
  Phase F (post-hard):  [...]

brevio.rank.pil_applied audits by phase:
  Phase A: 0  ✓ (expected — global off)
  Phase B: 0  ✓ (expected — preflight ERRORS; runtime fail-closed if bypassed)
  Phase C: _<expect ≥ 1>_
  Phase E (post-soft): 0  (LOAD-BEARING C7)
  Phase F (post-hard): 0  (LOAD-BEARING C8)
```

Per `[[real-or-absent-no-half-wired]]`: if any LOAD-BEARING criterion (C2, C3, C6, C7, C8) fails, v0.5.13 must NOT ship.

Per `[[no-gate-creep-on-extra-smokes]]`: ONE canary cycle is sufficient PASS evidence when regression tests + smoke evidence + carry-forward + no-leak scan all check out.

## 13. Founder observations

| Observation | Note |
|---|---|
| Did the canary feel safe end-to-end? Reversibility, allowlist, rollback paths convincing? | _<…>_ |
| `pil_score_delta` magnitudes — were they small enough that founder didn't feel a "different ranker," or noticeable? | _<distribution + perception>_ |
| Did the model mention PIL prior in `rank.reason` often enough to feel transparent? (Subjective; not a gate criterion) | _<sample reasons + freq>_ |
| Did soft rollback feel fast enough? Did hard rollback feel fast enough? | _<seconds-to-rollback + perception>_ |
| Did any canary observation surface a real edge case worth promoting to a hardening item? | _<…>_ |
| **Founder decision (correction #3): final production state for `FOMO_PIL_LIVE_ENABLED`?** | _<OFF (default) or ON-with-explicit-approval>_ |
| **Founder decision: final production state for `FOMO_PIL_LIVE_USER_ALLOWLIST`?** | _<unset (default) or `<founder_user_id>` (kept ON)>_ |
| Does v0.5.13 feel like the right shape for v0.5.14+ next phase to build on? | _<…>_ |

### Bonus findings (real-incident-backed, candidates for next-phase 6Q gate)

1. _<…>_
2. _<…>_

## 14. Verdict

☐ **PASS** — all 12 criteria green; LOAD-BEARING C2/C3/C6/C7/C8 verified in vivo (or operator-confirmed per §12 substitution); v0.5.12 BB1-BB8 still PASS deterministically (3/3); v0.5.9 + v0.5.10 + v0.5.11 + v0.5.12 evidence still PASS; privacy canary clean.

☐ **FAIL** — list below. Founder lock: if ANY LOAD-BEARING criterion (C2, C3, C6, C7, C8) fails, v0.5.13 does NOT ship — Brevio cannot ship a controlled-activation phase that can't be controlled or reversed.

☐ **PENDING** — runtime commit(s) not yet on branch; re-run after runtime lands.

Failures / followups:

- _…_

## 15. Sign-off

- Founder signature: Galiette Mita
- Date: _<YYYY-MM-DD>_
- No friend consent needed this phase (founder-only canary)

## 16. Aftercare confirmation

- [ ] Killed dev server + ngrok (if used)
- [ ] No friend deletion ops (no friend involved)
- [ ] v0.5.7 HMR template_version still `human-message-v0.3.0` (no renderer edits this phase)
- [ ] v0.5.9 substrate unchanged: BREVIO_FEEDBACK_SURFACES=13, ACTIVE=`['email_alert']`
- [ ] v0.5.10 reply-parser unchanged: `PROMPT_VERSION='reply-parser-v0.2.0'`, 8 intents
- [ ] v0.5.11 substrate write path unchanged: `applyPilAggregation` + `brevio.signal.aggregated` audit still fire on natural-reply feedback
- [ ] v0.5.12 live PIL ranker path unchanged: `buildLivePilContext` + `rankEmailWithLivePil` + `brevio.rank.pil_applied` audit unchanged; v0.5.13 only adds the worker-level allowlist gate in front
- [ ] No LLM call introduced into renderer (3E.1 invariant)
- [ ] No raw private content in any new audit detail (C5 canary PASS)
- [ ] Read-side filter enforced: scope_key ~ `^[a-f0-9]{32}$` AND user_id = userId AND v0.5.12 n_events floor still applied
- [ ] Kill switch contract verified: off → bit-identical v0.5.11
- [ ] **Allowlist contract verified**: global=true + allowlist=founder → only founder hybrid; global=true + empty list → all baseline (fail-closed); global=false + any list → all baseline (carry-forward)
- [ ] Production divergence audit (`FOMO_PIL_DIVERGENCE_AUDIT_ENABLED`) remains OFF by default
- [ ] No new active `source_surface` activated beyond `email_alert`
- [ ] No new memory_signal kinds beyond v0.5.11's two
- [ ] **Final state: `FOMO_PIL_LIVE_ENABLED=_<true/false per founder §17 decision>_`** + `FOMO_PIL_LIVE_USER_ALLOWLIST=_<unset / `<founder_user_id>` per §17>_`

## 17. What v0.5.13 PASS does NOT promise

- **HMR Feedback Acknowledgment / Feedback Prompt Surface** — own future 6Q gate
- **"Why?" answer surface** explaining PIL influence to the user — own future 6Q gate
- **Friend activation** (allowlist stays founder-only this phase; broader activation own future 6Q gate)
- **Per-user PIL opt-in / opt-out UI for end users**
- **Slack-button kill-switch surface**
- **Production divergence audit on by default** (`FOMO_PIL_DIVERGENCE_AUDIT_ENABLED=true`) — own future gate
- **Per-user PIL score-cap overrides**
- **New memory_signal kinds beyond `sender_importance` + `sender_suppressed`**
- **Activating any source_surface beyond `email_alert`** — each its own 6Q gate
- **Friend C onboarding** — three-friend cap
- **Autonomy tiers / auto-send / new tools / new modalities / production scale**
- **3E.1 reversal**
- **Removal of the two-call hybrid**
- **Brevio proposing rules to the user** (silent observation + observable behavior change for founder only this phase)
- **User-facing preference inspection / editing surface**
- **Storing reply text in any column / detail field**
- **STOP/START as preference feedback**
- **Backfill of pre-migration `alerts.sender_email_hash` NULL rows**
- **Migration / cleanup of v0.5.10 legacy `message:<id>` placeholder rows**

The next phase is decided AT THE NEXT 6-question gate.
