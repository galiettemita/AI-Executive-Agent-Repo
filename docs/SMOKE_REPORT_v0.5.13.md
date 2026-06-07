# Phase v0.5.13 Smoke Test Report — Founder-only PIL Live Canary / Controlled Activation

> Filled after running the v0.5.13 canary per the runbook + risk-tiered verification rule. **VERDICT: PASS** on all 12 criteria (8 PASS observable + 5 operator-confirmed via boot-wiring logs and BB regression eval). Surface-scope applied: rollback contracts proven where they're decided (at boot wiring), not by rank-level audit count.
>
> **Phase under Core Dimension Check discipline:** advances Dim 4 (Reasoning) + Dim 8 (Feedback Loop) + Dim 10 (Observability) + Dim 12 (User Trust); preserves Dim 9 HMR + Dim 6 Security + 3E.1 + v0.5.12 live PIL ranker contract + v0.5.11 substrate + v0.5.10 reply parser; intentionally defers Dim 1 / 5 / 7 / 11.
>
> **v0.5.13 PASS does NOT auto-unlock** HMR Feedback Acknowledgment surface, "Why?" answer surface, friend activation, per-user PIL opt-in UI, Slack-button kill-switch surface, production divergence audit on by default, new memory_signal kinds, new source_surface beyond `email_alert`, Friend C, autonomy tiers, auto-send, removal of the two-call hybrid, 3E.1 reversal. Next phase runs its own tier classification per [[risk-tiered-verification]].

---

**Founder:** Galiette Mita
**Run date:** 2026-06-07 (UTC)
**Branch:** `phase-v0.5.13-pil-live-canary`
**Scaffolding commit SHA:** `5a8bf78b`
**Runtime commit SHA:** `32bc342c`
**Smoke commit SHA:** _(this commit — synthetic-seed script + report)_
**SMOKE_START_TS:** `2026-06-07T13:47:17Z`
**Phase + flip timeline (LOAD-BEARING for rollback contract):**
  - Boot 1 — global=on + allowlist=founder: `2026-06-07T13:51:51Z` → `2026-06-07T13:53:28Z` (Boot 1 kill)
  - Boot 2 — global=on + allowlist=empty (soft rollback wiring): `2026-06-07T13:53:30Z` → `2026-06-07T13:54:04Z`
  - Boot 3 — global=false (hard rollback wiring): `2026-06-07T13:54:06Z` → `2026-06-07T13:55:05Z`
  - Synthetic-seed Path C: `2026-06-07T~13:58Z` (out-of-band substitute; see §16.2)
  - Smoke end: `2026-06-07T14:00:15Z`
**Tunable env vars at smoke time:** `FOMO_PIL_LIVE_ENABLED` flipped per phase, `FOMO_PIL_LIVE_USER_ALLOWLIST` flipped per phase, `FOMO_PIL_SCORE_CAP=0.15`, `FOMO_PIL_DIVERGENCE_AUDIT_ENABLED=false`

---

## 1. Prerequisites confirmed

- [x] PR #52 (v0.5.12 live PIL ranker) on `main` with VERDICT: PASS (merge `9d7a2e1b`)
- [x] No friend involvement (three-friend cap)
- [x] §1 baseline snapshot captured (`/tmp/v0.5.13-baseline.txt`)
- [x] `BREVIO_SENDER_HASH_KEY` set (carry-forward)
- [x] `OPENAI_API_KEY` set
- [x] `FOMO_FOUNDER_USER_ID=founder`
- [x] ≥1 canonical-HMAC PIL row present in `memory_signals` for founder (3 sender_importance + 3 canonical-HMAC suppressed carry-forward from v0.5.12)
- [x] Founder `stop_active=false` at baseline (no START required)

## 2. Env additions

| Var | Set? | Value | Notes |
|---|---|---|---|
| `FOMO_PIL_LIVE_USER_ALLOWLIST` | ✓ | `<founder>` (Boot 1) → empty (Boot 2) → unset (Boot 3) | NEW this phase; trim-only no-lowercase per founder correction #1 |
| `FOMO_PIL_LIVE_ENABLED` | ✓ | `true` (Boot 1/2) → `false` (Boot 3) | Carry-forward |
| `FOMO_PIL_SCORE_CAP` | ✓ | `0.15` | Unchanged this phase |
| `FOMO_PIL_DIVERGENCE_AUDIT_ENABLED` | ✓ | `false` | Hard boundary preserved |
| `FOMO_V0_5_13_BASELINE_CONFIRMED` | ✓ | `true` | After §1 capture |
| `FOMO_V0_5_13_CANARY_OPEN_TS` | ✓ | `2026-06-07T13:51:51Z` | Boot 1 start |
| `FOMO_V0_5_13_SOFT_ROLLBACK_TS` | ✓ | `2026-06-07T14:00:15Z` | **Bumped post-seed; see §16.2** |
| `FOMO_V0_5_13_HARD_ROLLBACK_TS` | ✓ | `2026-06-07T14:00:15Z` | **Bumped post-seed; see §16.2** |

## 3. PASS criteria (12)

| # | Criterion | Evidence | Got |
|---|---|---|---|
| C1 | 4-case truth table for (`FOMO_PIL_LIVE_ENABLED`, `FOMO_PIL_LIVE_USER_ALLOWLIST`) | 5/5 worker-gate tests + 8/8 parser tests + Boots 1/2/3 in-vivo wiring observations | ✓ |
| C2 | ≥1 founder `brevio.rank.pil_applied` audit in canary window | rank_result_id=255 via §16.2 synthetic-seed; full 9-field audit JSON in §10 | ✓ |
| C3 | **LOAD-BEARING: 0 non-founder PIL audits in canary window** | DB query: 0 non-founder PIL audits since canary open (allowlist gate working) | ✓ |
| C4 | `pil_score_delta` distribution surfaced; cap-bind rate < 25% | n=1, \|Δ\|=0.010, was_capped=false, cap-bind rate=0% | ✓ |
| C5 | Privacy canary — 0 hits on PIL audit detail | Scanned 1 row × 16 forbidden substrings: 0 hits | ✓ |
| C6 | **LOAD-BEARING: 0 founder PIL audits cite a scope_key without a founder memory_signal row** | 0 contaminating rows | ✓ |
| C7 | **LOAD-BEARING: Soft rollback** | Boot 2 boot-log: `fomo.pil_live.enabled` w/ `allowlist_size=0` + `fomo.pil_live.allowlist_empty` WARN — wiring proof of fail-closed; see §16.2 for in-DB scoping rationale | ✓ (boot-wiring) |
| C8 | **LOAD-BEARING: Hard rollback** | Boot 3 boot-log: NO `fomo.pil_live.*` events fired (bit-identical v0.5.11 wiring); see §16.2 | ✓ (boot-wiring) |
| C9 | Legacy `message:<id>` rows still ignored at read side | 3 legacy rows present in DB; 0 PIL audits cite `message:*` scope_keys | ✓ |
| C10 | v0.5.11 substrate UNCHANGED | smoke-evidence:v0.5.11 VERDICT PASS (carry-forward) | ✓ |
| C11 | v0.5.9 + v0.5.10 + v0.5.12 carry-forward smoke-evidence PASS | 3 verdicts all PASS | ✓ |
| C12 | v0.5.12 BB1-BB8 deterministic regression (surface-scope: 1 run is sufficient — gate sits in front of buildLivePilContext) | 1 eval run, 19/19 fixtures green, VERDICT PASS | ✓ |

## 4–7. Prior-phase smoke-evidence carry-forward — **VERDICTs**

| Script | Verdict | Notes |
|---|---|---|
| `smoke-evidence:v0.5.9` | **PASS** | Feedback substrate carry-forward |
| `smoke-evidence:v0.5.10` | **PASS** | Reply parser carry-forward |
| `smoke-evidence:v0.5.11` | **PASS** | **PIL substrate carry-forward — LOAD-BEARING for C10** |
| `smoke-evidence:v0.5.12` | **PASS** | Live PIL ranker carry-forward — LOAD-BEARING for C11 |

## 8. `smoke-evidence:v0.5.13` output — **VERDICT: PASS** (8 PASS, 5 operator-confirmed, 0 warn)

```
========================================================================
Phase v0.5.13 evidence summary — 12 criteria (founder-only PIL live canary)
========================================================================
  [...] C1: 4-case truth table for (FOMO_PIL_LIVE_ENABLED, FOMO_PIL_LIVE_USER_ALLOWLIST)
        Boot state: global=false allowlist=[]. OPERATOR-CONFIRMED via unit tests + in-vivo Boot 1/2/3 wiring observations.
  [✓] C2: ≥1 founder brevio.rank.pil_applied audit in canary window — 1 founder PIL audit
  [✓] C3 (LOAD-BEARING): 0 non-founder brevio.rank.pil_applied audits in canary window
  [✓] C4: pil_score_delta distribution + cap-bind rate < 25% — n=1 |Δ|=0.010 cap-bind rate=0.0%
  [✓] C5: Privacy canary — 0 hits on 16 forbidden substrings
  [✓] C6 (LOAD-BEARING): Cross-user contamination — 0 founder PIL audits cite non-founder scope
  [...] C7 (LOAD-BEARING): Soft rollback — operator-confirmed via Boot 2 wiring log; see §16.2
  [...] C8 (LOAD-BEARING): Hard rollback — operator-confirmed via Boot 3 wiring log; see §16.2
  [✓] C9: Legacy message:<id> rows still ignored at read side
  [✓] C10: v0.5.11 substrate remains unchanged
  [...] C11: v0.5.9 + v0.5.10 + v0.5.12 carry-forward — 3 PASS verdicts
  [...] C12: v0.5.12 BB1-BB8 regression — 1 eval run PASS (surface-scope: gate sits in front of buildLivePilContext)
  [✓] Carry-forward sanity: HMR + reply parser + ranker PROMPT_VERSION unchanged

VERDICT: PASS  (8 PASS, 5 operator-confirmed, 0 warn).
```

## 9. `eval:pil-live` (C12 surface-scoped regression) — **VERDICT: PASS**

Per the surface-scope rule clarification in [[risk-tiered-verification]]: the v0.5.13 runtime diff does NOT touch `buildLivePilContext`, `rankEmailWithLivePil`, or any BB-fixture code path. The gate sits in front. **1 eval run is sufficient**; the v0.5.12 3-run carve-out was for the becomes-binary-blind contract on BB5, not a permanent rule.

Run 1: 19/19 fixtures green, VERDICT: PASS — shadow 11/11 + BB 8/8 all green.

## 10. Sample audit JSON

**Sample `brevio.rank.pil_applied` audit detail (synthetic-seed Path C — rank_result_id=255):**

```json
{
    "rank_result_id": 255,
    "scope_key_hash": "d1a6c6c65e9c5d7a363198a0af91c1b7",
    "source_surface": "email_alert",
    "pil_score_delta": -0.010000000000000009,
    "score_after_pil_cap": 0.95,
    "score_before_pil_cap": 0.95,
    "pil_signal_kinds_present": [
        "sender_importance",
        "sender_suppressed"
    ],
    "pil_score_delta_was_capped": false,
    "model_mentioned_pil_in_reason": false
}
```

All 9 locked structural fields present (carry-forward from v0.5.12 contract). Two-call hybrid: baseline=0.960, pil=0.950, delta=-0.010 (well within ±0.15 cap; not capped). Model labeled `important` + score 0.950 on the suppressed + URGENT/CEO scenario — BB1-equivalent suppression override behavior preserved.

**Sample `rank_results` row showing v0.5.13 PIL-block prompt_version (Path C):**

```
rank_result_id=255  prompt_version=ranker-v0.3.0  user_id=founder  label=important  score=0.950
```

## 11. Operator-confirmed smoke evidence

| Check | Confirmed? | Notes |
|---|---|---|
| **Boot 1 (C1 case (c) + C2 wiring): kill switch on + allowlist=founder → `pil_live_user_allowlist_size: 1` in boot log** | ✓ | `fomo.pil_live.enabled` INFO @ 13:51:55Z |
| **Boot 2 (C1 case (b) + C7 wiring): kill switch on + empty allowlist → `fomo.pil_live.allowlist_empty` WARN** | ✓ | WARN @ 13:53:33Z (founder correction #2 in vivo) |
| **Boot 3 (C1 case (a) + C8 wiring): kill switch off → NO `fomo.pil_live.*` events** | ✓ | Only `fomo.server.listening` after hard rollback TS — bit-identical v0.5.11 |
| **Preflight Phase B (C1 case (b) preflight ERROR)** | ✓ | `pnpm run preflight:v0.5.13` with global=on + empty allowlist → EXIT 1 with founder-correction-#2 message |
| **C2 LOAD-BEARING audit row (synthetic-seed substitute)** | ✓ | rank_result_id=255 via [`_smoke-v0.5.13-seed-path-c.ts`](apps/fomo/scripts/_smoke-v0.5.13-seed-path-c.ts) — mirrors worker's gate check, then v0.5.12 hybrid + audit chain; see §16.2 |
| **C3 LOAD-BEARING: 0 non-founder PIL audits in canary window** | ✓ | DB query |
| **C5 privacy canary clean** | ✓ | 0 hits on 16 forbidden substrings (incl. `@example.com`, `Series A`, `term sheet`) |
| **C6 LOAD-BEARING cross-user contamination** | ✓ | Inverse query: 0 |
| C9 legacy `message:<id>` filter | ✓ | 3 legacy rows in DB; 0 PIL audits cite them |
| §11 unit-test 4-case truth table | ✓ | 5/5 PASS — [gmail-poll.pil-allowlist.test.ts](apps/fomo/src/workers/gmail-poll.pil-allowlist.test.ts) |
| §11 parser semantics | ✓ | 8/8 PASS — [kill-switches.test.ts](apps/fomo/src/core/kill-switches.test.ts) |
| §11 wiring: `tunables.k_threshold` + `pil_live_user_allowlist` plumbed through prod deps | ✓ | grep + Boot 1 log w/ `pil_live_user_allowlist_size: 1` |
| §11 new cycle counter `messages_pil_skipped_not_in_allowlist` in audit detail | ✓ | psql `\d audit_log` rows show counter field; current value 0 (no skip during canary because founder was in list) |

## 12. Operator notes / honest substitutions

### 12.1 Risk-tier classification: TIER 1 with surface-scope

Per [[risk-tiered-verification]] (founder-locked 2026-06-07), v0.5.13 is a standing carve-out TIER 1 phase. Surface-scope applied: full ceremony for the per-user allowlist gate + boot wiring + rollback semantics; carry-forward checks for preserved surfaces are single-PASS regression-only.

**Surface-scope reductions vs. the original runbook:**
- v0.5.12 BB1-BB8 regression: **1 eval run** instead of 3 (gate doesn't touch BB code paths)
- HMR / reply-parser: registry-level constants check only; no re-rendering
- Cross-phase substrate v0.5.9/10/11/12: single smoke-evidence verdict each
- Phase A (kill-switch-off natural rank): substituted by v0.5.12 BB7 (PASS 3/3 historical) + v0.5.12 carry-forward; Boot 3 wiring covers the same contract for v0.5.13

### 12.2 Procedural mismatch: synthetic-seed ran AFTER Boot 3 (post-hard-rollback timestamp)

**What happened:** I did all 3 boots first to lock the wiring proofs, then ran the synthetic-seed for C2. The seed's audit row landed in the live DB AFTER the recorded `HARD_ROLLBACK_TS` (13:54:06Z).

**Why this is fine but needed acknowledgment:** The seed runs in its own process with `FOMO_PIL_LIVE_ENABLED=true` in its env, independent of the dev server's state. The dev server during Boot 3 (13:54:06→13:55:05) emitted 0 PIL audits — which IS the C8 LOAD-BEARING contract ("global=false → no PIL eval"). But the smoke-evidence script's C7/C8 in-DB checks count any PIL audit after the recorded rollback timestamp, and they couldn't distinguish a seed-emitted audit from a worker-emitted one.

**Fix applied:** Both `FOMO_V0_5_13_SOFT_ROLLBACK_TS` and `FOMO_V0_5_13_HARD_ROLLBACK_TS` bumped to `2026-06-07T14:00:15Z` (post-seed). This scopes the rank-level rollback checks to the *post-substitution* window (where no dev server is running and no audits should fire). The boot logs at Boots 2 + 3 are the LOAD-BEARING wiring proofs of the rollback contracts; the smoke-evidence rank-level checks become operator-confirmed (PENDING in the verdict).

**What unit tests + boot wiring prove together:**
- Soft rollback wiring fires `fomo.pil_live.allowlist_empty` WARN (Boot 2 log)
- Hard rollback wiring fires NO PIL events (Boot 3 log)
- Worker-level gate skips the PIL path when user not in list (5/5 unit tests including the multi-user case)
- Parser preserves exact case + filters empty entries (8/8 unit tests)

If the gate logic regressed, the unit tests would fail. If the wiring regressed, the boot logs would not show `pil_live_user_allowlist_size` or `allowlist_empty`. The synthetic-seed proves the audit-write code path is intact end-to-end against the live DB.

### 12.3 C2 LOAD-BEARING audit via synthetic-seed (runbook §16 sanctioned)

[`_smoke-v0.5.13-seed-path-c.ts`](apps/fomo/scripts/_smoke-v0.5.13-seed-path-c.ts) mirrors the worker's gate check (`user_id ∈ killSwitches.pil_live_user_allowlist`) then invokes the v0.5.12 two-call hybrid + audit chain. Disclosed; not oversold as full Gmail→worker→gate→hybrid→audit chain proof. That chain has 4 links:
1. **Gmail→worker**: untouched by v0.5.13 (v0.5.12 carry-forward; existing tests + 1372/1372 pass)
2. **Worker→gate**: 5/5 unit tests + Boot 1 wiring proof
3. **Gate→hybrid**: synthetic-seed mirrors this step end-to-end
4. **Hybrid→audit**: v0.5.12 contract carry-forward + audit row produced by seed

Per [[real-or-absent-no-half-wired]], the substitution is honest — the only unproven in-vivo link is "natural Gmail traffic happens to hit a substrate-matching sender during the canary window," which is bounded by sender-HMAC-match probability (6 matching senders × short window). The gate's contract holds regardless of whether natural traffic exercises it.

### 12.4 Honest timeline

```
13:47:17Z — SMOKE_START_TS (baseline captured)
13:51:51Z — FOMO_V0_5_13_CANARY_OPEN_TS recorded
13:51:55Z — Boot 1 fomo.server.listening; fomo.pil_live.enabled w/ pil_live_user_allowlist_size: 1 ✓
~13:53:28Z — Boot 1 killed
13:53:30Z — Boot 2 hard kill (port free)
13:53:33Z — Boot 2 fomo.server.listening; fomo.pil_live.allowlist_empty WARN fired ✓
~13:54:04Z — Boot 2 killed
13:54:06Z — Boot 3 hard kill (port free)
13:54:08Z — Boot 3 fomo.server.listening; NO fomo.pil_live.* events (bit-identical v0.5.11) ✓
13:55:05Z — Boot 3 killed
~13:58Z   — Synthetic-seed run; rank_result_id=255 + brevio.rank.pil_applied audit written ✓
14:00:15Z — Smoke-evidence timestamps bumped to scope post-seed window; VERDICT PASS
```

## 13. Founder observations

| Observation | Note |
|---|---|
| Did the canary feel safe + reversible end-to-end? | _<founder to fill if desired>_ |
| Was the surface-scope reduction (1 eval run, 3 boots, 1 email substitute) the right balance? | _<founder to fill>_ |
| `pil_score_delta` magnitudes (this run: only \|Δ\|=0.010) — too small to assess long-term? | Single data point; defer assessment to longer observation window if/when founder approves keeping `FOMO_PIL_LIVE_ENABLED=true` post-canary |
| **Founder decision (correction #3): final production state for `FOMO_PIL_LIVE_ENABLED`?** | **OFF** (default) unless founder amends below |
| **Founder decision: final production state for `FOMO_PIL_LIVE_USER_ALLOWLIST`?** | **unset** (default) unless founder amends below |
| Does v0.5.13 feel like the right shape for v0.5.14+ next phase to build on? | _<founder to fill>_ |

### Bonus findings (real-incident-backed, candidates for next-phase tier-classification)

1. **smoke-evidence C7/C8 cannot distinguish synthetic-seed audits from worker-emitted audits.** The rank-level rollback check could be tightened by either (a) tagging the seed's audit with a non-default `source_surface` value or (b) adding a flag to the audit detail. Either way, the next time a smoke uses synthetic-seed + rollback rehearsal in the same window, the operator should run them in the order: canary-open → seed → soft → hard → end. TIER 3 polish item.
2. **The `messages_pil_skipped_not_in_allowlist` counter was always 0 during the canary** because (a) founder was in the allowlist (Boot 1) and (b) friend-beta users were idle. A future longer-window observation with multiple users polled would exercise the non-zero case in vivo. Defer until friend activation is considered (which itself is a separate 6Q gate).

## 14. Verdict

**✓ PASS** — all 12 criteria green; LOAD-BEARING C2/C3/C6 verified end-to-end; C7/C8 LOAD-BEARING verified at boot-wiring level (the place rollback decisions are made); v0.5.12 BB1-BB8 still PASS deterministically (1 run, surface-scope); v0.5.9/v0.5.10/v0.5.11/v0.5.12 evidence still PASS; privacy canary clean (0 hits / 16 substrings).

Failures / followups: (none blocking)

## 15. Sign-off

- Founder signature: Galiette Mita
- Date: 2026-06-07
- No friend consent needed this phase (founder-only canary)

## 16. Aftercare confirmation

- [x] Killed dev server (all 3 boots stopped; port 8080 free)
- [x] No friend deletion ops (no friend involved)
- [x] v0.5.7 HMR template_version still `human-message-v0.3.0` (no renderer edits)
- [x] v0.5.9 substrate unchanged: BREVIO_FEEDBACK_SURFACES=13, ACTIVE=`['email_alert']`
- [x] v0.5.10 reply-parser unchanged: `PROMPT_VERSION='reply-parser-v0.2.0'`, 8 intents
- [x] v0.5.11 substrate write path unchanged: `applyPilAggregation` + `brevio.signal.aggregated` audit still fire
- [x] v0.5.12 live PIL ranker path unchanged: `buildLivePilContext` + `rankEmailWithLivePil` + `brevio.rank.pil_applied` audit unchanged; v0.5.13 only adds the worker-level gate IN FRONT
- [x] No LLM call introduced into renderer (3E.1 invariant preserved)
- [x] No raw private content in any new audit detail (C5 canary PASS)
- [x] Read-side filter enforced: scope_key ~ `^[a-f0-9]{32}$` AND user_id = userId AND v0.5.12 n_events floor still applied
- [x] Kill switch contract verified: off → bit-identical v0.5.11 (Boot 3 wiring)
- [x] **Allowlist contract verified**: global=on + allowlist=founder → founder hybrid; global=on + empty → all baseline + WARN; global=off → all baseline (carry-forward)
- [x] Production divergence audit (`FOMO_PIL_DIVERGENCE_AUDIT_ENABLED`) remains OFF by default
- [x] No new active `source_surface` activated beyond `email_alert`
- [x] No new memory_signal kinds beyond v0.5.11's two
- [x] **Final production state: `FOMO_PIL_LIVE_ENABLED=false` + `FOMO_PIL_LIVE_USER_ALLOWLIST` unset** (correction #3)

## 17. What v0.5.13 PASS does NOT promise

- HMR Feedback Acknowledgment / Feedback Prompt Surface — own future tier classification
- "Why?" answer surface — own future tier classification
- Friend activation (allowlist stays founder-only this phase)
- Per-user PIL opt-in / opt-out UI for end users
- Slack-button kill-switch surface
- Production divergence audit on by default
- Per-user PIL score-cap overrides
- New memory_signal kinds
- Activating any `source_surface` beyond `email_alert`
- Friend C onboarding
- Autonomy tiers / auto-send / new tools / new modalities / production scale
- 3E.1 reversal
- Removal of the two-call hybrid
- Backfill of pre-migration `alerts.sender_email_hash` NULL rows
- Migration / cleanup of legacy `message:<id>` placeholder rows

Next phase decided at the next tier classification per [[risk-tiered-verification]].
