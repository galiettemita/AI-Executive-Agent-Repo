# Phase 3C.2 Ranker Bake-Off Report

> Fill this after running [bakeoff-3c2-runbook.md](bakeoff-3c2-runbook.md).
> Commit as `docs/BAKEOFF_REPORT_3C2.md` (drop `_TEMPLATE_`) once
> primary + failover are picked. Phase 3C.3 is blocked until this
> report lands.

---

**Founder:** _<your name>_
**Run date:** _<YYYY-MM-DD HH:MM TZ>_
**Branch:** `phase3c2-ranker-bakeoff`
**Commit SHA at run time:** _<git rev-parse HEAD>_
**Prompt version:** _<from script output, e.g. ranker-v0.1.0>_
**Anthropic account:** _<email or workspace name; no API key>_

---

## 1. Setup confirmation

- `ANTHROPIC_API_KEY` set in shell: ☐ yes
- `pnpm preflight:3c2` exited 0: ☐ yes
- Approximate Anthropic spend on this run: $_<from console>_

---

## 2. Candidates evaluated

| Model id                          | Tier   | List input $/1M | List output $/1M |
| --------------------------------- | ------ | --------------- | ---------------- |
| `claude-haiku-4-5-20251001`       | Haiku  | $1.00           | $5.00            |
| `claude-sonnet-4-6`               | Sonnet | $3.00           | $15.00           |

Pick rule applied: **Conservative**
(precision ≥ 0.85, recall ≥ 0.85, json_valid ≥ 0.95;
cheapest passing = primary; other = failover iff json_valid ≥ 0.95).

---

## 3. Per-model metrics

Paste from the script's "Per-model summary" section.

### `claude-haiku-4-5-20251001`

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
| passes gate?        | ☐ yes / ☐ no |

### `claude-sonnet-4-6`

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
| passes gate?        | ☐ yes / ☐ no |

---

## 4. Script recommendation

Paste from the script's "Recommendation" section verbatim:

```
Gate: ...
Primary:  ...
Failover: ...
Reason: ...
Verdict: ...
```

---

## 5. Notable per-fixture misses

If either model missed a fixture, list the fixture id, expected vs
predicted, and a one-line guess at why.

| Model | Fixture id | Expected | Predicted | Likely cause |
| ----- | ---------- | -------- | --------- | ------------ |
| ...   | ...        | ...      | ...       | ...          |

If you bumped `PROMPT_VERSION` and re-ran: note the version history
and whether the prompt change addressed the misses.

---

## 6. Founder interpretation

Do you accept the script's recommendation?

☐ **Accept as-is**
☐ **Override** (state the alternative pick + why)
☐ **Defer** (reason: prompt regression needed / fixtures need
expansion / etc.)

Notes / followups for 3C.3 or later phases:

- _…_

---

## 7. Final pick

- **Primary model:** _<e.g. claude-haiku-4-5-20251001>_
- **Failover model:** _<e.g. claude-sonnet-4-6, or "none">_

Phase 3C.3 will wire the primary into the polling worker (behind
`FOMO_RANKER_ENABLED=false`) and use the failover as second-attempt
when the primary errors.

---

## 8. Artifact

- Structured results JSON committed alongside this report:
  `docs/bakeoff-3c2-results.json` ☐

---

## 9. Sign-off

- **Founder signature:** _<name>_
- **Date:** _<YYYY-MM-DD>_
