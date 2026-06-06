# Phase v0.5.7 Smoke Test — Human Message Renderer

> Founder-only smoke. **First surface of the [Human Message Renderer](../README.md) product layer** per the founder-locked principle ([memory: brevio-human-message-renderer-principle](../.claude/projects/-Users-galiettemita-Downloads-Executive-AI-Agent-backend/memory/feedback_brevio-human-message-renderer-principle.md)).
>
> No friend involved. Three-friend cap holds.
>
> **C10 correction:** Real iMessage delivery is OPPORTUNISTIC ONLY. Founder taste check on **rendered bodies** (via the taste-check fixture) is load-bearing. If SendBlue OPTED_OUT / tier state still blocks delivery, mark real iMessage as `N/A — BLOCKED BY SENDBLUE STATE`, NOT failure. v0.5.7 is HMR, not SendBlue unblock work.

---

## §0 — Decide Test 3 path

The smoke has 4 tests. Test 3 is the **load-bearing taste check on rendered bodies**. It runs entirely offline against the taste-check fixture script. Real-iMessage delivery is opportunistic.

- **Path A (default; load-bearing only):** run Tests 1, 2, 3 (taste-check fixture), 4. Real iMessage NOT attempted. C10 real-iMessage line = `N/A — BLOCKED BY SENDBLUE STATE` if applicable.
- **Path B (opportunistic real iMessage):** un-flag founder phone in SendBlue dashboard first → run Tests 1, 2, 3 (BOTH taste-check fixture AND real iMessage on iPhone), 4.

Path A is the default per Q5.A C10 correction. The runbook below covers both.

---

## §1 — Baseline snapshot (Terminal 1, run once)

```bash
cd "/Users/galiettemita/Downloads/Executive AI Agent/backend"
set -a; source apps/fomo/.env.3b3.local; set +a

# Sanity-check DATABASE_URL didn't get clobbered by your zshrc (see memory
# stale-database-url-shell-export):
echo "DATABASE_URL tail: ...$(echo "$DATABASE_URL" | tail -c 40)"
# Must end with sslmode=require. If clobbered, re-source.

# Snapshot recent fomo.send.attempted rows (load-bearing for C3/C4 + the
# four new HMR audit fields once runtime lands):
psql "$DATABASE_URL" -P pager=off -c "
SELECT actor_user_id,
       detail->>'template_version' AS template_version,
       (detail->>'content_chars')::int AS content_chars,
       detail->>'sender_resolution_path' AS sender_resolution_path,
       detail->>'subject_strip_applied' AS subject_strip_applied,
       detail->>'reason_voice' AS reason_voice,
       detail->>'template_shape' AS template_shape,
       occurred_at
FROM audit_log
WHERE action = 'fomo.send.attempted'
  AND occurred_at > now() - interval '7 days'
ORDER BY occurred_at DESC
LIMIT 20;
" | tee /tmp/v0.5.7-baseline-send-attempted.txt

# Snapshot stop_active rows (load-bearing for C11 cross-tenant):
psql "$DATABASE_URL" -P pager=off -c "
SELECT user_id, kind, jsonb_pretty(detail) AS detail, source, updated_at
FROM memory_signals
WHERE kind = 'stop_active'
ORDER BY user_id;
" | tee /tmp/v0.5.7-baseline-stop-active.txt

# Note the timestamp NOW — you'll use it later as the smoke-start cutoff:
date -u +"%Y-%m-%dT%H:%M:%SZ"
```

Confirm both snapshot files have content. Copy the timestamp output — paste into queries below as `<SMOKE_START_TS>`.

**Pre-smoke setup note:** If founder is currently in `stop_active = true` from a prior smoke, DELETE the row BEFORE the baseline (same as v0.5.6 runbook). The v0.5.3 OPTED_OUT drift detector will likely re-record it after the first send if SendBlue blocks delivery — that's correct hardening behavior, not a v0.5.7 violation (the C11 diff is for NON-founder rows).

---

## §2 — Add v0.5.7 env vars

Append to `apps/fomo/.env.3b3.local`:

```
FOMO_V0_5_7_BASELINE_CONFIRMED=true
FOMO_V0_5_7_WINDOW_HOURS=24
```

Re-source in Terminal 1: `set -a; source apps/fomo/.env.3b3.local; set +a`.

---

## §3 — Preflight

```bash
pnpm --filter @brevio/fomo run preflight:v0.5.7
```

Expect: `✓ Preflight passed.` with 1–2 WARNs at SCAFFOLDING-time:
- `FOMO_AUDIT_ACTIONS: 'fomo.alert.hmr_degradation_applied' PENDING runtime commit`
- `FOUNDER_TEXT_TEMPLATE_VERSION: still 'founder-text-v0.2.0' PENDING runtime commit`

Zero WARNs after the runtime commit lands.

If any ERROR fires, fix the named env var and re-run before proceeding.

---

## §4 — Boot dev server (Terminal 1)

```bash
pnpm --filter @brevio/fomo run build
pnpm --filter @brevio/fomo dev 2>&1 | tee /tmp/fomo-v0.5.7.log
```

Wait for `fomo.server.listening` on port 8080. Leave running.

---

## §5 — ngrok (Terminal 2, ONLY if Path B Test 3 real iMessage)

Skip if Path A. Otherwise:

```bash
ngrok http --domain=unshivering-interaulic-beatriz.ngrok-free.dev 8080
```

---

## §6 — Tests

### Test 1 — Mock regression (the CI bar)

**Goal:** prove a new v0.5.7 send produces `template_version=human-message-v0.3.0` and the four new structural audit fields populated.

In Terminal 3:

```bash
cd "/Users/galiettemita/Downloads/Executive AI Agent/backend"
set -a; source apps/fomo/.env.3b3.local; set +a

# Inject a synthetic important email — easiest is to email yourself
# (founder Gmail) with a unique subject:
# Subject: "[v0.5.7-smoke] Q4 board deck final draft"
# Body: time-sensitive (counselor / employer / school / contract style)
```

Send that email from any account to your founder Gmail. Wait ~60s for the polling worker (poll interval 10s).

Watch Terminal 1 for:
- `fomo.rank.completed` (ranker classified it)
- `fomo.slack.posted` (Slack card appeared in your founder DM)

**Before approving the Slack card** — note the rank.reason length to inform Test 2's induced violation:

```bash
psql "$DATABASE_URL" -P pager=off -c "
SELECT id, label, round(score::numeric, 3) AS score, length(reason) AS reason_len, reason, created_at
FROM rank_results
WHERE user_id='founder' AND created_at > '<SMOKE_START_TS>'
ORDER BY created_at DESC LIMIT 3;
"
```

Approve the Slack card. Then watch for:
- `fomo.send.attempted`
- `fomo.send.succeeded` (or `fomo.send.failed` if SendBlue blocks — fine, audit is what we verify)

Query the result:

```bash
psql "$DATABASE_URL" -P pager=off -c "
SELECT detail->>'template_version' AS template_version,
       (detail->>'content_chars')::int AS content_chars,
       detail->>'sender_resolution_path' AS sender_resolution_path,
       detail->>'subject_strip_applied' AS subject_strip_applied,
       detail->>'reason_voice' AS reason_voice,
       detail->>'template_shape' AS template_shape,
       occurred_at
FROM audit_log
WHERE action = 'fomo.send.attempted'
  AND occurred_at > '<SMOKE_START_TS>'
ORDER BY occurred_at DESC
LIMIT 5;
"
```

**Pass criteria for Test 1:**
- `template_version = human-message-v0.3.0` ✓
- `content_chars` ≤ 320 (hard cap; 220-target floor is informational) ✓
- `sender_resolution_path` populated (likely `first_name` for iCloud) ✓
- `subject_strip_applied = 'bracket_prefix'` (we used `[v0.5.7-smoke]`) ✓
- `reason_voice` populated (`2p_action` after ranker-v0.2.0 rollout; `legacy_3p` transitional acceptable) ✓
- `template_shape = 'two_sentence'` ✓
- No `fomo.alert.hmr_degradation_applied` audit row from this test ✓

### Test 2 — Q5.A degradation matrix

**Goal:** prove each Q5.A fallback rule fires + audits + degrades gracefully.

There are 4 degradation paths to exercise. The cheapest way is the **taste-check fixture script** (runtime commit ships `apps/fomo/scripts/render-hmr-samples.ts`) which renders synthetic inputs and prints both the rendered text AND the would-be audit detail. The fixture covers all 4 paths in one run.

```bash
pnpm --filter @brevio/fomo run render-hmr-samples
```

Expected output (per Q5.A):

| Synthetic input | sender_resolution_path | subject_strip_applied | reason_voice | template_shape |
|---|---|---|---|---|
| sender_name=`Galiette Mita`, subject=`Q4 review`, reason=2p | `first_name` | `none` | `2p_action` | `two_sentence` |
| sender_name=empty, email=`no-reply@github.com`, subject=`Re: [PR] fix`, reason=2p | `domain_label` | `re_fwd` then `bracket_prefix` → `multiple` | `2p_action` | `two_sentence` |
| sender_name=empty, email=`john.doe@acme.com`, subject=empty, reason=2p | `email_local` | `subject_empty` | `2p_action` | `single_sentence_no_subject` |
| sender_name=empty, email=`galiettemita@icloud.com`, subject=empty, reason=`x`.repeat(250) | `generic` ("Someone") | `subject_empty` | `fallback` | `fallback_string` |

For an **end-to-end Test 2 run** (audit row actually written), induce a real path:

```bash
# Send yourself a SECOND synthetic email with no subject AND a long body.
# Wait for fomo.rank.completed.

# BEFORE approving, mutate the rank.reason to >180 chars to induce
# the 'fallback' path:
psql "$DATABASE_URL" -P pager=off -c "
UPDATE rank_results SET reason = repeat('x', 250)
WHERE id = (SELECT id FROM rank_results WHERE user_id='founder' AND created_at > '<SMOKE_START_TS>' ORDER BY created_at DESC LIMIT 1);
"
```

Approve the Slack card. Watch for `fomo.alert.hmr_degradation_applied` AND `fomo.alert.drafter_schema_failed` (v0.5.6 carry-forward — the reason-schema-fail audit still fires alongside the new HMR-degradation audit).

```bash
psql "$DATABASE_URL" -P pager=off -c "
SELECT action, jsonb_pretty(detail) AS detail, occurred_at
FROM audit_log
WHERE action IN ('fomo.alert.hmr_degradation_applied','fomo.alert.drafter_schema_failed')
  AND occurred_at > '<SMOKE_START_TS>'
ORDER BY occurred_at DESC;
"
```

**Pass criteria for Test 2:**
- ≥1 `fomo.alert.hmr_degradation_applied` row ✓
- Detail captures which fallback fired (e.g. `fallback_applied='reason_schema'`) ✓
- Companion `fomo.alert.drafter_schema_failed` still fires on reason violation (v0.5.6 carry-forward) ✓
- No retry: only one `fomo.send.attempted` per alert_id ✓
- Fallback string substituted; no raw subject/body/header in any new audit field ✓

### Test 3 — Load-bearing taste check on rendered bodies

**Goal (load-bearing per C10 correction):** founder eye-tests rendered bodies against the founder example bar without depending on SendBlue delivery.

```bash
pnpm --filter @brevio/fomo run render-hmr-samples > /tmp/v0.5.7-rendered-samples.txt
cat /tmp/v0.5.7-rendered-samples.txt
```

For each rendered body in the output, founder evaluates:

- [ ] **Reads as a natural 1–2 sentence message**, not as `<sender>\n<subject>\n<reason>` field-newline list
- [ ] **Opens with a person-named or domain-named sender** — not `g***@icloud.com`-style masked email
- [ ] No `FOMO · IMPORTANT (0.92)` telemetry header (v0.5.6 carry-forward)
- [ ] No arbitrary `…` ellipsis (v0.5.6 carry-forward)
- [ ] Subject reads cleanly (no `[v0.5.7-smoke]` prefix, no `Re:` artifacts)
- [ ] `Why_clause` reads as 2nd-person action prose if `reason_voice='2p_action'` (`legacy_3p` transitional acceptable; `fallback` shows `Marked important by Brevio.`)
- [ ] Length feels right for lock-screen reading
- [ ] **Feels like a person curated it.** Compare to founder example: *"Galiette emailed you about the Q3 board deck. It looks time-sensitive — she needs sign-off by tomorrow."*

Paste the **N sample bodies** verbatim into SMOKE_REPORT §10 with founder notes.

**Path B opportunistic real-iMessage check (only if SendBlue allowed delivery during Test 1):**

On founder iPhone, evaluate the same checklist against the actually delivered iMessage. Paste the exact received text into SMOKE_REPORT §10. If SendBlue blocked delivery, mark `N/A — BLOCKED BY SENDBLUE STATE` per C10 correction.

### Test 4 — Cross-tenant isolation

```bash
psql "$DATABASE_URL" -P pager=off -c "
SELECT user_id, kind, jsonb_pretty(detail) AS detail, source, updated_at
FROM memory_signals
WHERE kind = 'stop_active'
ORDER BY user_id;
" | tee /tmp/v0.5.7-post-stop-active.txt

# Non-founder diff — these rows MUST be byte-identical to baseline:
diff <(grep -v "founder " /tmp/v0.5.7-baseline-stop-active.txt) <(grep -v "founder " /tmp/v0.5.7-post-stop-active.txt)
```

**Pass criterion:** the non-founder diff is empty. (Founder row may differ if v0.5.3 drift detector re-recorded it after a SendBlue OPTED_OUT bounce — that's correct cross-phase behavior, not a v0.5.7 violation.)

Also check no non-founder sends:

```bash
psql "$DATABASE_URL" -P pager=off -c "
SELECT actor_user_id, COUNT(*) AS sends
FROM audit_log
WHERE action = 'fomo.send.attempted'
  AND occurred_at > '<SMOKE_START_TS>'
GROUP BY actor_user_id;
"
```

Only `actor_user_id='founder'` should appear.

---

## §7 — Run all 7 evidence scripts

```bash
pnpm --filter @brevio/fomo run smoke-evidence:v0.5.1
FOMO_V0_5_2_WINDOW_HOURS=168 pnpm --filter @brevio/fomo run smoke-evidence:v0.5.2
pnpm --filter @brevio/fomo run smoke-evidence:v0.5.3
FOMO_V0_5_4_WINDOW_HOURS=168 pnpm --filter @brevio/fomo run smoke-evidence:v0.5.4
pnpm --filter @brevio/fomo run smoke-evidence:v0.5.5
pnpm --filter @brevio/fomo run smoke-evidence:v0.5.6
pnpm --filter @brevio/fomo run smoke-evidence:v0.5.7
```

**Known expected non-PASS shapes (NOT v0.5.7 regressions per C14 — same shapes SMOKE_REPORT_v0.5.6.md §4–§8 documented):**

- v0.5.3 may FAIL on Item #1 (no `/onboard/callback` in window — v0.5.7 is founder-only, expected)
- v0.5.4 may FAIL on C13/C14 (window-slide false positives)
- v0.5.5 will FAIL C2/C3/C11 (SendBlue OPTED_OUT blocked-external — F1 own future phase)

v0.5.7 should report **VERDICT: PASS** if Tests 1+2+3+4 succeeded.

---

## §8 — Fill `SMOKE_REPORT_v0.5.7.md`

```bash
cp docs/SMOKE_REPORT_TEMPLATE_v0.5.7.md docs/SMOKE_REPORT_v0.5.7.md
```

Open and fill:
- §1 prerequisites (tick what's done; Path A → real-iMessage rows = N/A)
- §3 PASS criteria table with evidence
- §4–§9 paste each evidence-script output
- §10 paste rendered-sample bodies + founder eye-test notes + (Path B only) real iMessage text
- §11 founder observations
- §12 verdict: PASS / FAIL / PENDING
- §13 sign-off + date

---

## §9 — Aftercare

- [ ] If Test 2 mutated a `rank_results.reason`, no rollback needed (historical)
- [ ] If Path B Test 3 re-flagged founder OPTED_OUT, that's expected (v0.5.7 doesn't undo it)
- [ ] Kill Terminal 1 dev server + Terminal 2 ngrok (if used)
- [ ] If runtime accidentally introduced an LLM call in `renderHumanMessage`, REVERT — 3E.1 invariant

---

## §10 — Commit the report

```bash
git checkout -b docs-smoke-report-v0.5.7
git add docs/SMOKE_REPORT_v0.5.7.md
git commit -m "docs: SMOKE_REPORT_v0.5.7 VERDICT: <PASS/FAIL>"
git push -u origin docs-smoke-report-v0.5.7
gh pr create --title "docs: v0.5.7 SMOKE_REPORT VERDICT: <verdict>" --body "..."
```

Or commit directly to main if preferred.

---

## What v0.5.7 PASS does NOT promise

Per the [scope memory](../.claude/projects/-Users-galiettemita-Downloads-Executive-AI-Agent-backend/memory/project_v05-7-scope.md) §"Scope boundaries":

- ❌ F1 SendBlue tier fix (own future phase)
- ❌ PIL substrate (own future phase)
- ❌ Auto-send (own gate per FOMO_PLAN v0.8)
- ❌ 3E.1 reversal (permanently preserved)
- ❌ Second HMR surface (calendar / drafts / tasks / etc. — each own 6Q gate)
- ❌ Per-user tone customization (PIL-adjacent)
- ❌ Short-body length policy resolution (its own future gate per v0.5.6 PASS finding #1)
- ❌ Runbook drift-detector amendment (its own future gate per v0.5.6 PASS finding #2)
- ❌ HMR plugin registry / multi-surface framework (Q6.A restraint)
- ❌ SaaS / vendor message renderer (Brevio owns HMR end-to-end)

**Next phase is decided AT THE NEXT 6-question gate.**
