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

Baselines:
- `tests/evals/baselines/baseline.json`
