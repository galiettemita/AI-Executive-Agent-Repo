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

**Founder:** Galiette Mita
**Run date:** 2026-05-24 EST
**Branch:** `phase3c2-openai-ranker-smoke-eval`
**Commit SHA at run time:** `cf3246a3deee23dfb9bfb471dd7f5265810a3acc`
**Prompt version:** `ranker-v0.1.0`
**OpenAI account:** galiettemita@icloud.com

---

## 1. Setup confirmation

- `OPENAI_API_KEY` set in shell: ☒ yes
- `pnpm preflight:3c2` exited 0: ☒  yes
- Approximate OpenAI spend on this run: $0.005

---

## 2. Model chosen

- **Model id used:** gpt-5-mini
- **Why this model:**
  - ☒  Default (founder directive: low-cost OpenAI mini, gpt-5-mini)
  - ☐ Fallback (gpt-5-mini unavailable on this account; cheapest
    structured-output-capable substitute)
  - ☐ Override (state reason: ...)

- **Pricing (per 1M tokens):** input $0.25 , output $2
- **Estimated cost per 1k emails (from script output):** $0.59

---

## 3. Smoke eval metrics

From the script's summary block:

| Metric                | Value             |
| --------------------- | ----------------- |
| prompt_version        | `ranker-v0.1.0`   |
| total fixtures        | 20                |
| json_valid            | 20/20 (100.0%)    |
| TP / FP / TN / FN     | 10 / 0 / 10 / 0   |
| precision             | 1.000             |
| recall                | 1.000             |
| F1                    | 1.000             |
| mean / p95 latency    | 4107ms / 7107ms   |
| total tokens (in/out) | 8100 / 4917       |
| total cost            | $0.0119           |
| cost per 1k emails    | $0.59             |

Pass-gate check (Conservative):

- precision ≥ 0.85   ☒ pass
- recall ≥ 0.85      ☒ pass
- json_valid ≥ 0.95  ☒ pass

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

**N/A — `json_valid_rate = 1.000` (20/20). Strict `response_format:
json_schema` held across every fixture; zero API errors, zero
refusals, zero parse failures.**

---

## 5. Notable per-fixture misses

If any fixture was classified wrong (predicted_label ≠ expected_label),
list it with a one-line guess at why.

**N/A — zero misses. All 10 `important` fixtures classified
`important`; all 10 `not_important` fixtures classified `not_important`.
PROMPT_VERSION held at `ranker-v0.1.0` throughout the run; no
revisions needed.**

---

## 6. Verdict

☒ **PASS** — all three pass-gate checks green. Phase 3C.3 may begin.
☐ **INVESTIGATE** — at least one gate failed. Phase 3C.3 is blocked
   until a follow-up smoke eval passes.

Reason / plan if INVESTIGATE:

- N/A (PASS).

---

## 7. Whether 3C.3 is allowed

- `allows_3c3` field from `docs/openai-smoke-eval-3c2-results.json`:
  ☒ true / ☐ false
- Matches §6 verdict: ☒ yes

If `false`: list the specific next action before re-attempting 3C.3:

- N/A (`allows_3c3 = true`).

**Phase 3C.3 is UNBLOCKED.** Wire the OpenAI ranker (model
`gpt-5-mini`, prompt `ranker-v0.1.0`) into the Gmail polling worker
behind `FOMO_RANKER_ENABLED=false` (default safe).

---

## 8. Artifact

- Structured results JSON committed alongside this report:
  `docs/openai-smoke-eval-3c2-results.json`☒

---

## 9. Sign-off

- **Founder signature:** Galiette Mita
- **Date:** 2026-05-24
