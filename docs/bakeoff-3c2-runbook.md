# Phase 3C.2 — Anthropic Ranker Bake-Off (Runbook)

> **Goal.** Pick `claude-haiku-4-5-20251001` vs `claude-sonnet-4-6` as
> the FOMO ranker's primary model on real Anthropic API. Measure
> precision, recall, FP, FN, JSON validity, latency, cost. Document
> results in [BAKEOFF_REPORT_TEMPLATE_3C2.md](BAKEOFF_REPORT_TEMPLATE_3C2.md)
> and commit as `docs/BAKEOFF_REPORT_3C2.md`.
>
> Phase 3C.3 (worker integration) is **blocked** until this report
> lands with a primary + failover (or with `manual_review_required` +
> a documented plan).

---

## 0. Non-goals (what this PR/run does **not** do)

- ❌ no Gmail-worker integration (3C.3)
- ❌ no `FOMO_RANKER_ENABLED` kill switch (3C.3)
- ❌ no real founder Gmail involvement (still synthetic fixtures only)
- ❌ no new tool ids, no executor_status flips, no HTTP routes
- ❌ no writes to Neon (`InMemoryCostStore` only; production data untouched)

---

## 1. Get an Anthropic API key

1. Open <https://console.anthropic.com/> → sign in.
2. Settings → API Keys → "Create Key". Name it `brevio-fomo-bakeoff`.
3. Copy the `sk-ant-...` string. You won't see it again.
4. Make sure you have credits or a card on file. Estimated cost of one
   full bake-off (20 fixtures × 2 models): **~$0.05–$0.10**.

> **Do not** commit the key. It lives only in your shell env or in a
> gitignored `.env.*.local` file.

---

## 2. Set the env

Either inline:

```bash
export ANTHROPIC_API_KEY='sk-ant-...'
```

Or add a line to your existing `apps/fomo/.env.3b3.local` (already
gitignored) and re-source:

```bash
# In apps/fomo/.env.3b3.local, append:
ANTHROPIC_API_KEY=sk-ant-...

# Re-source:
set -a; source apps/fomo/.env.3b3.local; set +a
```

Sanity-check:

```bash
echo "ANTHROPIC_API_KEY prefix: ${ANTHROPIC_API_KEY:0:8}..."
# Should print: ANTHROPIC_API_KEY prefix: sk-ant-a... (or similar)
```

---

## 3. Preflight

```bash
cd "/Users/galiettemita/Downloads/Executive AI Agent/backend"
pnpm --filter @brevio/fomo run preflight:3c2
```

Expected output ends with `✓ Preflight passed.`. If it fails, fix the
listed env var and re-run.

The preflight is read-only — no Anthropic API call, no DB call, no money
spent.

---

## 4. Run the bake-off

```bash
pnpm --filter @brevio/fomo run bakeoff:3c2
```

The script will:

1. Build the 20 synthetic fixtures through `applyEgressForRanker` +
   `buildRankerPrompt` (same path the production ranker uses).
2. For each candidate model (Haiku 4.5 then Sonnet 4.6):
   - Call the Anthropic Messages API once per fixture.
   - Parse the JSON output via `validateRankerOutput`.
   - Record latency + input/output tokens + estimated cost.
3. Print a per-fixture trace + per-model summary table.
4. Apply the **Conservative pick rule**:
   - `precision ≥ 0.85` AND `recall ≥ 0.85` AND `json_valid ≥ 0.95`
   - **Primary** = cheapest model passing all three
   - **Failover** = the other model, only if its `json_valid ≥ 0.95`
   - If no model passes → `manual_review_required` (no auto-pick)
5. Print the recommendation.
6. Write a structured JSON artifact to
   `docs/bakeoff-3c2-results.json` for the founder report.

Expected total runtime: ~1–2 minutes (40 sequential API calls, average
2–3s each). Expected total spend: ~$0.05–$0.10 against your Anthropic
account.

---

## 5. Review the output

Sample success output:

```
========================================================================
Phase 3C.2 bake-off — per-model summary
========================================================================

Model: claude-haiku-4-5-20251001
  prompt_version       ranker-v0.1.0
  total fixtures       20
  json_valid           20/20  (100.0%)
  TP/FP/TN/FN          10/0/9/1
  precision            1.000
  recall               0.900
  F1                   0.947
  mean / p95 latency   1100ms / 1800ms
  total tokens (in/out) 12200 / 480
  total cost           $0.0146
  cost per 1k emails   $0.73

Model: claude-sonnet-4-6
  ...

========================================================================
Recommendation
========================================================================
  Gate: precision >= 0.85, recall >= 0.85, json_valid >= 0.95
  Primary:  claude-haiku-4-5-20251001
  Failover: claude-sonnet-4-6
  claude-haiku-4-5-20251001 is the cheapest model passing the gate; ...
  Verdict: AUTO_PICKED
```

Both your `stdout` capture and the JSON artifact at
`docs/bakeoff-3c2-results.json` go into the report.

### If `Verdict: MANUAL_REVIEW_REQUIRED`

The script exits 1. Read the per-fixture trace to see which fixtures
each model missed:

- Many false positives → prompt is too permissive; tighten the
  "default to not_important" instruction.
- Many false negatives → prompt is too conservative; add examples of
  the missed categories.
- Low `json_valid_rate` → output schema isn't being respected; tighten
  the "Output ONLY a single-line JSON object" instruction or move the
  schema reminder closer to the email context.

Edit `apps/fomo/src/ranker/prompt.ts`, **bump `PROMPT_VERSION`**, re-run
the bake-off, and re-evaluate. Both the old and new prompts will show
in the cost_records of any production-wired run later, so a future
audit can attribute regressions.

---

## 6. Write the report

```bash
cd "/Users/galiettemita/Downloads/Executive AI Agent/backend"
cp docs/BAKEOFF_REPORT_TEMPLATE_3C2.md docs/BAKEOFF_REPORT_3C2.md
```

Fill in `docs/BAKEOFF_REPORT_3C2.md` with:

- Run metadata (date, your name, commit SHA, prompt version)
- The script's recommendation (primary + failover)
- Per-model metrics from the summary table
- Your interpretation:
  - Do you agree with the pick? Why / why not?
  - Notable per-fixture misses (cite the fixture IDs)
  - Any prompt revisions you'd plan for 3C.3 or later
- Final verdict: **PRIMARY = …**, **FAILOVER = …** (or "deferred")

Also commit `docs/bakeoff-3c2-results.json` so the raw data is
preserved alongside the interpretive report.

---

## 7. Commit the report to the SAME PR branch — then merge

**Important.** PR #23 for `phase3c2-ranker-bakeoff` was opened when the
scaffolding was first pushed. It stays OPEN through the bake-off. The
bake-off result + report commit lands on the same branch and shows up
in the same PR. **Only merge the PR after the report commit is in the
branch.** Phase 3C.2 is "complete" only when the decision is
documented, not when the scripts exist.

```bash
git add docs/BAKEOFF_REPORT_3C2.md docs/bakeoff-3c2-results.json
git commit -m "phase 3C.2: bake-off complete — primary <model>, failover <model>"
git push origin phase3c2-ranker-bakeoff
```

That push updates the existing PR. Review the diff (now includes
scaffolding + decision), then merge.

If the verdict is `manual_review_required`:
- Either bump `PROMPT_VERSION`, re-run the bake-off, and commit a new
  report iteration on the same branch.
- Or commit the report with verdict `INVESTIGATE` + a written plan,
  push, and merge — the report explicitly marks 3C.3 as blocked until
  a follow-up bake-off picks models.

## After merge: 3C.3 unblocked

Phase 3C.3 wires the ranker into the polling worker behind
`FOMO_RANKER_ENABLED=false` using the **primary** model picked here,
with `failover` as the second-attempt model when the primary errors.
The bake-off report is the load-bearing input.
