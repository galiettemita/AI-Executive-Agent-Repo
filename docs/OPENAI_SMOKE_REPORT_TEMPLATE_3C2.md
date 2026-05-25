# Phase 3C.2 OpenAI Ranker Smoke Eval Report

> Founder directive 2026-05-24 superseded the prior Anthropic
> Haiku-vs-Sonnet bake-off path. 3C.2 is now: OpenAI as initial Brevio
> ranker provider, with a single-model smoke eval against the 20
> synthetic fixtures.
>
> Fill this after running [openai-smoke-eval-3c2-runbook.md](openai-smoke-eval-3c2-runbook.md).
> Commit as `docs/OPENAI_SMOKE_REPORT_3C2.md` (drop `_TEMPLATE_`) to
> the same PR branch alongside `docs/openai-smoke-eval-3c2-results.json`,
> then merge. Phase 3C.3 is blocked until VERDICT = PASS lands.

---

**Founder:** _<your name>_
**Run date:** _<YYYY-MM-DD HH:MM TZ>_
**Branch:** `phase3c2-openai-ranker-smoke-eval`
**Commit SHA at run time:** _<git rev-parse HEAD>_
**Prompt version:** _<from script output, e.g. ranker-v0.1.0>_
**OpenAI account:** _<email or workspace name; no API key>_

---

## 1. Setup confirmation

- `OPENAI_API_KEY` set in shell: ☐ yes
- `pnpm preflight:3c2` exited 0: ☐ yes
- Approximate OpenAI spend on this run: $_<from console>_

---

## 2. Model chosen

- **Model id used:** _<e.g. gpt-5-mini>_
- **Why this model:**
  - ☐ Default (founder directive: low-cost OpenAI mini, gpt-5-mini)
  - ☐ Fallback (gpt-5-mini unavailable on this account; cheapest
    structured-output-capable substitute)
  - ☐ Override (state reason: ...)

- **Pricing (per 1M tokens):** input $_<…>_, output $_<…>_
- **Estimated cost per 1k emails (from script output):** $_<…>_

---

## 3. Smoke eval metrics

Paste from the script's "summary" section.

| Metric              | Value |
| ------------------- | ----- |
| total fixtures      |       |
| json_valid          |  / 20 ( %) |
| TP / FP / TN / FN   |       |
| precision           |       |
| recall              |       |
| F1                  |       |
| mean / p95 latency  |   ms / ms |
| total tokens (in/out) |     |
| total cost          | $     |
| cost per 1k emails  | $     |

Pass-gate check (Conservative):

- precision ≥ 0.85  ☐ pass / ☐ fail
- recall ≥ 0.85     ☐ pass / ☐ fail
- json_valid ≥ 0.95 ☐ pass / ☐ fail

---

## 4. JSON validity

OpenAI was configured with strict `response_format: json_schema` so
the model should produce shape-valid output every time. Any
`json_valid: false` rows indicate either:

- the model fell back to refusal (caught as `OpenAIApiError(model_refusal)`)
- the response_format wasn't honored (model id might not support strict mode)
- a 4xx/5xx error blocked the call entirely

If `json_valid_rate < 1.0`, list the failing fixtures and the error
reason from the script trace:

| Fixture id | Error reason |
| ---------- | ------------ |
| _…_        | _…_          |

---

## 5. Notable per-fixture misses

If any fixture was classified wrong (predicted_label ≠ expected_label),
list it with a one-line guess at why.

| Fixture id | Expected | Predicted | Likely cause |
| ---------- | -------- | --------- | ------------ |
| _…_        | _…_      | _…_       | _…_          |

If you bumped `PROMPT_VERSION` and re-ran: note the version history.

---

## 6. Verdict

☐ **PASS** — all three pass-gate checks green. Phase 3C.3 may begin.
☐ **INVESTIGATE** — at least one gate failed. Phase 3C.3 is blocked
   until a follow-up smoke eval passes.

Reason / plan if INVESTIGATE:

- _…_

---

## 7. Whether 3C.3 is allowed

- `allows_3c3` field from `docs/openai-smoke-eval-3c2-results.json`:
  ☐ true / ☐ false
- Matches §6 verdict: ☐ yes

If `false`: list the specific next action before re-attempting 3C.3:

- _…_

---

## 8. Artifact

- Structured results JSON committed alongside this report:
  `docs/openai-smoke-eval-3c2-results.json` ☐

---

## 9. Sign-off

- **Founder signature:** _<name>_
- **Date:** _<YYYY-MM-DD>_
