# LLM Evaluation Framework

Datasets:
- `intent_classification.jsonl` (200 examples)
- `task_decomposition.jsonl` (50 examples)
- `response_generation.jsonl` (100 examples)
- `disambiguation.jsonl` (110 examples)

Run:
- `bash scripts/run-evals.sh`

Outputs:
- `tests/evals/results/eval-<timestamp>.json`
- Includes dataset-level pass/fail details, latency (`p50`/`p95`), token usage, estimated run cost, and regression checks against baseline metrics.

Baselines:
- `tests/evals/baselines/baseline.json`

Notes:
- The harness is deterministic and offline-safe: it scores datasets with reproducible evaluators for intent classification, task decomposition, response guardrails, and disambiguation routing.
