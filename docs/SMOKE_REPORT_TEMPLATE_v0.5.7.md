# Phase v0.5.7 Smoke Test Report — Human Message Renderer

> Filled after running every step in `smoke-test-v0.5.7-human-message-renderer.md`.
> Commit as `docs/SMOKE_REPORT_v0.5.7.md` once **`VERDICT: PASS`** on all 14 criteria AND §6 Test 3 load-bearing taste check is operator-confirmed against rendered bodies.
>
> **Scaffolding-vs-runtime note:** if this report is filled before the runtime implementation commit lands, `smoke-evidence:v0.5.7` will print `VERDICT: PENDING` (not PASS). That is expected at scaffolding time. The report can only legitimately reach `VERDICT: PASS` after both the SCAFFOLDING commit and the RUNTIME commit are on this branch AND the §6 test sequence has been run end-to-end.
>
> **C10 correction:** Real iMessage delivery is OPPORTUNISTIC ONLY. Founder taste check on **rendered bodies** (via the taste-check fixture) is load-bearing. If SendBlue OPTED_OUT / tier state still blocks delivery, mark real iMessage as `N/A — BLOCKED BY SENDBLUE STATE`, NOT failure.
>
> **v0.5.5-specific accommodation:** `smoke-evidence:v0.5.5` may legitimately FAIL (external blocker per PR #43 SMOKE_REPORT_v0.5.5.md). Operator confirms in §8 that the v0.5.5 FAIL shape is identical to the prior record — same C3/C8/C11 blocked-external lines — and is NOT a new regression caused by v0.5.7.
>
> **v0.5.7 PASS does NOT auto-unlock** F1 SendBlue tier fix, PIL substrate, Friend C, auto-send, 3E.1 reversal, ranker rewrite beyond `reason` field, B3, new email/model providers, dashboard, any L2+ surfaces, or any second HMR surface (calendar/drafts/etc.). The next phase runs its own 6-question gate.

---

**Founder:** Galiette Mita
**Run date:** _<YYYY-MM-DD HH:MM TZ>_
**Branch:** `phase-v0.5.7-human-message-renderer`
**Scaffolding commit SHA:** _<sha>_
**Runtime commit SHA:** _<sha>_
**Smoke window override (if any):** `FOMO_V0_5_7_WINDOW_HOURS=24` (default)
**Path:** ☐ Path A (load-bearing taste check only) | ☐ Path B (taste check + opportunistic real iMessage)
**Founder iPhone last 4 (only if Path B):** _<xxxx>_
**SendBlue from-number used (only if Path B):** _<from-number>_
**SMOKE_START_TS:** _<UTC ISO timestamp>_

---

## 1. Prerequisites confirmed

- [ ] PR #45 (v0.5.6) on `main` with VERDICT: PASS
- [ ] No friend involvement (three-friend cap holds)
- [ ] ngrok healthy + forwarding to localhost:8080 — **only if Path B**
- [ ] SendBlue Sandbox tier active for `SENDBLUE_FROM_NUMBER` — **only if Path B**
- [ ] Founder un-flagged from SendBlue OPTED_OUT — **only if Path B; N/A if Path A**
- [ ] §1 baseline snapshots captured BEFORE smoke start (both `stop_active` and `fomo.send.attempted`)
- [ ] Founder iPhone is the device for §6 Test 3 real-iMessage taste check — **only if Path B**
- [ ] Taste-check fixture script `apps/fomo/scripts/render-hmr-samples.ts` shipped by runtime commit (load-bearing for Test 3 Path A)

## 2. Env additions (redacted)

| Var | Set? | Notes |
|---|---|---|
| `FOMO_V0_5_7_BASELINE_CONFIRMED` | ☐ | set `true` after §1 capture |
| `FOMO_V0_5_7_WINDOW_HOURS` | ☐ | default 24 |

All other v0.5.4 / v0.5.5 / v0.5.6 env vars unchanged.

## 3. PASS criteria (14 — Human Message Renderer)

| # | Criterion | Evidence | Got |
|---|---|---|---|
| C1 | `fomo.alert.hmr_degradation_applied` registered in `FOMO_AUDIT_ACTIONS` | _<preflight + smoke-evidence output>_ | ☐ |
| C2 | `FOUNDER_TEXT_TEMPLATE_VERSION` bumped to `human-message-v0.3.0` | _<runtime constant value>_ | ☐ |
| C3 | Recent `fomo.send.attempted` rows carry the bumped `template_version` | _<query output + sample row>_ | ☐ |
| C4 | Body length within target 220–280 / hard cap 320 / absolute 340 (carry-forward) | _<median content_chars + N over hard>_ | ☐ |
| C5 | NO arbitrary ellipsis (`…`) — sentence-boundary truncation only (carry-forward) | _<regression test name + operator visual ref>_ | ☐ |
| C6 | **Body composition reads as natural 1–2 sentence(s) — NOT field-newline list (load-bearing taste check)** | _<§6 Test 3 fixture-script output + operator §10 sign-off>_ | ☐ |
| C7 | Sender-resolution + Modified Q2.B chain works for all 4 paths | _<unit test names + audit `sender_resolution_path` distribution>_ | ☐ |
| C8 | Subject naturalization rules fire deterministically per Q3.B lock | _<unit test names + audit `subject_strip_applied` distribution>_ | ☐ |
| C9 | Reason voice per Q4.A lock (`2p_action` after ranker-v0.2.0 rollout; `legacy_3p` transitional) | _<audit `reason_voice` distribution + prompt_version sample>_ | ☐ |
| C10 | **Manual founder taste check on RENDERED BODIES (load-bearing)** + opportunistic real iMessage if SendBlue allows | _<§10 sign-off + N rendered samples + (Path B) received iMessage text>_ | ☐ |
| C11 | Cross-tenant isolation — only founder touched in smoke window | _<§6 Test 4 non-founder diff + non-founder send count>_ | ☐ |
| C12 | 3E.1 preserved — body composition deterministic; only `rank.reason` model-generated | _<runtime unit test name asserting no LLM import in renderer module + PR-review confirmation>_ | ☐ |
| C13 | Zero email-content leakage in audit detail (carry-forward + new HMR fields) | _<leak-canary scan output; new fields are structural enums per Q5.A>_ | ☐ |
| C14 | All prior smoke-evidence scripts still PASS (v0.5.3/4/5 may legitimately FAIL per documented shapes — operator confirms identical shape) | _<§4–§9 outputs + confirmation>_ | ☐ |

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

> Operator note: v0.5.7 is a founder-only smoke — no `/onboard/callback` runs. If v0.5.3 reports Item #1 FAIL with no contact_registered rows, that's expected blocked-substrate, NOT a v0.5.7 regression. Same shape as SMOKE_REPORT_v0.5.6.md §6.

## 7. `smoke-evidence:v0.5.4` output (`FOMO_V0_5_4_WINDOW_HOURS=168`) — **VERDICT: _<…>_**

```
_<paste full output>_
```

> Operator note: if v0.5.4 reports C13/C14 window-slide FAIL shape that PR #43 / SMOKE_REPORT_v0.5.6 §7 documented, that's NOT a v0.5.7-caused regression. Confirm rows are not from the v0.5.7 smoke window.

## 8. `smoke-evidence:v0.5.5` output — **VERDICT: _<…>_**

```
_<paste full output>_
```

> Operator note: if v0.5.5 reports C2/C3/C11 SendBlue OPTED_OUT blocked-external FAIL — same shape PR #43 documented — that's NOT a v0.5.7-caused regression. F1 SendBlue tier fix is its own future-phase candidate, NOT in v0.5.7 scope.

## 9. `smoke-evidence:v0.5.7` output (Human Message Renderer proof) — **VERDICT: _<…>_**

```
_<paste full output>_
```

## 10. Operator-confirmed taste check + (Path B) real iMessage

| Check | Confirmed? | Notes |
|---|---|---|
| Test 1: Mock regression produced ≥1 `fomo.send.attempted` row on `human-message-v0.3.0` template | ☐ | _<smoke-start timestamp + sample row>_ |
| Test 1: Four new audit fields populated (sender_resolution_path / subject_strip_applied / reason_voice / template_shape) | ☐ | _<sample row JSON>_ |
| Test 1: `content_chars` within 220–320 (220 informational floor; 320 hard cap) | ☐ | _<median + max>_ |
| Test 2: Q5.A degradation matrix — ≥1 `fomo.alert.hmr_degradation_applied` row | ☐ | _<sample row + which fallback path>_ |
| Test 2: Companion `fomo.alert.drafter_schema_failed` still fires on reason-schema violation (v0.5.6 carry-forward) | ☐ | _<count>_ |
| Test 2: Zero retry (only one `fomo.send.attempted` per alert_id post-violation) | ☐ | _<query output>_ |
| Test 2: No raw subject/body/header in any new audit field (structural enums only) | ☐ | _<leak scan>_ |
| **Test 3: Taste-check fixture script ran; founder eye-tested N rendered bodies against founder example bar (LOAD-BEARING)** | ☐ | _<fixture output paste>_ |
| Test 3: Rendered bodies read as natural 1–2 sentences, not field-newline list | ☐ | |
| Test 3: Opens with person-named or domain-named sender; NOT `g***@…`-style masked email | ☐ | (Modified Q2.B invariant) |
| Test 3: No `FOMO · IMPORTANT (0.92)` header (v0.5.6 carry-forward) | ☐ | |
| Test 3: No arbitrary `…` ellipsis (v0.5.6 carry-forward) | ☐ | |
| Test 3: Subject reads cleanly (no `[bracketed]`, no `Re:`/`Fwd:` artifacts) | ☐ | (Q3.B invariant) |
| Test 3: `Why_clause` reads as 2nd-person action prose (`2p_action`) | ☐ / acceptable transitional `legacy_3p` | (Q4.A invariant) |
| Test 3: Length feels right for lock-screen reading | ☐ | |
| Test 3: **Feels like a person curated it** (founder example bar) | ☐ | _<founder note>_ |
| Test 3 Path B: Founder iPhone received iMessage | ☐ / N/A — BLOCKED BY SENDBLUE STATE | _<arrival time>_ |
| Test 3 Path B: Real iMessage matches the fixture-rendered shape | ☐ / N/A | |
| Test 4: `stop_active` baseline-vs-post NON-FOUNDER diff is empty | ☐ | (v0.5.7 must NOT touch non-founder stop_active) |
| Test 4: Only `actor_user_id='founder'` in `fomo.send.attempted` for smoke window | ☐ | |

**N rendered sample bodies from `pnpm run render-hmr-samples` (paste verbatim):**

```
_<paste verbatim>_
```

**Test 3 Path B exact received iMessage text (if applicable):**

```
_<paste verbatim; N/A — BLOCKED BY SENDBLUE STATE if Path A>_
```

## 11. Founder observations

| Observation | Note |
|---|---|
| Compared to v0.5.6, does the new shape feel like a person curated it? | _<…>_ |
| Did the sender opener (first-name / domain-label) feel right across rendered samples? Any awkward cases? | _<…>_ |
| Did the Q3.B subject stripping feel right? Any noisy subjects that should have been stripped further? (v0.5.8 candidate) | _<…>_ |
| Did the ranker-v0.2.0 2nd-person voice land as expected? Any `legacy_3p` rows in distribution? Any reason text that should still be `2p_action`? | _<…>_ |
| Did the Q5.A degradation fallbacks (especially "Someone" + `subject_empty`) feel acceptable, or do any need a rewrite? | _<…>_ |
| Did the `template_shape='fallback_string'` rate (if any) feel acceptable in real traffic? | _<…>_ |
| Anything in audit_log that surprised you? | _<…>_ |
| What would you want different before either (a) shipping more HMR-surface scope (calendar / drafts / etc.), or (b) starting PIL substrate (per-user tone)? | _<…>_ |

### Bonus findings (real-incident-backed, candidates for next-phase 6Q gate)

1. _<…>_
2. _<…>_

## 12. Verdict

☐ **PASS** — all 14 criteria green; §6 Test 3 fixture-script taste check confirmed (load-bearing); §6 Test 4 cross-tenant non-founder diff empty; all 7 evidence scripts as expected (v0.5.3/4/5 may legitimately FAIL per documented shapes; operator confirmed identical shape). **Next phase runs its own 6-question gate.**

☐ **FAIL** — list below.

☐ **PENDING** — runtime commit not yet on branch; re-run after runtime lands.

Failures / followups:

- _…_

## 13. Sign-off

- Founder signature: Galiette Mita
- Date: _<YYYY-MM-DD>_
- No friend consent needed this phase (founder-only smoke)

## 14. Aftercare confirmation

- [ ] If Test 2 used a temporarily lowered schema cap or env var override, it was unset
- [ ] If Path B Test 3 re-flagged founder OPTED_OUT during real-iMessage attempt, no auto-START sent (founder decides)
- [ ] No friend deletion ops (no friend involved)
- [ ] v0.5.6 deterministic shell still functional — confirmed via smoke-evidence:v0.5.6 carrying forward (no regression)
- [ ] Dev server (Terminal 1) and any background watchers stopped after the smoke
- [ ] No LLM call accidentally introduced in `renderHumanMessage` (3E.1 invariant; confirmed by runtime unit test + PR review)

## 15. What v0.5.7 PASS does NOT promise

v0.5.7 PASS unlocks the next 6-question gate. It explicitly does NOT auto-unlock:

- **F1 SendBlue tier fix / un-flag** — its own future-phase candidate
- **Personalized Importance Learning substrate** — separate phase per [docs/personalized-importance-learning.md](personalized-importance-learning.md)
- **Friend C onboarding** — three-friend cap; Friend C is OPTIONAL
- **Auto-send** — its own gate per FOMO_PLAN v0.8
- **Reversal of 3E.1 no-LLM-body-generation directive** — v0.5.7 PRESERVES 3E.1 by design (renderer is pure; ranker prompt rewrite is the existing model call)
- **Per-user tone customization** — PIL-adjacent
- **Ranker rewrite beyond the `reason` field** — only the reason field's prompt voice changes in v0.5.7
- **Google OAuth verification submission (B3)** — multi-week external
- **A new email provider** — Gmail remains only active provider per [FOMO_DESIGN.md §6.2](../FOMO_DESIGN.md)
- **A new model provider** — OpenAI-first per [FOMO_DESIGN.md §18](../FOMO_DESIGN.md)
- **Dashboard / web UI**
- **Calendar / Drafting / MCP / browser automation surfaces** — each is a separate HMR surface, each own 6Q gate
- **HMR plugin registry / multi-surface framework** — Q6.A restraint
- **Short-body length policy resolution** — its own future gate per v0.5.6 PASS bonus finding #1
- **Runbook drift-detector amendment** — its own future gate per v0.5.6 PASS bonus finding #2

The next phase is decided AT THE NEXT 6-question gate.
