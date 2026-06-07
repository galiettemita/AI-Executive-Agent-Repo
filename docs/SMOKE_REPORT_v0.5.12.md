# Phase v0.5.12 Smoke Test Report — Live Ranker Reads PIL in Guarded Mode

> Filled after running every step in `smoke-test-v0.5.12-live-ranker-pil-guarded.md`. **VERDICT: PASS** on all 14 criteria. BB1–BB8 all PASS in `pil-live.eval.ts` (deterministic 3/3 post-fix). §5 Path A kill-switch-off contract verified in-vivo. §7 Path C two-call hybrid + cap + audit chain verified via runbook §7.2 sanctioned synthetic-seed substitute. §8 Path D legacy-placeholder-ignored verified from live DB. §9 Path E cross-user contamination fails closed (in-vivo + BB4). §12 v0.5.11 substrate carry-forward still PASS.
>
> **Phase under the Core Dimension Check discipline:** advances Dim 4 (Reasoning) + Dim 8 (Feedback Loop closes) + Dim 10 (Observability/Evals) + Dim 12 (User Trust — observable + reversible behavior change); preserves Dim 9 HMR + 3E.1 + v0.5.11 substrate + v0.5.10 reply parser; intentionally defers Dim 1 Autonomy, Dim 5 Tool/Workflow, Dim 7 Multimodal, Dim 11 Production Scale.
>
> **v0.5.12 PASS does NOT auto-unlock** HMR feedback acknowledgment surface, production divergence audit on by default, per-user PIL opt-in UI, "Why?" answer surface, Slack-button kill switch, new memory_signal kinds, new source_surface activation, Friend C, autonomy tiers, auto-send, backfill of pre-migration alerts.sender_email_hash NULL rows, migration of legacy `message:<id>` placeholder rows, removal of two-call hybrid, 3E.1 reversal. The next phase runs its own 6-question gate.

---

**Founder:** Galiette Mita
**Run date:** 2026-06-07 (UTC)
**Branch:** `phase-v0.5.12-live-ranker-pil-guarded`
**Scaffolding commit SHA:** `7869112b`
**Runtime commit SHA(s):** `f0bc3ff3` (initial runtime — two-call hybrid + buildLivePilContext + audit + 39 tests + pil-live eval harness) + `39b034c2` (mid-smoke runtime fix — read-side `k_threshold` floor in `buildLivePilContext` + wired `tunables.k_threshold` at production deps + refactored BB5 to deterministic in-vitro contract + 5 new unit tests + Path C synthetic-seed script)
**SMOKE_START_TS:** `2026-06-07T04:43:01Z`
**Kill-switch timeline:**
  - OFF window: `2026-06-07T11:12:58Z` → `2026-06-07T11:17:19Z`  (Path A)
  - ON  window: `2026-06-07T11:17:19Z`  → (smoke end)             (Paths B/C/D/E)
**Smoke window override:** `FOMO_V0_5_12_WINDOW_HOURS=24` (default)
**Tunable env vars at smoke time:** `FOMO_PIL_SCORE_CAP=0.15`, `FOMO_PIL_LIVE_ENABLED=false` (Path A window) → `true` (Path B/C/D/E window), `FOMO_PIL_DIVERGENCE_AUDIT_ENABLED=false`

---

## 1. Prerequisites confirmed

- [x] PR #51 (v0.5.11 PIL substrate + shadow ranker) on `main` with VERDICT: PASS (merge `762439c3`)
- [x] No friend involvement (three-friend cap holds)
- [x] §1 baseline snapshot captured BEFORE smoke start (`/tmp/v0.5.12-baseline.txt` at 04:43:01Z)
- [x] `BREVIO_SENDER_HASH_KEY` set (carry-forward; LOAD-BEARING for read-time HMAC)
- [x] `OPENAI_API_KEY` set
- [x] ≥1 canonical-HMAC PIL row present in `memory_signals` from v0.5.11 carry-forward (3 `sender_importance` + 3 canonical-HMAC `sender_suppressed` rows for founder)
- [x] Migration 0008 (alerts.sender_email_hash) already applied (v0.5.11 carry-forward)
- [x] `FOMO_PIL_LIVE_ENABLED=false` at start (Path A boot)

## 2. Env additions

| Var | Set? | Value | Notes |
|---|---|---|---|
| `FOMO_PIL_LIVE_ENABLED` | ✓ | `false` (Path A) → `true` (Path B/C/D/E) | Q5.A — flipped at 11:17:19Z |
| `FOMO_PIL_SCORE_CAP` | ✓ | `0.15` | Q2.A default; bounds [0.05, 0.25] |
| `FOMO_PIL_DIVERGENCE_AUDIT_ENABLED` | ✓ | `false` (default) | Q4.A — OFF this phase |
| `FOMO_V0_5_12_BASELINE_CONFIRMED` | ✓ | `true` | set after §1 capture |
| `FOMO_V0_5_12_WINDOW_HOURS` | ✓ | `24` (default) | |

All v0.5.11 substrate env (`FOMO_PIL_K_THRESHOLD=3`, `FOMO_PIL_SCORE_DELTA=0.1` default, `FOMO_PIL_RECENCY_FULL_DAYS=90`, `FOMO_PIL_RECENCY_DECAY_DAYS=90`) unchanged. `FOMO_PIL_K_THRESHOLD` **now also gates the read side** post-fix.

## 3. PASS criteria (14)

| # | Criterion | Evidence | Got |
|---|---|---|---|
| C1 | Kill switch off → bit-identical v0.5.11 behavior (no PIL prompt block, no `brevio.rank.pil_applied` audit, single ranker call) | Path A rank_id=251 @ 11:13:14Z, `prompt_version='ranker-v0.2.0'`, 0 PIL audits in OFF window | ✓ |
| C2 | Kill switch on + matching canonical PIL row → PIL context included + audit fires with all 9 locked fields | Path C rank_id=253, scope_key_hash=`d1a6c6c65e9c5d7a363198a0af91c1b7`, all 9 fields populated (sample in §14) | ✓ |
| C3 | **Legacy `scope_key='message:<id>'` row produces null PIL context (LOAD-BEARING)** — no influence, no audit | 3 legacy `message:*` rows in DB; `pil_audits_with_legacy_scope=0`; BB6 PASS 3/3 | ✓ |
| C4 | Cap is enforced against a baseline no-PIL call (two-call hybrid; cap is REAL only if baseline runs) | Path C audit: `score_before_pil_cap=0.98`, `score_after_pil_cap=0.98`, `pil_score_delta=+0.030`, `was_capped=false`; baseline=0.95 → cap math checks out. BB8 PASS (max-input → `was_capped=true`, |delta|=0.150 = cap) | ✓ |
| C5 | `brevio.rank.pil_applied` fires only when PIL context is non-null (no audit on Path B null-context rank) | Path B rank_id=252, sender HMAC `30ca254fa546ee1101de71726ff4ea28` (no PIL row), 0 PIL audits in ON window for this rank | ✓ |
| C6 | Privacy canary — no raw sender_email/subject/body/snippet/header in audit detail | scanned 1 brevio.rank.pil_applied row in window vs 16 forbidden substrings (incl. `@example.com`, `Series A`, `term sheet`); **0 hits** | ✓ |
| C7 | **Cross-user contamination — DB layer (LOAD-BEARING)** — user A signal cannot leak to user B's rank | 0 founder PIL audits cite a scope_key without a founder memory_signal row; BB4 PASS 3/3; HMAC user_id construction architectural | ✓ |
| C8 | **Suppressed sender with high-intrinsic importance can still surface (BB1 LOAD-BEARING)** — model overrides prior | BB1 PASS 3/3 (live OpenAI): label=important, score≥0.7. Path C in-vivo confirmed: scope had `sender_suppressed=true` + `sender_importance=-0.30`, model produced label=important, score=0.98 | ✓ |
| C9 | Old/stale PIL signals decay to zero (BB3) — Q3.C | BB3 PASS 3/3: 200d old signal → `decay_factor_applied=0`, `sender_importance_score=0`. Shadow F6/F7/F8 (200d/135d/90d boundaries) all PASS 3/3 | ✓ |
| C10 | One false-positive correction does NOT materially change live ranker output (BB5) — Q3.A guardrail 4 | **BB5 contract restructured mid-phase.** Pre-fix BB5 (model noise-floor test) flaked 2/3 runs (FAIL/PASS/FAIL) — revealed read-side `k_threshold` gap allowing becomes-binary-blind on weak priors. Post-fix BB5 (architectural: `buildLivePilContext` returns null when `n_events < k_threshold` AND `!sender_suppressed`) PASS 3/3 deterministic. See §16 substitution note. | ✓ |
| C11 | **PIL cannot bypass score cap via prompt/model behavior (BB8 LOAD-BEARING)** | BB8 PASS 3/3: `pil_score_delta_was_capped=true`, |delta|=0.150 = FOMO_PIL_SCORE_CAP exactly | ✓ |
| C12 | v0.5.11 substrate remains unchanged — `applyPilAggregation` writes still happen, `brevio.signal.aggregated` still fires; PIL kinds + audit registry intact | smoke-evidence:v0.5.11 VERDICT: PASS (carry-forward run); 9 `brevio.signal.aggregated` audits in window; substrate kinds + sender_email_hash column intact | ✓ |
| C13 | v0.5.7 HMR + v0.5.10 reply parser remain unchanged — `human-message-v0.3.0`, `reply-parser-v0.2.0` | smoke-evidence assertion: `HMR template='human-message-v0.3.0'`, `reply-parser='reply-parser-v0.2.0'`, ranker baseline `'ranker-v0.2.0'` + PIL-block `'ranker-v0.3.0'` both valid | ✓ |
| C14 | 3E.1 invariant remains preserved — no LLM in body composition; only `rank.reason` is model-generated | smoke-evidence C14 OPERATOR+CODE-LEVEL: PIL block added ONLY to ranker prompt; `renderFounderText` + HMR remain deterministic | ✓ |

## 4. `smoke-evidence:v0.5.9` — **VERDICT: PASS**

13 PASS + 3 operator-confirmed criteria. Feedback substrate (BREVIO_FEEDBACK_SURFACES=13, ACTIVE=`['email_alert']`) intact. C13 Slack interactivity regression PASS (1 row from approval path). C16 privacy canary clean.

## 5–11. Prior-phase smoke-evidence carry-forward — **VERDICTs**

| Script | Verdict | Notes |
|---|---|---|
| `smoke-evidence:v0.5.9` | **PASS** | 13 PASS + 3 op-confirmed; cross-tenant + privacy canary clean |
| `smoke-evidence:v0.5.10` | **PASS** | 10 PASS + 6 op-confirmed; reply-parser intent set unchanged |
| `smoke-evidence:v0.5.11` | **PASS** | 7 PASS + 11 op-confirmed; **PIL substrate UNCHANGED** — LOAD-BEARING for C12; 9 brevio.signal.aggregated audits carry-forward; 0 cross-tenant leak |

> Operator note: v0.5.11 substrate UNCHANGED is the C12 contract. Verified.

## 12. `smoke-evidence:v0.5.12` output — **VERDICT: PASS** (7 PASS, 7 operator-confirmed, 0 warn)

```
========================================================================
Phase v0.5.12 evidence summary — 14 criteria (live ranker reads PIL guarded)
========================================================================
  [...] C1: Kill switch off → bit-identical v0.5.11 behavior (no PIL prompt block, no audit fires)
        OPERATOR-CONFIRMED. 1 brevio.rank.pil_applied row(s) in window. (the 1 row is the Path C synthetic-seed in the ON window — see §16 timeline)
  [✓] C2: Kill switch on + canonical PIL row → PIL context included + audit fires
        1 row(s); all 9 locked detail fields present on sample
  [...] C3: Legacy message:<id> placeholder row produces null PIL context (no audit, no influence)
        OPERATOR-CONFIRMED. 3 legacy message:<id> placeholder row(s) present in memory_signals
  [...] C4: Cap enforced via two-call hybrid (baseline + PIL); cap is REAL only if baseline runs
        1 audit row(s) in window; 0 had pil_score_delta_was_capped=true (Path C delta=+0.030 well within cap)
  [✓] C5: brevio.rank.pil_applied fires only when PIL context is non-null
        0 audit row(s) with empty pil_signal_kinds_present or empty scope_key_hash
  [✓] C6: Privacy canary — no raw private content in new audit
        scanned 1 brevio.rank.pil_applied row(s) against 16 forbidden substring(s); zero hits
  [✓] C7: Cross-user contamination — no founder PIL audit reads from a non-founder scope_key (LOAD-BEARING)
        0 contaminating row(s)
  [...] C8: Suppressed sender with high-intrinsic importance can still surface (BB1 LOAD-BEARING)
        OPERATOR + EVAL CONFIRMED. BB1 PASS 3/3; Path C in-vivo confirmed: suppressed scope produced label=important score=0.98
  [...] C9: Old/stale PIL signals decay to zero (BB3) — Q3.C
        OPERATOR + EVAL CONFIRMED. BB3 PASS 3/3
  [...] C10: One false-positive correction does not materially change live ranker (BB5)
        OPERATOR + EVAL CONFIRMED. Post-fix BB5 PASS 3/3 deterministic (was 2/3 FAIL pre-fix; see §16 substitution note)
  [✓] C11: PIL cannot bypass score cap (BB8 LOAD-BEARING)
        1 audit row(s); 0 with null/NaN pil_score_delta. BB8 PASS 3/3
  [✓] C12: v0.5.11 substrate remains unchanged
        Registry + module + column checks PASS. 9 brevio.signal.aggregated audit(s) in window
  [✓] C13: v0.5.7 HMR + v0.5.10 reply parser + ranker PROMPT_VERSION carry-forward unchanged
        HMR template='human-message-v0.3.0', reply-parser='reply-parser-v0.2.0', ranker='ranker-v0.2.0'/'ranker-v0.3.0' both valid
  [...] C14: 3E.1 invariant — no LLM in body composition; only rank.reason is model-generated
        OPERATOR + CODE-LEVEL CONFIRMED

VERDICT: PASS  (7 PASS, 7 operator-confirmed, 0 warn).
```

## 13. Path F — `eval:pil-live` output — **VERDICT: PASS** (deterministic 3/3 post-fix)

```
Phase v0.5.12 PIL live ranker eval harness

Fixtures: 11 shadow carry-forward + 8 LOAD-BEARING becomes-blind (BB1–BB8)
Model: live (OPENAI_API_KEY set; BB1/BB2/BB3* will call OpenAI)

-- Carry-forward shadow fixtures (deterministic) --
  [✓] F1 — clean baseline, no PIL row → unchanged
  [✓] F2 — strong positive (score=+0.5, 5 events, 1d old) → shift UP
  [✓] F3 — weak positive (score=+0.1, fresh) → small shift UP (direction-only)
  [✓] F4 — strong negative (score=-0.5, 5 events, fresh) → shift DOWN
  [✓] F5 — sender_suppressed=true → suppressed regardless of baseline
  [✓] F6 — ancient signal (200d old) → decay to zero, treated as unchanged
  [✓] F7 — mid-decay signal (135d old, score=+0.4) → factor 0.5, still UP
  [✓] F8 — boundary at 90d (still full weight) → shift DOWN
  [✓] F9 — score=0 (offsetting positive + negative events) → unchanged
  [✓] F10 — different scope_key (clean baseline at scope_C; row exists at scope_A) → unchanged
  [✓] F11 LOAD-BEARING — cross-user contamination: user B asks at user A scope_key → unchanged

-- BB1–BB8 LOAD-BEARING becomes-blind fixtures --
  [✓] BB1 — suppressed sender + URGENT/CEO email → model overrides; rank.reason mentions prior + override
  [✓] BB2 — sender_importance.score=-0.3 (no suppression) + normal-strength email → live within ±cap of baseline
  [✓] BB3 — sender_importance.score=+0.3 but 200d old → decay → effective score 0
  [✓] BB4 — cross-user contamination — userB.buildLivePilContext(SCOPE) returns null when row exists ONLY at userA
  [✓] BB5 — one false_positive (1 event, score=-0.1) below k_threshold → null PIL context (no model call, no cap-pin possible); suppression bypasses the floor
  [✓] BB6 — legacy scope_key="message:<id>" row → null PIL context (filter ignores legacy placeholder rows)
  [✓] BB7 — FOMO_PIL_LIVE_ENABLED=false + PIL rows present → pil_context null, single call, no audit, bit-identical v0.5.11
  [✓] BB8 — sender_importance.score=+1.0 (theoretical max) → pil_score_delta_was_capped=true; |delta| ≤ FOMO_PIL_SCORE_CAP

VERDICT: PASS — shadow 11/11 + BB 8/8 all green. (3 runs / 3 PASS — deterministic post-fix)
```

**8 LOAD-BEARING becomes-blind fixtures:**

| ID | Setup | Required result | Got |
|---|---|---|---|
| BB1 | sender_suppressed=true 3d old + "URGENT: from CEO" email | label=important, score ≥ 0.7, rank.reason mentions prior + override | ✓ |
| BB2 | sender_importance.score=-0.3 (no suppression) + normal-strength email | Live score within ±`FOMO_PIL_SCORE_CAP` of baseline | ✓ (\|delta\|=0.090 < 0.15) |
| BB3 | sender_importance.score=+0.3 but 200d old + weak signals | Live ≈ baseline (\|Δ\| ≤ 0.05); decay → effective score 0 | ✓ (decay_factor=0, score=0) |
| BB4 | Cross-user hash-collision sim (DB row at scope under userA's id) | `buildLivePilContext('userB', SCOPE)` reads userB row only; userA row ignored | ✓ |
| BB5 | Single false_positive (1 event, score=-0.1) **below k_threshold** | **`buildLivePilContext` returns null (k_threshold floor blocks weak prior); suppression bypasses the floor (BB1 path stays green); architectural assertion, no model call** | ✓ |
| BB6 | Legacy `message:<id>` row only; NO canonical HMAC row | `pil_context: null`; bit-identical to baseline; NO audit | ✓ |
| BB7 | `FOMO_PIL_LIVE_ENABLED=false` + PIL rows present + strong-signal email | `pil_context: null`; baseline-only call; no audit | ✓ |
| BB8 | sender_importance.score=+1.0 (theoretical max) | `pil_score_delta_was_capped=true`; \|delta\| ≤ `FOMO_PIL_SCORE_CAP` | ✓ (was_capped=true, \|delta\|=0.150) |

## 14. Sample audit JSON

**Sample `brevio.rank.pil_applied` audit detail (Path C — typical PIL-influenced rank, suppressed + URGENT override):**

```json
{
    "rank_result_id": 253,
    "scope_key_hash": "d1a6c6c65e9c5d7a363198a0af91c1b7",
    "source_surface": "email_alert",
    "pil_score_delta": 0.030000000000000027,
    "score_after_pil_cap": 0.98,
    "score_before_pil_cap": 0.98,
    "pil_signal_kinds_present": [
        "sender_importance",
        "sender_suppressed"
    ],
    "pil_score_delta_was_capped": false,
    "model_mentioned_pil_in_reason": false
}
```

All 9 locked structural fields present. Baseline 0.950 → PIL 0.980 → delta +0.030 (within cap; not capped). Model labeled important + score 0.98 despite `sender_suppressed=true` + `sender_importance.score=-0.30` prior — BB1-equivalent suppression override on the URGENT/CEO context.

**Sample `brevio.rank.pil_applied` audit detail (cap-enforced case, was_capped=true):**

In-vivo: cap was not hit during smoke (only one Path C rank executed; delta +0.030 well within ±0.15). In-vitro BB8 (`pil-live.eval.ts`) is the cap-enforcement adversarial proof:

```
BB8 result: delta=0.150 was_capped=true final_score=0.550
  - input: sender_importance.score=+1.0 (theoretical max)
  - baseline: ~0.40 (model decision on the synthetic email without PIL prior)
  - pil pre-cap: ~0.55+ (model nudged UP by the +1.0 prior block)
  - raw_delta: ~0.15+ → clamped to exactly +FOMO_PIL_SCORE_CAP (0.150)
  - was_capped: true
```

**Sample rank_results row showing v0.5.12 PIL-block prompt_version:**

```
rank_result_id=253  prompt_version=ranker-v0.3.0  label=important  score=0.980  reason_head="Your counsel needs your signature on the Series A term sheet by 9pm tonight."
```

**Kill-switch-off rank_results row (Path A, bit-identical v0.5.11):**

```
rank_result_id=251  prompt_version=ranker-v0.2.0  label=not_important  score=0.050  user_id=14a6639f-2776-4b1a-af9b-834fc1855899  (non-founder)
```

(Path A naturally fell on a non-founder user because founder had `stop_active=true` at boot; the kill-switch-off contract is per-rank, not per-user. Founder's STOP cleared at 11:15:06Z via inbound START; Path B at rank_result_id=252 then ran for founder with `prompt_version='ranker-v0.2.0'` under kill-switch ON because the fresh sender HMAC `30ca254fa546ee1101de71726ff4ea28` had no canonical PIL row → `buildLivePilContext` returned null → baseline-only call → `ranker-v0.2.0`. C5 PASS.)

## 15. Operator-confirmed smoke evidence

| Check | Confirmed? | Notes |
|---|---|---|
| **Path A (LOAD-BEARING C1): kill switch off → bit-identical v0.5.11** | ✓ | rank_result_id=251, `prompt_version='ranker-v0.2.0'`, 0 `brevio.rank.pil_applied` audits in OFF window (11:12:58Z → 11:17:19Z) |
| Path A: single ranker call per rank (NOT two-call hybrid) | ✓ | label produced by single-call baseline-only path; no Path A audit row implies hybrid did not run |
| Path B: kill switch on, no canonical PIL row → no audit, pil_context null | ✓ | rank_result_id=252, founder, sender HMAC `30ca254fa546ee1101de71726ff4ea28` has 0 PIL rows for founder; 0 PIL audits cite this rank_id; `prompt_version='ranker-v0.2.0'` |
| **Path C (LOAD-BEARING C2 + C4 + C5): kill switch on, canonical PIL row → two-call hybrid + audit with all 9 fields** | ✓ | rank_result_id=253, scope_key_hash=`d1a6c6c65e9c5d7a363198a0af91c1b7`, all 9 fields populated, `prompt_version='ranker-v0.3.0'` (synthetic-seed substitute — see §16) |
| Path C: score_after_pil_cap = baseline_score + clamp(pil_score - baseline_score, ±cap) | ✓ | baseline=0.950, pil=0.980, raw_delta=+0.030 < +0.150 cap → clamped_delta=+0.030, final=0.950 + 0.030 = 0.980 = audit.score_after_pil_cap ✓ |
| Path C: model_mentioned_pil_in_reason bool computed via regex on rank.reason text (text itself NOT in audit) | ✓ | audit detail contains the bool only (`false` on this sample — reason text "Your counsel needs your signature..." doesn't reference "based on past" / "you previously"); raw reason text NOT in audit detail |
| **Path D (LOAD-BEARING C3): legacy message:<id> row only → null PIL context, no audit** | ✓ | 3 legacy `message:*` rows in DB for founder (carry-forward from v0.5.10/v0.5.11); query `pil_audits_with_legacy_scope=0`; BB6 PASS 3/3 in-vitro |
| **Path E (LOAD-BEARING C7): cross-user contamination test** | ✓ | inverse query: 0 founder PIL audits cite a scope_key that lacks a founder memory_signal row; BB4 PASS 3/3 in-vitro (adversarial DB seeding userA at scope SCOPE; userB.buildLivePilContext(SCOPE)→null) |
| **Path F BB1 (LOAD-BEARING C8): suppressed sender + URGENT email surfaces** | ✓ | 3/3 PASS; live OpenAI; corroborated by Path C in-vivo |
| Path F BB2: not-yet-suppressed score within cap | ✓ | 3/3 PASS; \|delta\|=0.090 |
| **Path F BB3 (LOAD-BEARING C9): 200d-old signal decays to zero** | ✓ | 3/3 PASS; decay_factor=0 |
| Path F BB4: cross-user read isolation | ✓ | 3/3 PASS deterministic |
| **Path F BB5 (LOAD-BEARING C10): k_threshold floor blocks weak prior; suppression bypasses** | ✓ | 3/3 PASS deterministic post-fix (architectural assertion). See §16 substitution note. |
| **Path F BB6 (LOAD-BEARING C3 in-vitro): legacy placeholder → null** | ✓ | 3/3 PASS deterministic |
| **Path F BB7 (LOAD-BEARING C1 in-vitro): kill switch off → null** | ✓ | 3/3 PASS deterministic |
| **Path F BB8 (LOAD-BEARING C11): cap not bypassable** | ✓ | 3/3 PASS; `was_capped=true`, \|delta\|=0.150 |
| §11 Live ranker invariant: kill-switch gate present BEFORE buildLivePilContext call site | ✓ | `apps/fomo/src/index.ts:1130-1140` builds `pilLive` dep only when `FOMO_PIL_LIVE_ENABLED=true`; `apps/fomo/src/workers/gmail-poll.ts:677` checks `deps.pilLive?.enabled` before calling `buildLivePilContext` |
| §11 rank_results schema unchanged | ✓ | column list identical to v0.5.11 baseline (no new `pil_*` columns) |
| §11 k_threshold wired at production deps | ✓ | `apps/fomo/src/index.ts:281` passes `k_threshold: tunables.k_threshold` into `pilContextDeps` (post-fix) |
| §12 v0.5.9 + v0.5.10 + v0.5.11 evidence scripts still PASS | ✓ | 3 PASS verdicts |
| Code-level: `buildLivePilContext` exported from `pil-context.ts`; `pil-live.eval.ts` present with 11 carry-forward + 8 BB fixtures | ✓ | grep confirmed; both files tracked |
| Unit-test sanity: all new tests green | ✓ | 65/65 PIL-related unit tests PASS (includes 5 new `k_threshold` floor tests in `pil-live-context.test.ts`) |

## 16. Operator notes / honest substitutions

### 16.1 Mid-smoke runtime fix (LOAD-BEARING)

The pil-live eval ran 3 times during §10 and BB5 flaked **2 FAIL / 1 PASS** (FAIL → PASS → FAIL). Investigation showed:

- `FOMO_PIL_K_THRESHOLD=3` was applied **write-side only** in `apps/fomo/src/memory/pil-aggregation.ts:298` (gates `sender_suppressed` writes). The READ path (`buildLivePilContext` → `pil-context.ts` → ranker prompt block) had no n_events floor.
- Production DB already contained a 1-event row at scope `9c0e077d84380c2067b1158deeec5d83` (`sender_importance.score=-0.10, n_negative_events=1`) — created by v0.5.11 substrate aggregation BEFORE k_threshold gating fired.
- Live ranker on borderline emails (e.g. BB5's "Scheduled maintenance Sunday 2am-4am") pinned the score against `-FOMO_PIL_SCORE_CAP` (delta=−0.150) on a single-event prior — the **becomes-binary-blind on a weak signal** failure mode explicitly forbidden by founder rule + `[[real-or-absent-no-half-wired]]`.

**Fix landed mid-smoke** (4 files):

1. [`apps/fomo/src/ranker/pil-context.ts`](apps/fomo/src/ranker/pil-context.ts) — added optional `k_threshold` to `PilContextDeps`; `buildLivePilContext` returns null when `n_events < k_threshold` AND `!sender_suppressed`. Suppression bypasses (BB1 path stays green; explicit ignore is binary).
2. [`apps/fomo/src/index.ts:281`](apps/fomo/src/index.ts#L281) — passes `tunables.k_threshold` into production `pilContextDeps`.
3. [`apps/fomo/src/eval/pil-live.eval.ts`](apps/fomo/src/eval/pil-live.eval.ts) — BB5 refactored from flaky model-behavior assertion to deterministic in-vitro architectural assertion (seed memory_signal store with n=1, verify `buildLivePilContext` returns null with k=3; verify suppression bypass; verify `k_threshold=undefined` preserves shadow path). **No model call.**
4. [`apps/fomo/src/ranker/pil-live-context.test.ts`](apps/fomo/src/ranker/pil-live-context.test.ts) — 5 new unit tests: null on `n_events < k`, non-null at floor, suppression bypass, `k_threshold=undefined` no-filter, `k_threshold=0` no-filter.

**Post-fix eval: 3 runs, 3 PASS, all 19 fixtures green every run.** Deterministic. 65/65 PIL unit tests PASS.

**This fix should land as a new runtime commit on this branch before PR merge.** Without it, v0.5.12 ships the becomes-binary-blind mode in production.

### 16.2 Path C substitution (runbook §7.2 sanctioned)

Path C in-vivo requires an email from a sender whose HMAC matches a canonical-HMAC PIL row in the founder substrate. The founder does not know which raw `sender_email` maps to each substrate `scope_key` (HMAC includes `BREVIO_SENDER_HASH_KEY` + user_id; opaque). Per runbook §7.2: synthetic-seed substitute is sanctioned.

[`apps/fomo/scripts/_smoke-v0.5.12-seed-path-c.ts`](apps/fomo/scripts/_smoke-v0.5.12-seed-path-c.ts) invokes the **same** `buildLivePilContext` → `rankEmailWithLivePil` → `writeBrevioRankPilAppliedAudit` code path that the production polling worker calls. It writes through `PostgresRankResultStore` + `PostgresAuditStore` (the same factories `apps/fomo/src/index.ts:225-291` uses). The seed used scope `d1a6c6c65e9c5d7a363198a0af91c1b7` (BB1-equivalent: `sender_importance.score=-0.30, n=3` + `sender_suppressed=true`). Real OpenAI call. Real rank_result + audit row written to live Neon DB.

This is in-vivo enough to confirm C2 + C4 + C5 contracts. **Substitute disclosed; not oversold as full natural-email proof.**

### 16.3 Stale `dist/` during in-vivo Paths A + B (acceptable for those paths)

The dev server runs from `dist/src/index.js`. `dist/` was last built at `Jun 7 00:35:04Z` — BEFORE the mid-smoke k_threshold fix (16.1). Paths A and B in-vivo ran against stale dist. **This is acceptable** because:
- Path A (kill switch OFF) does not invoke `buildLivePilContext` at all (`pilLive` dep is `null` when `FOMO_PIL_LIVE_ENABLED=false`). k_threshold filter irrelevant. Contract holds.
- Path B sender had no PIL row at all (`30ca254fa546ee1101de71726ff4ea28` not in substrate). `buildLivePilContext` returns null at the existing pre-fix check (`importance===null && suppressed===null`). k_threshold filter never reached. Contract holds.
- Path C ran via `_smoke-v0.5.12-seed-path-c.ts` via the test loader (`--experimental-strip-types`), which executes the **fresh TypeScript sources** including the fix.
- The PR build will regenerate `dist/` from the post-fix sources before merge.

### 16.4 Path A natural rank on non-founder user

Path A's natural rank (rank_id=251) fell on user `14a6639f-2776-4b1a-af9b-834fc1855899` (a friend-beta user), not founder, because founder's `stop_active=true` was set at boot (age 10.4h). The kill-switch-off contract is per-rank, not per-user — any rank during the OFF window produces `ranker-v0.2.0` with no `brevio.rank.pil_applied` audit. Founder cleared STOP at 11:15:06Z via SendBlue inbound; Path B then ran for founder (rank_id=252) at 11:20:30Z under kill-switch ON.

### 16.5 Kill-switch timeline

```
OFF window:  2026-06-07T11:12:58Z  →  2026-06-07T11:17:19Z   (4m21s)
ON  window:  2026-06-07T11:17:19Z  →  (smoke end)

Rank_result_ids observed in OFF window: [251]
Rank_result_ids observed in ON window:  [252 (Path B), 253 (Path C synthetic-seed)]
brevio.rank.pil_applied audits in OFF window: 0  ✓ (expected)
brevio.rank.pil_applied audits in ON window:  1  ✓ (Path C synthetic-seed; rank_id=253)
```

## 17. Founder observations

| Observation | Note |
|---|---|
| Does the two-call hybrid feel like the right shape, or would post-hoc-only (Q1.B) have been better as a first step? | _<to fill>_ |
| Is `FOMO_PIL_SCORE_CAP=0.15` the right value, or should it be tighter / looser based on observed shift magnitudes? | Observed magnitudes: BB2=0.090 (mid-strength prior), BB8=0.150 (max-input → capped), Path C=0.030 (suppressed+importance combined, model overrode). Cap rarely binding except on synthetic max. _<founder to weigh in>_ |
| Did the model mention PIL prior in `rank.reason` often enough to feel transparent? | Path C: `model_mentioned_pil_in_reason=false` on the suppressed+URGENT case (reason was direct: "Your counsel needs your signature..."). The model produced a correct override but didn't acknowledge the prior textually. _<founder to weigh in on whether prompt-tune is warranted>_ |
| Did any BB fixture surface a real edge case worth promoting to a hardening item? | **Yes — BB5.** See §17 bonus findings #1. |
| Does v0.5.12 feel like the right shape for v0.5.13+ next phase? | _<founder to weigh in>_ |
| Should `FOMO_PIL_LIVE_ENABLED` stay TRUE in production after smoke PASS, or stay false until further validation? | **OFF** (runbook §14 safe state). Production stays bit-identical to v0.5.11. Flip ON later via env-only change once additional validation warrants it. |

### Bonus findings (real-incident-backed, candidates for next-phase 6Q gate)

1. **Read-side `k_threshold` floor was missing — fixed mid-phase (§16.1).** Permanent invariant: any future memory_signal kind that gates production behavior via a write-side n_events threshold must also gate the **read side** symmetrically. Production DB already had a 1-event row that the pre-fix live ranker would have cap-pinned on borderline emails. Add to permanent architecture checks: "Are write-side aggregation gates and read-side context-building gates symmetric for every memory_signal kind?"

2. **BB5 noise-floor model-behavior test was inherently brittle.** When the cap is the limiting factor and the email is borderline, model nondeterminism can blow past a noise-floor magnitude threshold even when the prior should not matter. **Lesson:** when a fixture is designed to assert "no signal influence," prefer the architectural assertion ("buildLivePilContext returns null") over the model-behavior assertion ("|Δ| ≤ N"). The architectural form is deterministic and load-bearing in a way model output never can be.

3. **Dev server runs from `dist/`; mid-smoke source edits don't reach the production code path unless rebuilt.** Future smokes that edit source mid-run should either (a) rebuild dist + re-boot dev server, or (b) run any in-vivo proof through `--experimental-strip-types` against fresh TS (as Path C synthetic-seed did). PR build will regenerate `dist/` from post-fix sources.

4. **Path C in-vivo via natural Gmail is friction-heavy.** The founder cannot predict which sender_email produces which substrate scope_key. Future PIL phases that need in-vivo Path-C-like proofs should ship a developer-only HMAC-helper script (or a substrate-row-to-sender-email reverse index that the founder maintains for smoke purposes). Synthetic-seed substitution is sanctioned but adds 5-10 lines of script + DB state to the smoke. Worth considering whether the run cost of "type the right sender" is worth optimizing.

## 18. Verdict

**✓ PASS** — all 14 criteria green; BB1–BB8 all PASS in `eval:pil-live` (deterministic 3/3 post-fix); §5 Path A kill-switch-off bit-identical contract verified in-vivo; §7 Path C two-call hybrid + cap + audit with all 9 fields verified via synthetic-seed substitute (§16.2); §8 Path D legacy-placeholder-ignored verified in-vivo (3 legacy rows + 0 audits cite them); §9 Path E cross-user contamination fails closed (in-vivo + BB4); §12 v0.5.9 + v0.5.10 + v0.5.11 evidence still PASS; privacy canary clean (0 hits on 16 forbidden substrings).

Mid-smoke runtime fix (§16.1) landed as commit `39b034c2` on this branch. Without that commit, v0.5.12 would ship the becomes-binary-blind failure mode in production via the existing 1-event row at scope `9c0e077d84380c2067b1158deeec5d83`. With it landed, the branch is ready for PR → main.

Failures / followups:

- (none blocking) — open PR `phase-v0.5.12-live-ranker-pil-guarded` → `main` when ready.

## 19. Sign-off

- Founder signature: Galiette Mita
- Date: 2026-06-07
- No friend consent needed this phase (founder-only smoke)

## 20. Aftercare confirmation

- [x] Killed dev server @ 11:35:17Z; port 8080 free
- [x] No friend deletion ops (no friend involved)
- [x] v0.5.7 HMR template_version still `human-message-v0.3.0` (no renderer edits this phase)
- [x] v0.5.9 substrate unchanged: BREVIO_FEEDBACK_SURFACES=13, ACTIVE=`['email_alert']`
- [x] v0.5.10 reply-parser unchanged: `PROMPT_VERSION='reply-parser-v0.2.0'`, 8 intents
- [x] v0.5.11 substrate write path unchanged: `applyPilAggregation` + `brevio.signal.aggregated` audit still fire on natural-reply feedback
- [x] No LLM call introduced into renderer (3E.1 invariant)
- [x] No raw private content in any new audit detail OR ranker PIL prompt block (C6 canary PASS, 0 hits)
- [x] Read-side filter enforced: `scope_key ~ '^[a-f0-9]{32}$'` AND `user_id = userId` (legacy `message:<id>` rows ignored); **AND `n_events ≥ k_threshold` floor when `!sender_suppressed`** (post-fix)
- [x] Kill switch contract verified: off → bit-identical v0.5.11
- [x] Production divergence audit (`FOMO_PIL_DIVERGENCE_AUDIT_ENABLED`) remains OFF by default
- [x] No new active `source_surface` activated beyond `email_alert`
- [x] No new memory_signal kinds beyond v0.5.11's two
- [x] Final state: `FOMO_PIL_LIVE_ENABLED=false` (kept OFF for production safety per runbook §14; documented in §17)

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
