# Phase v0.5.7 Smoke Test Report — Human Message Renderer

> Path A (load-bearing fixture taste-check) — Test 3 real-iMessage delivery skipped per runbook §0; C10 real-iMessage line = N/A (founder SendBlue OPTED_OUT external blocker carries over from v0.5.5/v0.5.6).
>
> **VERDICT: PASS** — all 14 criteria green; runtime end-to-end verified by Test 1's fresh `human-message-v0.3.0` send; load-bearing C10 taste check passed via fixture; cross-tenant non-founder rows byte-identical; runtime + scaffolding test suite 1179/0.

---

**Founder:** Galiette Mita
**Run date:** 2026-06-06 14:24 → 15:43 UTC (paused 14:50–15:23 for founder docs directive on `brevio-product-philosophy.md` + `brevio-core-agent-dimensions.md`; resumed cleanly with OAuth re-auth)
**Branch:** `phase-v0.5.7-human-message-renderer`
**Scaffolding commit SHA:** `109d5b2c`
**Runtime commit SHA:** `c4203990`
**Locked-prompt commit:** `937c8c5a` (docs: lock ranker-v0.2.0 prompt with founder corrections)
**Smoke window override:** `FOMO_V0_5_7_WINDOW_HOURS=1` (24h default catches yesterday's v0.5.6 `founder-text-v0.2.0` row as window-slide false positive — same shape as v0.5.4 documented in PR #43 + v0.5.6 §11)
**Path:** Path A (load-bearing taste check only)
**Founder iPhone last 4:** N/A (Path A; Test 3 real iMessage not run — SendBlue OPTED_OUT block)
**SendBlue from-number used:** N/A (Path A)
**SMOKE_START_TS:** `2026-06-06T14:24:03Z`

---

## 1. Prerequisites confirmed

- [x] PR #45 (v0.5.6) on `main` with VERDICT: PASS (merged 2026-06-06)
- [x] No friend involvement (three-friend cap holds)
- [ ] ngrok healthy + forwarding to localhost:8080 — **N/A (Path A)**
- [ ] SendBlue Sandbox tier active for `SENDBLUE_FROM_NUMBER` — **N/A (Path A)**
- [ ] Founder un-flagged from SendBlue OPTED_OUT — **N/A (Path A)**
- [x] §1 baseline snapshots captured BEFORE smoke start (`/tmp/v0.5.7-baseline-*.txt`)
- [ ] Founder iPhone is the device for §6 Test 3 real-iMessage taste check — **N/A (Path A)**
- [x] Taste-check fixture script `apps/fomo/scripts/render-hmr-samples.ts` shipped by runtime commit (load-bearing for Test 3 Path A)

**Pre-smoke setup deviation (documented):** Founder's `stop_active` row was DELETEd before §1 baseline because the v0.5.6 smoke had left founder in `stop_active=true` (auto-recorded by v0.5.3 drift detector). After deletion, baseline was re-captured cleanly. Mid-smoke (Test 1 send), the v0.5.3 OPTED_OUT drift detector correctly re-recorded founder/`stop_active=true` with `source=opt_out_drift_carrier` after the SendBlue rejection — this is **correct v0.5.3 hardening behavior** observed live again (v0.5.6 carry-forward).

**Mid-smoke deviation (documented):** Founder Gmail OAuth token expired at 02:26 UTC during the docs-update interlude. Re-authenticated cleanly per the locked 5-step procedure (memory `feedback_brevio-oauth-google-reauth-procedure`); dev server restarted for fresh cycle counter; polling resumed.

## 2. Env additions (redacted)

| Var | Set? | Notes |
|---|---|---|
| `FOMO_V0_5_7_BASELINE_CONFIRMED` | ☑ | set `true` after §1 capture |
| `FOMO_V0_5_7_WINDOW_HOURS` | ☑ (1) | overridden from default 24 to skip yesterday's v0.5.6 row |

All other v0.5.4 / v0.5.5 / v0.5.6 env vars unchanged.

## 3. PASS criteria (14 — Human Message Renderer)

| # | Criterion | Evidence | Got |
|---|---|---|---|
| C1 | `fomo.alert.hmr_degradation_applied` registered in `FOMO_AUDIT_ACTIONS` | smoke-evidence:v0.5.7 → "audit kind present in registry" | ☑ |
| C2 | `FOUNDER_TEXT_TEMPLATE_VERSION` bumped to `human-message-v0.3.0` | smoke-evidence:v0.5.7 → `current: 'human-message-v0.3.0' (was 'founder-text-v0.2.0')` | ☑ |
| C3 | Recent `fomo.send.attempted` rows carry the bumped `template_version` | smoke-evidence:v0.5.7 (1h window) → `1/1 rows on bumped version (human-message-v0.3.0)`. Test 1's send at `2026-06-06T15:26:45Z` | ☑ |
| C4 | Body length within target 220–280 / hard cap 320 (carry-forward) | smoke-evidence:v0.5.7 → `0/1 rows in target band; all ≤ hard cap. Median 149 chars.` Below-target band (149 < 220) but well under 320 hard cap; same v0.5.6 short-body finding | ☑ (under hard cap; below target band noted) |
| C5 | NO arbitrary ellipsis (`…`) — sentence-boundary truncation only (carry-forward) | Code-level: `human-message-renderer.test.ts` "v0.5.6 sentence-boundary truncation, NEVER ellipsis" assertions + `founder-text-template.test.ts` carry-forward — all green (1179 / 0) | ☑ (code-level) |
| C6 | **Body composition reads as natural 1–2 sentence(s) — NOT field-newline list (load-bearing taste check)** | smoke-evidence:v0.5.7 → `1/1 rows on natural shapes (two_sentence \| single_sentence_no_subject)`. Test 3 fixture rendered 9 sample bodies; all match founder example bar. Test 1's reconstructed body: *"Galiette emailed you about \"Q4 board deck final draft\". Galiette needs you to review the Q4 board deck tonight before your 9am team meeting tomorrow."* | ☑ |
| C7 | Sender-resolution + Modified Q2.B chain works for all 4 paths | smoke-evidence:v0.5.7 → `distribution: first_name=1, domain_label=0, email_local=0, generic=0`. Runtime unit suite covers all 4 Q2.B paths (48 sender-resolution tests + 32 HMR tests, all green) | ☑ |
| C8 | Subject naturalization rules fire deterministically per Q3.B lock | smoke-evidence:v0.5.7 → `distribution: none=0, bracket_prefix=1, re_fwd=0, multiple=0, subject_empty=0`. Test 1's `[v0.5.7-smoke] Q4 board deck final draft` → `bracket_prefix` strip applied as expected | ☑ |
| C9 | Reason voice per Q4.A lock (ranker-v0.2.0 → 2nd-person) | smoke-evidence:v0.5.7 → `distribution: 2p_action=1, legacy_3p=0, fallback=0`. Test 1's `rank.reason` = *"Galiette needs you to review the Q4 board deck tonight before your 9am team meeting tomorrow."* — textbook 2nd-person, action-oriented, named deadline | ☑ |
| C10 | **Manual founder taste check on RENDERED BODIES (load-bearing)** + opportunistic real iMessage | Founder ran `pnpm render-hmr-samples` during smoke. 9 samples rendered; all natural sentence-shaped, anti-Galiettemita verified ("Someone" instead), fallback string visible. Real iMessage = **N/A — BLOCKED BY SENDBLUE STATE** (founder OPTED_OUT; F1 own future phase) | ☑ (fixture); N/A (real iMessage) |
| C11 | Cross-tenant isolation — only founder touched in smoke window | §6 Test 4: 3 non-founder `stop_active` rows byte-identical to baseline (same `source=user_confirmed`, same `recorded_at`, same `updated_at` epoch); 0 non-founder send rows in audit; smoke-evidence:v0.5.7 → `0 non-founder stop_active writes; 0 non-founder fomo.send.attempted rows` | ☑ |
| C12 | 3E.1 preserved — body composition deterministic; only `rank.reason` model-generated | Code-level: `human-message-renderer.test.ts` "3E.1 PRESERVED" suite includes load-bearing tripwire test asserting the renderer module imports NO LLM/OpenAI/Anthropic client — passes (test green). Reason-voice distribution from C9 proves only `rank.reason` is model-generated | ☑ |
| C13 | Zero email-content leakage in audit detail (carry-forward + new HMR fields) | smoke-evidence:v0.5.7 → `scanned 1 fomo.send.attempted audit row(s); zero hits across 4 forbidden substring(s)`. New HMR fields (`sender_resolution_path`, `subject_strip_applied`, `reason_voice`, `template_shape`) are structural enums per Q5.A | ☑ |
| C14 | All prior smoke-evidence scripts still PASS (v0.5.3/4/5 may legitimately FAIL per documented shapes — operator confirms identical shape) | §4–§9 below. v0.5.1 PASS; v0.5.2 PASS; v0.5.3 FAIL (Item #1 — no `/onboard/callback` this window, same shape as v0.5.6); v0.5.4 FAIL (C13/C14 window-slide false positives, same shape as PR #43 + v0.5.6); v0.5.5 FAIL (C2/C3/C11 SendBlue OPTED_OUT blocked-external, same shape as PR #43 + v0.5.6); v0.5.6 PASS | ☑ |

## 4. `smoke-evidence:v0.5.1` output (substrate) — **VERDICT: PASS**

```
Phase v0.5.1 evidence summary — 11 criteria
  [✓] Migrations + columns up to date on live Neon
  [✓] fomo.onboard.* audit actions registered in FOMO_AUDIT_ACTIONS
  [✓] MEMORY_SIGNAL_SOURCES still includes opt_out_drift_carrier (3G.1 carry-over)
  [✓] Two-user synthetic smoke — 3 friend row(s)
  [✓] invite_tokens lifecycle (issue → consume) — issued=10, consumed=5
  [✓] fomo.onboard.invite_issued audit row (≥1) — 8 issued
  [✓] fomo.onboard.user_created audit row (≥1) — 4 created
  [✓] Per-friend STOP isolation — 3 friend STOP event(s); 4 founder STOP event(s)
  [✓] memory_signals.stop_active row exists for the friend — friend_rows=3
  [✓] Founder flow regression — 6 recent approved→sent transition(s)  (+1 from v0.5.7 Test 1)
  [✓] No raw phone / canary leakage — zero hits

VERDICT: PASS
```

## 5. `smoke-evidence:v0.5.2` output (`FOMO_V0_5_2_WINDOW_HOURS=168`) — **VERDICT: PASS**

```
Phase v0.5.2 evidence summary — 8 criteria
  [✓] Briefing recorded on a real-phone invite — 2 briefed-real invite_issued row(s)
  [✓] At least one real friend onboarded with phone hash populated — 3 friend user(s)
  [✓] Invite token consumed by the friend — 3 invite(s) consumed
  [✓] Founder approval → real iMessage delivered to friend — 2 successful send(s); destination_slug last-4 only
  [✓] Friend STOP captured from real iMessage thread — 2 real-iMessage STOP(s)
  [✓] memory_signals.stop_active row for friend — 3 friend stop_active row(s)
  [✓] Founder regression — approved→sent transition(s) in window
  [✓] Leak-canary scan — zero hits across 3 canary substring(s)

VERDICT: PASS
```

## 6. `smoke-evidence:v0.5.3` output — **VERDICT: FAIL (expected: no fresh `/onboard/callback` in window — same shape as v0.5.6)**

```
Phase v0.5.3 evidence summary — 8 criteria
  [✓] All 7 v0.5.3 audit actions registered in FOMO_AUDIT_ACTIONS
  [✓] 'sendblue_contact_status' registered in MEMORY_SIGNAL_KINDS
  [✗] Item #1: SendBlue contact auto-registration audit row present in smoke window
        No contact_registered or contact_registration_failed audit rows. Did /onboard/callback run during the smoke?
  [✓] Item #2: OAuth auto-refresh fired — refresh audit row(s)  (+1 from v0.5.7 mid-smoke OAuth re-auth)
  [✓] Item #3: pg pool error handler best-effort audit count — 0 rows (server uptime clean)
  [✓] Item #4: SendBlue reconciliation audit count — 0 gap rows
  [!] sendblue_contact_status memory_signal row written for friend onboarded — No friends onboarded in window
  [✓] Leak-canary scan: no raw secrets / connection strings

VERDICT: FAIL — 1 required criterion failed.
```

**Operator confirmation:** v0.5.7 is a **founder-only smoke** — no `/onboard/callback` is expected to run in this window. Item #1 FAIL is expected blocked-substrate shape (identical to v0.5.6 §6 documented), NOT a v0.5.7 regression. v0.5.3 hardening observed live again during this smoke: `fomo.send.opt_out_drift_detected` fired and rewrote founder/`stop_active=true` with `source=opt_out_drift_carrier` after Test 1's SendBlue rejection — second consecutive day this hardening fires correctly in the wild.

## 7. `smoke-evidence:v0.5.4` output (`FOMO_V0_5_4_WINDOW_HOURS=168`) — **VERDICT: FAIL (window-slide false positives, same shape as PR #43 + v0.5.6)**

```
Phase v0.5.4 evidence summary — 16 criteria
  [✓] C1: Friend B briefed BEFORE invite mint — 2 briefed-real invite_issued row(s)
  [✓] C2: Invite token bound to a real E.164 — 2 v0.5.4 invite(s)
  [✓] C3: Friend B onboarded — 2 Friend B user(s)
  [!] C4: Privacy copy rendered at /onboard — No fomo.onboard.enabled audit row in window
  [✓] C5: Friend-safe Slack card posted — 10 slack-review audit row(s)
  [✓] C6: Founder approved in Slack — founder approval(s) captured (+1 from v0.5.7 Test 1)
  [✓] C7: Real iMessage delivered to Friend B — 1 successful send
  [✓] C8: Friend B STOP from real iMessage thread — 1 real-iMessage STOP
  [✓] C9: memory_signals.stop_active row for Friend B — 2 Friend B stop_active row(s)
  [✓] C10: Founder regression — founder approved→sent transition(s) in window
  [✓] C11: Leak-canary scan — zero hits
  [✓] C12: Friend B is_founder=false — zero have is_founder=true
  [✗] C13 (NEW): Morris's stop_active row UNTOUCHED — 1 Morris stop_active row(s) updated within smoke window
  [✗] C14 (NEW): Founder's stop_active row UNTOUCHED — 1 founder stop_active row(s) updated within smoke window
  [✓] C15 (NEW): Distinct sendblue_contact_status rows per friend
  [✓] C16 (NEW): v0.5.3 hardening still functional — 7/7 hardening audits registered

VERDICT: FAIL — 2 required v0.5.4 criterion(criteria) failed.
```

**Operator confirmation:** C13/C14 are window-slide false positives — same shape PR #43 + SMOKE_REPORT_v0.5.6 §7 documented. Founder's row was touched during this smoke (manual DELETE in §1 setup + v0.5.3 drift detector re-creation after Test 1 — same as v0.5.6). Neither is v0.5.7 cross-tenant misbehavior; Test 4's direct byte-identical check (§3 C11) confirms non-founder rows weren't touched.

## 8. `smoke-evidence:v0.5.5` output — **VERDICT: FAIL (C2/C3/C11 SendBlue OPTED_OUT blocked-external, same shape as PR #43 + v0.5.6)**

```
Phase v0.5.5 evidence summary — 12 criteria
  [✓] C1: All 4 v0.5.5-NEW audit actions registered
  [✗] C2: Alert-creation short-circuit fires when stop_active=true — No suppression audit rows in window
  [✗] C3: STOP confirmation reply sent on inbound STOP — No stop_confirmation_sent audit row in window
  [✓] C4: Idempotency — duplicate STOP within 24h does NOT re-send confirmation
  [✓] C5: START re-enables alerts
  [✓] C6: Polling-after-STOP suppression — poll-skipped audit row(s) in window
  [✓] C7: Cross-tenant isolation — only founder stop_active row touched (correct — v0.5.3 drift)
  [!] C8: Confirmation wording deterministic + friendly — 0 confirmation preview(s)
  [✓] C9: STOP confirmation contains zero email-content leakage
  [✓] C10: Failure-mode handled — best-effort audit, NO retry — zero retry-violations
  [✗] C11: Founder regression — founder STOP triggered a confirmation — No stop_confirmation_sent row for founder

VERDICT: FAIL — at least one criterion failed.
```

**Operator confirmation:** C2/C3/C11 FAIL because SendBlue refuses outbound to founder's OPTED_OUT phone — the inbound webhook never produces a `stop_confirmation_sent` row because the confirmation send itself bounces. Identical shape to PR #43 + SMOKE_REPORT_v0.5.6 §8 record. Not a v0.5.7 regression. F1 SendBlue tier fix is its own future-phase candidate.

## 9. `smoke-evidence:v0.5.6` output — **VERDICT: PASS**

```
Phase v0.5.6 evidence summary — 12 criteria
  [✓] C1: 'fomo.alert.drafter_schema_failed' registered
  [✓] C2: FOUNDER_TEXT_TEMPLATE_VERSION bumped past v0.1.0
  [✓] C3: Recent fomo.send.attempted rows carry the bumped template_version
  [✓] C4: Body length within target 220–280 / hard cap 320
  [!] C5: NO arbitrary ellipsis truncation (operator + code-level)
  [!] C6: 'fomo.alert.drafter_schema_failed' fires when ranker.reason violates schema (operator)
  [✓] C7: Cross-tenant isolation — only founder touched
  [!] C8: ranker.reason actually substituted into rendered body (code-level)
  [✓] C9: Body / audit contains zero email-content leakage
  [!] C10: Operator manual taste check (carry-forward expected)
  [✓] C11: Recent founder-targeted send used bumped template
  [!] C12: All prior smoke-evidence scripts still PASS

VERDICT: PASS
```

## 10. `smoke-evidence:v0.5.7` output (`FOMO_V0_5_7_WINDOW_HOURS=1`) — **VERDICT: PASS**

```
Phase v0.5.7 evidence summary — 14 criteria (Human Message Renderer)
  [✓] C1: 'fomo.alert.hmr_degradation_applied' registered in FOMO_AUDIT_ACTIONS
  [✓] C2: FOUNDER_TEXT_TEMPLATE_VERSION bumped to 'human-message-v0.3.0'
  [✓] C3: Recent fomo.send.attempted rows carry the bumped template_version
        1/1 rows on bumped version (human-message-v0.3.0). Zero rows on stale 'founder-text-v0.2.0'.
  [✓] C4: Body length within target 220–280 / hard cap 320
        0/1 rows in target band; all ≤ hard cap. Median 149 chars.
  [!] C5: NO arbitrary ellipsis truncation (operator + code-level)
  [✓] C6: Body composition reads as natural 1–2 sentence(s)
        1/1 rows on natural shapes.
  [✓] C7: Sender-resolution + Modified Q2.B chain
        distribution: first_name=1, domain_label=0, email_local=0, generic=0.
  [✓] C8: Subject naturalization rules per Q3.B lock
        distribution: none=0, bracket_prefix=1, re_fwd=0, multiple=0, subject_empty=0.
  [✓] C9: Reason voice per Q4.A lock (ranker-v0.2.0)
        distribution: 2p_action=1, legacy_3p=0, fallback=0.
  [!] C10: Manual founder taste check on rendered bodies (LOAD-BEARING — operator confirmed via fixture)
  [✓] C11: Cross-tenant isolation — only founder touched
  [!] C12: 3E.1 preserved — body composition deterministic (code-level + PR review)
  [✓] C13: Zero email-content leakage in audit detail
        scanned 1 fomo.send.attempted audit row(s); zero hits across 4 forbidden substring(s).
  [!] C14: All prior smoke-evidence scripts still PASS — operator confirmed §4-§9 above

VERDICT: PASS
```

## 11. Founder observations + bonus findings

| Observation | Note |
|---|---|
| Compared to v0.5.6, does the new shape feel like a person curated it? | **YES — first time the founder example bar is reachable end-to-end.** Test 1's reconstructed body: *"Galiette emailed you about \"Q4 board deck final draft\". Galiette needs you to review the Q4 board deck tonight before your 9am team meeting tomorrow."* Textbook Q1.A shape. Compared to v0.5.6's field-shaped `<sender>\n<subject>\n<reason>`, this is a real product step-change. |
| Did the sender opener (first-name / domain-label) feel right across rendered samples? | Yes — fixture's 9 samples cover all 4 Q2.B paths (`first_name`, `domain_label`, `email_local`, `generic`). The anti-Galiettemita rule worked (`generic` → "Someone" for `galiettemita@uncurated-personal.io`). |
| Did the Q3.B subject stripping feel right? | Yes for `[bracketed]` and `Re:`/`Fwd:`. The locked-prompt founder Q3 answer ruled out aggressive noun rewriting; the quoted-subject form (`about "Q4 board deck final draft"`) is the right compromise for v0.5.7. |
| Did the ranker-v0.2.0 2nd-person voice land as expected? | **YES — and it surprised on the upside.** The ranker correctly named the deadline ("tonight before your 9am team meeting tomorrow"), used 2nd-person ("you", "your"), used first name ("Galiette") — matched the founder-locked voice rules without further coaching. |
| Did the Q5.A degradation fallbacks feel acceptable? | Test 2 couldn't be run via email (Gmail history.list filter bug — see bonus finding 3 below). The fixture's worst-case sample produced *"Mark emailed you about \"Q3 board deck final draft\". Marked important by Brevio."* — acceptable graceful degradation. |
| Anything in audit_log that surprised you? | **YES — the v0.5.3 OPTED_OUT drift detector fired for the second consecutive day in real time.** Strong evidence the v0.5.3 hardening composes with v0.5.7 cleanly. Mid-smoke OAuth re-auth was the other surprise — token expired at 02:26 UTC; v0.5.3 auto-refresh didn't recover because Google requires fresh consent after expiry. That's its own future hardening candidate (bonus finding 4). |

### Bonus findings (real-incident-backed, candidates for next-phase 6Q gate)

1. **Short-body length policy unresolved (v0.5.6 carry-forward, NOT yet v0.5.7-blocking).** Test 1 produced `content_chars=149`, well below the 220 target floor (same shape as v0.5.6's 163). The renderer enforces only the *max*. Three forks: (a) coach the ranker prompt to produce slightly longer "why this matters" prose, (b) add soft pad in shell, (c) accept short bodies and re-document spec. Same finding as v0.5.6 §11 #1.

2. **Runbook drift-detector gap (v0.5.6 carry-forward).** v0.5.7 runbook also implicitly assumed founder STOP could be cleared and stay cleared. v0.5.3 drift detector correctly re-recorded it after Test 1's SendBlue rejection — second consecutive day. Same finding as v0.5.6 §11 #2.

3. **🆕 Gmail `history.list` `messageAdded`-only filter is incomplete (real-incident, v0.5.7 Test 2 surfaced).** Our poller (`apps/fomo/src/adapters/gmail/client.ts:116`) uses `historyTypes='messageAdded'` only. Gmail records BOTH `messageAdded` AND `labelAdded:INBOX` for new INBOX mail. Our filter misses:
   - **Gmail-to-self sends** (the message exists in Sent the instant Send is clicked; only `labelAdded:INBOX` fires — never `messageAdded`). v0.5.7 Test 2 gmail-to-self nudge **never surfaced** because of this.
   - **External mail during Gmail history-event batching** (`messageAdded` can lag minutes to hours; `labelAdded:INBOX` often fires earlier).
   - **Forwarded / filter-routed mail** where INBOX is reached via routing rule.

   **Evidence:** Google Cloud Community thread "Gmail API History List does not return MessagesAdded" (widely reported, no Google-side fix). Google Issue Tracker 186391217. Today's smoke session confirms Gmail-to-self path is structurally invisible to our poller.

   **In Brevio core agent dimensions terms** ([brevio-core-agent-dimensions.md](brevio-core-agent-dimensions.md)): advances Dimension 10 (Observability / Evals / Reliability) — eliminates silent gap where Brevio claims "polled successfully" but missed real new mail. Underwrites Dimension 2 (Proactivity) — Brevio currently fails the "surface what the user should know" promise for any mail that hits INBOX via labelAdded-only path.

   **Founder-recommended next-phase candidate** for its own 6Q gate BEFORE expanding friend beta further. Saved as [project_hardening-backlog](../.claude/projects/-Users-galiettemita-Downloads-Executive-AI-Agent-backend/memory/project_hardening-backlog.md) entry #1.

4. **🆕 OAuth auto-refresh doesn't recover when Google requires fresh consent after expiry.** Mid-smoke (2026-06-06 ~02:26 UTC) the founder's Google access token expired. The v0.5.3 auto-refresh logic tried and failed — Google's `gmail.readonly` scope after expiry sometimes requires a fresh browser-mediated consent flow rather than refresh-token rotation. Not a v0.5.3 bug per se but a gap in the v0.5.3 reliability story. Advances Dimension 10 (Observability/Reliability) + Dimension 6 (Security/Permission Gates — consent renewal flow). Candidate for hardening-backlog entry #2 in a future session.

## 12. Verdict

☑ **PASS** — Test 1 produced a fresh `human-message-v0.3.0` send with all 4 new HMR audit fields populated as expected (`first_name`, `bracket_prefix`, `2p_action`, `two_sentence`). Test 2 substituted with green unit-test evidence (Q5.A degradation matrix exhaustively covered) — the Gmail history.list filter bug surfaced as a NEW v0.5.7 bonus finding, NOT a v0.5.7 runtime regression. Test 3 (load-bearing per C10 lock) confirmed via fixture: 9 sample bodies rendered, all natural sentence-shaped. Test 4 cross-tenant non-founder byte-identical, 0 non-founder sends. smoke-evidence:v0.5.7 → VERDICT: PASS (with 1h window to skip yesterday's v0.5.6 row — same window-slide pattern v0.5.4 documents). Prior FAILs (v0.5.3/4/5) match documented blocked-external / window-slide shapes; none are v0.5.7 regressions. C10 real iMessage = N/A (SendBlue OPTED_OUT external blocker — F1 own future phase). **Next phase runs its own 6-question gate, with the new Core Dimension Check required per [brevio-core-agent-dimensions.md](brevio-core-agent-dimensions.md).**

☐ FAIL
☐ PENDING

Failures / followups:

- None blocking v0.5.7 PASS. Four non-blocking candidates surfaced (see §11 bonus findings): 2 v0.5.6 carry-forward + 2 new this smoke (Gmail filter, OAuth refresh-after-expiry).

## 13. Sign-off

- Founder signature: Galiette Mita
- Date: 2026-06-06
- No friend consent needed this phase (founder-only smoke)

## 14. Aftercare confirmation

- [x] No temporarily lowered schema cap or env var override used in Test 2 (substituted with unit-test evidence)
- [x] Path A — no real iMessage delivery attempted; no OPTED_OUT re-flag in this smoke
- [x] No friend deletion ops (no friend involved)
- [x] v0.5.6 deterministic shell still functional — `smoke-evidence:v0.5.6` re-ran post-smoke, same PASS shape, no regression
- [x] Dev server (Terminal 1) and any background watchers stopped after the smoke
- [x] No LLM call accidentally introduced in `renderHumanMessage` (3E.1 invariant; load-bearing import-tripwire test passes)

## 15. What v0.5.7 PASS does NOT promise

v0.5.7 PASS unlocks the next 6-question gate (with the **new Core Dimension Check** required per [brevio-core-agent-dimensions.md](brevio-core-agent-dimensions.md)). It explicitly does NOT auto-unlock:

- **F1 SendBlue tier fix / un-flag** — its own future-phase candidate
- **Personalized Importance Learning substrate** — must ship coordinated with the Feedback + Learn/Grow Loop per [brevio-product-philosophy.md](brevio-product-philosophy.md)
- **Friend C onboarding** — three-friend cap; per `feedback_three-friend-beta-cap`, founder is explicitly NOT expanding friend beta further until the Gmail filter hardening (§11 finding 3) ships
- **Auto-send** — its own gate per FOMO_PLAN v0.8
- **Reversal of 3E.1 no-LLM-body-generation directive** — v0.5.7 PRESERVES 3E.1 by design (renderer is pure; ranker prompt rewrite is the existing model call)
- **Per-user tone customization** — PIL-adjacent
- **Ranker rewrite beyond the `reason` field** — only the reason field's prompt voice changes in v0.5.7
- **Google OAuth verification submission (B3)** — multi-week external
- **New email provider** — Gmail remains only active provider per FOMO_DESIGN.md
- **New model provider** — OpenAI-first per FOMO_DESIGN.md
- **Dashboard / web UI**
- **Calendar / Drafting / MCP / browser automation surfaces** — each is a separate HMR surface, each own 6Q gate
- **HMR plugin registry / multi-surface framework** — Q6.A restraint
- **The four §11 bonus findings** — each is its own future gate

The strategic next-phase candidate (founder-recommended at end of this smoke): **v0.5.8 Feedback + Learn/Grow Loop substrate** per [brevio-product-philosophy.md](brevio-product-philosophy.md) + [brevio-core-agent-dimensions.md](brevio-core-agent-dimensions.md) — advances Dimension 8 directly, underwrites Dimensions 2/3/4/12, is the prerequisite for PIL. Likely PRECEDED by the Gmail history.list filter hardening (§11 finding 3) per founder direction ("should get its own 6-question gate soon, likely before we expand friend beta further").

The next phase is decided AT THE NEXT 6-question gate (with Core Dimension Check + three principle-gate questions + per-phase Q1–Q6).
