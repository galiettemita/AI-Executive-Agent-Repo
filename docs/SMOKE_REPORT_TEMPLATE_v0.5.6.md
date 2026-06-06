# Phase v0.5.6 Smoke Test Report — iMessage Tone + Summary Length

> Filled after running every step in `smoke-test-v0.5.6-imessage-tone.md`.
> Commit as `docs/SMOKE_REPORT_v0.5.6.md` once **`VERDICT: PASS`** on ALL SIX evidence scripts (v0.5.1 + v0.5.2 + v0.5.3 + v0.5.4 + v0.5.5 + v0.5.6) AND §6 Test 3 manual taste check is operator-confirmed.
>
> **Scaffolding-vs-runtime note:** if this report is filled before the runtime implementation commit lands, `smoke-evidence:v0.5.6` will print `VERDICT: PENDING` (not PASS). That is expected at scaffolding time. The report can only legitimately reach `VERDICT: PASS` after both the SCAFFOLDING commit and the RUNTIME commit are on this branch and the §6 test sequence has been run end-to-end.
>
> **v0.5.5-specific accommodation:** `smoke-evidence:v0.5.5` may legitimately FAIL (external blocker per PR #43 SMOKE_REPORT_v0.5.5.md). Operator confirms in §6 that the v0.5.5 FAIL shape is identical to PR #43's record — same C3/C8/C11 blocked-external lines — and is NOT a new regression caused by v0.5.6.
>
> **v0.5.6 PASS does NOT auto-unlock v1.0, Friend C, auto-send, F1 SendBlue unblock, PIL substrate, or any other phase.** The next phase runs its own 6-question gate.

---

**Founder:** Galiette Mita
**Run date:** _<YYYY-MM-DD HH:MM TZ>_
**Branch:** `phase-v0.5.6-imessage-tone`
**Scaffolding commit SHA:** _<sha>_
**Runtime commit SHA:** _<sha>_
**Smoke window override (if any):** `FOMO_V0_5_6_WINDOW_HOURS=24` (default)
**Founder iPhone last 4 (for traceability — last 4 only):** _<xxxx>_
**SendBlue from-number used:** _<from-number>_

---

## 1. Prerequisites confirmed

- [ ] PR #43 (v0.5.5) on `main` with its known FAIL-external-blocker verdict
- [ ] No friend involvement (three-friend cap holds)
- [ ] ngrok healthy + forwarding to localhost:8080 (required only for §6 Test 3)
- [ ] SendBlue Sandbox tier active for `SENDBLUE_FROM_NUMBER` (required only for §6 Test 3)
- [ ] Founder un-flagged from SendBlue OPTED_OUT (required only for §6 Test 3 — N/A if Test 3 not run)
- [ ] §1 baseline snapshots captured BEFORE smoke start (both `stop_active` and `fomo.send.attempted`)
- [ ] Founder iPhone is the device for §6 Test 3 taste check

## 2. Env additions (redacted)

| Var | Set? | Notes |
|---|---|---|
| `FOMO_V0_5_6_BASELINE_CONFIRMED` | ☐ | set `true` after §1 capture |
| `FOMO_V0_5_6_WINDOW_HOURS` | ☐ | default 24 |

All other v0.5.4 / v0.5.5 env vars unchanged.

## 3. PASS criteria (12 — iMessage Tone + Summary Length)

| # | Criterion | Evidence | Got |
|---|---|---|---|
| C1 | `fomo.alert.drafter_schema_failed` registered in `FOMO_AUDIT_ACTIONS` | _<preflight + smoke-evidence output>_ | ☐ |
| C2 | `FOUNDER_TEXT_TEMPLATE_VERSION` bumped past `founder-text-v0.1.0` | _<runtime constant value>_ | ☐ |
| C3 | Recent `fomo.send.attempted` rows carry the bumped template_version | _<query output + sample row>_ | ☐ |
| C4 | Body length within target 220–280 / hard cap 320 / absolute 340 | _<median content_chars + N over hard>_ | ☐ |
| C5 | NO arbitrary ellipsis (`…`) — sentence-boundary truncation only | _<regression test name + operator visual ref>_ | ☐ |
| C6 | Schema-violation fallback path fires + writes `fomo.alert.drafter_schema_failed` | _<§6 Test 2 audit count + sample row>_ | ☐ |
| C7 | Cross-tenant isolation — only founder touched | _<§8 diff + non-founder send count>_ | ☐ |
| C8 | `ranker.reason` substituted into rendered body (input wiring) | _<runtime unit test name + operator visual ref>_ | ☐ |
| C9 | Zero email-content leakage in audit detail | _<scanned N rows / 0 hits>_ | ☐ |
| C10 | Manual taste check — real iMessage passed founder eye-test | _<paste exact received text + observation>_ | ☐ |
| C11 | Founder regression — recent founder-targeted send used bumped template | _<query output for actor_user_id=founder>_ | ☐ |
| C12 | All prior smoke-evidence scripts still PASS (v0.5.5 may legitimately FAIL per PR #43; operator confirms identical shape) | _<§4–§9 outputs + confirmation>_ | ☐ |

## 4. `smoke-evidence:v0.5.1` output (substrate) — **VERDICT: _<…>_**

```
_<paste full output>_
```

## 5. `smoke-evidence:v0.5.2` output (with `FOMO_V0_5_2_WINDOW_HOURS=168` if needed) — **VERDICT: _<…>_**

```
_<paste full output>_
```

## 6. `smoke-evidence:v0.5.3` output (hardening still wired) — **VERDICT: _<…>_**

```
_<paste full output>_
```

## 7. `smoke-evidence:v0.5.4` output (with `FOMO_V0_5_4_WINDOW_HOURS=168` if needed) — **VERDICT: _<…>_**

```
_<paste full output>_
```

> Operator note re known F2 window-slide false positive (recorded in PR #43 SMOKE_REPORT §7): if the v0.5.4 script reports the same window-slide FAIL shape that PR #43 documented, that is NOT a v0.5.6-caused regression. Confirm by checking that the failing rows are not from the v0.5.6 smoke window.

## 8. `smoke-evidence:v0.5.5` output — **VERDICT: _<…>_**

```
_<paste full output>_
```

> Operator note re known external-blocker FAIL (recorded in PR #43 SMOKE_REPORT): if the v0.5.5 script reports the same C3/C8/C11 blocked-external FAIL shape that PR #43 documented, that is NOT a v0.5.6-caused regression. The blocker is the SendBlue free/sandbox tier OPTED_OUT behavior, scoped to F1 in a separate future 6Q gate.

## 9. `smoke-evidence:v0.5.6` output (tone + length proof) — **VERDICT: _<…>_**

```
_<paste full output>_
```

## 10. Operator-confirmed visual + iMessage checks

| Check | Confirmed? | Notes |
|---|---|---|
| Test 1: Mock-SendBlue regression produced ≥1 `fomo.send.attempted` row on bumped template_version | ☐ | _<smoke-start timestamp + sample row>_ |
| Test 1: `content_chars` was within 220–320 | ☐ | _<median + max>_ |
| Test 2: Schema-violation produced ≥1 `fomo.alert.drafter_schema_failed` row | ☐ | _<sample row>_ |
| Test 2: Deterministic fallback was substituted (rendered output uses fallback string, not LLM reason) | ☐ | _<verification approach>_ |
| Test 2: Zero retry (only one `fomo.send.attempted` per alert_id post-violation) | ☐ | _<query output>_ |
| Test 3: Founder iPhone received iMessage | ☐ / N/A if Test 3 skipped | _<arrival time>_ |
| Test 3: No `FOMO · IMPORTANT (0.92)` header in received text | ☐ / N/A | (the most "robotic" thing Friend B reacted to) |
| Test 3: Sentence-shaped — not newline-separated raw fields | ☐ / N/A | |
| Test 3: No arbitrary `…` ellipsis | ☐ / N/A | (sentence-boundary truncation only) |
| Test 3: Body contains ranker's "why this matters" prose | ☐ / N/A | (not the email body snippet) |
| Test 3: Length feels right on lock screen | ☐ / N/A | |
| Test 3: Felt friendly, not robotic | ☐ / N/A | |
| Test 4: `stop_active` baseline-vs-post diff is empty | ☐ | (v0.5.6 must NOT touch stop_active) |
| Test 4: Only `actor_user_id='founder'` in `fomo.send.attempted` for smoke window | ☐ | |

**Test 3 exact received iMessage text (paste verbatim, redact PII if synthetic email had any):**

```
_<paste here>_
```

## 11. Founder observations

| Observation | Note |
|---|---|
| Did the new shape feel like a "helpful iMessage nudge" or still bot-ish? | _<…>_ |
| Did dropping "FOMO · IMPORTANT (0.92)" change the feel as expected? | _<…>_ |
| Did the ranker.reason prose actually explain "why this matters"? | _<…>_ |
| Was the length right? Too short? Too long? | _<…>_ |
| Did Test 2's deterministic fallback feel acceptable, or does the fallback string need a rewrite? | _<…>_ |
| Anything in audit_log that surprised you? | _<…>_ |
| What would you want different before either: (a) shipping further drafter polish, or (b) starting Personalized Importance Learning substrate (C1)? | _<…>_ |

## 12. Verdict

☐ **PASS** — all 12 criteria green; §8 cross-tenant diff empty; all 6 evidence scripts as expected (v0.5.5 may legitimately FAIL per PR #43; operator confirmed identical shape); operator visual + iMessage checks confirmed (or C10 N/A if Test 3 skipped). **Next phase runs its own 6-question gate.**

☐ **FAIL** — list below.

☐ **PENDING** — runtime commit not yet on branch; re-run after runtime lands.

Failures / followups:

- _…_

## 13. Sign-off

- Founder signature: Galiette Mita
- Date: _<YYYY-MM-DD>_
- No friend consent needed this phase (founder-only smoke)

## 14. Aftercare confirmation

- [ ] If Test 2 used a temporarily lowered schema cap, env var was unset
- [ ] If Tests 1/2 used `FOMO_OUTBOUND_USE_MOCK_SENDBLUE=true`, it was unset
- [ ] If the founder re-flagged themselves OPTED_OUT during the smoke, a real START was sent
- [ ] No friend deletion ops (no friend involved)
- [ ] v0.5.5 STOP enforcement still functional (re-ran `smoke-evidence:v0.5.5` post-smoke; same FAIL shape as PR #43, no new regression)

## 15. What v0.5.6 PASS does NOT promise

v0.5.6 PASS unlocks the next 6-question gate. It explicitly does NOT auto-unlock:

- **F1 SendBlue tier fix** — its own future-phase candidate
- **Personalized Importance Learning substrate** — separate phase per [docs/personalized-importance-learning.md](personalized-importance-learning.md)
- **Friend C onboarding** — three-friend cap; Friend C is OPTIONAL
- **Auto-send** — its own gate per FOMO_PLAN v0.8
- **Reversal of 3E.1 no-LLM-body-generation directive** — v0.5.6 PRESERVES 3E.1 via the hybrid scope
- **Per-user tone customization** — PIL-adjacent, future
- **Ranker rewrite** — only the `reason` field's prompt + schema changes in v0.5.6
- **Google OAuth verification submission (B3)** — multi-week external
- **A new email provider** — Gmail remains only active provider per [FOMO_DESIGN.md §6.2](../FOMO_DESIGN.md)
- **A new model provider** — OpenAI-first per [FOMO_DESIGN.md §18](../FOMO_DESIGN.md)
- **Dashboard / web UI**
- **Calendar / Drafting / MCP / browser automation** — L2+ surfaces

The next phase is decided AT THE NEXT 6-question gate.
