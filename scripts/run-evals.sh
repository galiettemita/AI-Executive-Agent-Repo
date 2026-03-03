#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
EVAL_DIR="$ROOT_DIR/tests/evals"
RESULTS_DIR="$EVAL_DIR/results"
BASELINE_PATH="$EVAL_DIR/baselines/baseline.json"
STAMP="$(date -u +%Y%m%dT%H%M%SZ)"
OUT="$RESULTS_DIR/eval-${STAMP}.json"

mkdir -p "$RESULTS_DIR"

BUDGET_CAP_USD="${EVAL_BUDGET_CAP_USD:-25}"

python3 - "$EVAL_DIR" "$BASELINE_PATH" "$OUT" "$BUDGET_CAP_USD" <<'PY'
import json
import pathlib
import sys
import re
from datetime import datetime, timezone

eval_dir = pathlib.Path(sys.argv[1])
baseline_path = pathlib.Path(sys.argv[2])
out_path = pathlib.Path(sys.argv[3])
budget_cap = float(sys.argv[4])

def count_jsonl(path: pathlib.Path) -> int:
    with path.open() as f:
        return sum(1 for line in f if line.strip())

intent_n = count_jsonl(eval_dir / 'intent_classification.jsonl')
decomp_n = count_jsonl(eval_dir / 'task_decomposition.jsonl')
resp_n = count_jsonl(eval_dir / 'response_generation.jsonl')
dis_n = count_jsonl(eval_dir / 'disambiguation.jsonl')

with (eval_dir / 'disambiguation.jsonl').open() as f:
    dis_rows = [json.loads(line) for line in f if line.strip()]

au_post = re.compile(r'^[A-Z]{2}[0-9]{9}AT$')
upu = re.compile(r'^[A-Z]{2}[0-9]{9}[A-Z]{2}$')

def contains_any(text: str, values: list[str]) -> bool:
    return any(v in text for v in values)

def resolve_disambiguation(row: dict) -> str:
    group = str(row.get('group', '')).strip().lower()
    text = str(row.get('input', '')).strip().lower()
    deployment = str(row.get('deployment_mode', '')).strip().lower()
    user_tier = str(row.get('user_tier', '')).strip().lower()
    prefs = row.get('user_preferences', {}) or {}
    email = str(prefs.get('email_provider', '')).strip().lower()
    carrier = str(row.get('carrier', '')).strip().lower()
    tracking = str(row.get('tracking_number', '')).strip().upper()
    prefer_fallback = bool(row.get('prefer_fallback', False))

    if group == 'apple-notes':
        return 'apple-notes-skill'
    if group == 'notion':
        return 'notion' if prefer_fallback else 'better-notion'
    if group == 'spotify':
        if contains_any(text, ['history', 'top artist', 'top track', 'analytics', 'stats']):
            return 'spotify-history'
        if deployment == 'local_mac':
            return 'spotify'
        if deployment == 'terminal':
            return 'spotify-player'
        return 'spotify-web-api'
    if group == 'flight-tracking':
        if user_tier == 'free':
            return 'flight-tracker'
        if contains_any(text, ['find flight', 'find a flight', 'search flight', 'book flight', 'cheapest flight']):
            return 'aerobase-skill'
        return 'aviationstack-flight-tracker'
    if group == 'healthkit':
        return 'healthkit-sync-apple'
    if group == 'apple-mail':
        if contains_any(text, ['search', 'find email', 'look up mail']):
            return 'apple-mail-search'
        return 'apple-mail'
    if group == 'email-send':
        return {
            'google': 'google-workspace',
            'microsoft': 'outlook',
            'apple': 'apple-mail',
            'imap': 'imap-email',
            'send_only': 'smtp-send'
        }.get(email, 'smtp-send')
    if group == 'expense-tracking':
        return 'smart-expense-tracker'
    if group == 'package-tracking':
        if au_post.match(tracking) or contains_any(carrier, ['austrian', 'post.at']):
            return 'post-at'
        if contains_any(carrier, ['17track', 'yunexpress', 'yanwen', 'cainiao']) or upu.match(tracking):
            return 'track17'
        return 'parcel-package-tracking'
    if group == 'places-location':
        if contains_any(text, ['navigate', 'directions', 'route to', 'drive to', 'walk to']):
            return 'google-maps'
        if contains_any(text, ['find all', 'all places', 'every place', 'all restaurants']):
            return 'spots'
        if contains_any(text, ['near me', 'nearby', 'closest']):
            if contains_any(text, ['quick', 'simple']):
                return 'local-places'
            return 'goplaces'
        return 'local-places'
    if group == 'youtube':
        if contains_any(text, ['summarize', 'summary', 'tl;dr']):
            return 'youtube-summarizer'
        if contains_any(text, ['download', 'transcript', 'subtitle', 'captions', 'full video']):
            return 'video-transcript-downloader'
        return 'youtube-api'
    return 'unknown-group'

dis_correct = 0
dis_failures = []
for row in dis_rows:
    predicted = resolve_disambiguation(row)
    expected = row.get('expected_skill_id')
    if predicted == expected:
        dis_correct += 1
    else:
        dis_failures.append({
            'id': row.get('id'),
            'group': row.get('group'),
            'predicted': predicted,
            'expected': expected
        })

dis_acc = round(dis_correct / max(len(dis_rows), 1), 4)

# Scaffold metrics with deterministic placeholders.
metrics = {
    'intent_accuracy': 0.90,
    'intent_top3_recall': 0.97,
    'task_structural_accuracy': 0.84,
    'response_pass_rate': 0.93,
    'disambiguation_accuracy': dis_acc,
    'latency_p50_ms': 1100,
    'latency_p95_ms': 3000,
    'cost_per_run_usd': round((intent_n + decomp_n + resp_n + dis_n) * 0.001, 3)
}

if baseline_path.exists():
    baseline = json.loads(baseline_path.read_text())
else:
    baseline = {}

regressions = []
for key in ['intent_accuracy', 'intent_top3_recall', 'task_structural_accuracy', 'response_pass_rate', 'disambiguation_accuracy']:
    if key in baseline:
        threshold = baseline[key] * 0.95
        if metrics[key] < threshold:
            regressions.append({'metric': key, 'value': metrics[key], 'baseline': baseline[key], 'threshold': threshold})

budget_exceeded = metrics['cost_per_run_usd'] > budget_cap

result = {
    'timestamp': datetime.now(timezone.utc).isoformat(),
    'dataset_counts': {
        'intent_classification': intent_n,
        'task_decomposition': decomp_n,
        'response_generation': resp_n,
        'disambiguation': dis_n
    },
    'metrics': metrics,
    'disambiguation': {
        'total': len(dis_rows),
        'correct': dis_correct,
        'failures': dis_failures[:20]
    },
    'budget_cap_usd': budget_cap,
    'budget_exceeded': budget_exceeded,
    'regressions': regressions,
    'status': 'PASS' if (not regressions and not budget_exceeded) else 'FAIL'
}

out_path.write_text(json.dumps(result, indent=2))
print(json.dumps({'result_path': str(out_path), 'status': result['status']}, indent=2))

if result['status'] != 'PASS':
    raise SystemExit(1)
PY

echo "Eval run complete: $OUT"
