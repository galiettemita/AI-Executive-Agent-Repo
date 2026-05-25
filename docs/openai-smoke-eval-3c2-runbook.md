# Phase 3C.2 — OpenAI Ranker Smoke Eval (Runbook)

> **Founder directive 2026-05-24:** OpenAI/ChatGPT is the initial brain
> for the FOMO ranker. The prior Phase 3C.2 path (Anthropic
> Haiku-vs-Sonnet bake-off) is **superseded**. AnthropicBackend stays
> in main as dormant future-provider support; the active ranker is
> OpenAI.
>
> Phase 3C.2 is **not a bake-off**. It is a single-provider, single-model
> smoke eval against the 20 synthetic fixtures, with a pass gate strict
> enough to catch catastrophic failures (bad prompt, wrong model id,
> structured-output regression) before Phase 3C.3 wires the ranker into
> the Gmail polling worker.
>
> When this report lands with `VERDICT: PASS`, Phase 3C.3 is unblocked.
> If it lands `INVESTIGATE`, 3C.3 stays blocked until a follow-up
> smoke eval passes.

---

## 0. Non-goals

- ❌ no bake-off (single provider, single model by directive)
- ❌ no Gmail-worker integration (3C.3)
- ❌ no `FOMO_RANKER_ENABLED` kill switch (3C.3)
- ❌ no Slack, SendBlue, reply parser, friend beta, auto-send
- ❌ no Neon writes (in-memory only; production data untouched)
- ❌ no founder Gmail involvement (synthetic fixtures only)

---

## 1. Get an OpenAI API key

1. Open <https://platform.openai.com/api-keys> → sign in.
2. Create new secret key. Name it `brevio-fomo-ranker-smoke`.
3. Copy the `sk-...` string.
4. Confirm credits or a payment method are on the account. Cost of one
   full smoke eval (20 fixtures × ~630 tokens × gpt-5-mini): **~$0.005**.

> **Do not commit the key.** Store in your shell env or a gitignored
> `.env.*.local` file.

---

## 2. Set the env

Inline:

```bash
export OPENAI_API_KEY='sk-...'
```

Or append to `apps/fomo/.env.3b3.local` (already gitignored) and
re-source:

```bash
# In apps/fomo/.env.3b3.local, append:
OPENAI_API_KEY=sk-...

# (Optional) override the default model:
# FOMO_OPENAI_MODEL=gpt-4o-mini

set -a; source apps/fomo/.env.3b3.local; set +a
```

Default model is `gpt-5-mini`. If your account doesn't have it
available yet, set `FOMO_OPENAI_MODEL` to the cheapest currently-
available OpenAI mini model that supports structured JSON output —
typically `gpt-4o-mini`. Anything other than the listed models will
still run, but `MODEL_PRICING` may not know its price and cost
estimates will report $0.

Sanity-check:

```bash
echo "OPENAI_API_KEY prefix: ${OPENAI_API_KEY:0:8}..."
echo "FOMO_OPENAI_MODEL: ${FOMO_OPENAI_MODEL:-<default gpt-5-mini>}"
```

---

## 3. Preflight

```bash
cd "/Users/galiettemita/Downloads/Executive AI Agent/backend"
pnpm --filter @brevio/fomo run preflight:3c2
```

Expected output ends with `✓ Preflight passed.` plus a printed cost
estimate. Fail-loud on missing key or wrong-prefix key.

---

## 4. Run the smoke eval

```bash
pnpm --filter @brevio/fomo run smoke-eval:3c2
```

The script:

1. Constructs `OpenAIBackend` with the strict JSON-schema response
   format — OpenAI enforces the `{label, score, reason}` shape
   server-side. `validateRankerOutput()` runs as defense-in-depth.
2. Builds the 20 synthetic fixtures through `applyEgressForRanker` +
   `buildRankerPrompt` (the exact path the production ranker uses).
3. Calls OpenAI once per fixture. Per-fixture trace prints `✓` (correct),
   `✗` (wrong label), or `!` (JSON-invalid / API error).
4. Computes precision / recall / F1 / TP-FP-TN-FN / json_valid_rate /
   mean & p95 latency / total cost / cost per 1k emails.
5. Applies the **Conservative pass gate**:
   - `precision ≥ 0.85`
   - `recall ≥ 0.85`
   - `json_valid_rate ≥ 0.95`
   All three required for PASS.
6. Prints the verdict (`PASS` or `INVESTIGATE`).
7. Writes structured JSON to `docs/openai-smoke-eval-3c2-results.json`.

Expected runtime: ~30–90 seconds (20 sequential API calls).
Expected spend: ~$0.005 against your OpenAI account.

---

## 5. Interpret the output

**PASS** — `docs/openai-smoke-eval-3c2-results.json` shows `allows_3c3: true`. Fill the report, commit, merge, then Phase 3C.3 unblocked.

**INVESTIGATE** — script exits 1. Read the per-fixture trace to see what failed:

- Many false negatives → model is over-conservative. Add explicit
  examples of personal-but-not-screaming-deadline-style emails to the
  prompt.
- Many false positives → model is over-permissive. Tighten the
  "default to not_important" instruction; add explicit not-important
  examples.
- Low `json_valid_rate` → structured-output enforcement is failing
  somehow. Check the OpenAI model id supports response_format
  json_schema (gpt-5 family and gpt-4o family do; older models may
  not). Try the fallback model.
- All-error → likely auth / model id wrong / billing problem (see §3
  preflight + the per-fixture error strings).

Edit `apps/fomo/src/ranker/prompt.ts`, **bump `PROMPT_VERSION`**, re-run
the smoke eval, commit the new artifact. Each PROMPT_VERSION shows up
in future production cost_records so regressions are attributable.

---

## 6. Write the report

```bash
cd "/Users/galiettemita/Downloads/Executive AI Agent/backend"
cp docs/OPENAI_SMOKE_REPORT_TEMPLATE_3C2.md docs/OPENAI_SMOKE_REPORT_3C2.md
```

Fill in:

- Run metadata (date, name, commit SHA, prompt version, resolved model)
- The verdict (PASS or INVESTIGATE)
- Per-model metrics from the script output
- Notable per-fixture misses (cite fixture IDs)
- Cost estimate
- Whether 3C.3 is allowed or blocked

---

## 7. Commit the report to the SAME PR branch — then merge

**Important.** The PR for `phase3c2-openai-ranker-smoke-eval` stays
OPEN through your smoke-eval run. Phase 3C.2 is "complete" only when
the verdict is documented and committed, NOT when the scripts exist.
This mirrors the 3B.3 pattern.

```bash
git add docs/OPENAI_SMOKE_REPORT_3C2.md docs/openai-smoke-eval-3c2-results.json
git commit -m "phase 3C.2: OpenAI smoke eval <PASS|INVESTIGATE> — model=<id>"
git push origin phase3c2-openai-ranker-smoke-eval
```

That push updates the open PR. Review the diff (scaffolding + verdict
in one PR), then merge.

If verdict = INVESTIGATE: do NOT merge until either (a) a re-run with
a bumped prompt-version PASSES, or (b) you intentionally land an
INVESTIGATE verdict + plan and explicitly mark 3C.3 blocked.

## After merge: 3C.3 unblocked (or blocked)

If PASS → Phase 3C.3 wires the chosen OpenAI model into the polling
worker behind `FOMO_RANKER_ENABLED=false`.

If INVESTIGATE landed → 3C.3 stays blocked until a follow-up 3C.2.x
smoke eval lands with PASS.
