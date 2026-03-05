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
import math
import pathlib
import sys
import re
import time
from datetime import datetime, timezone

eval_dir = pathlib.Path(sys.argv[1])
baseline_path = pathlib.Path(sys.argv[2])
out_path = pathlib.Path(sys.argv[3])
budget_cap = float(sys.argv[4])

def count_jsonl(path: pathlib.Path) -> int:
    with path.open() as f:
        return sum(1 for line in f if line.strip())

def load_jsonl(path: pathlib.Path):
    with path.open() as f:
        return [json.loads(line) for line in f if line.strip()]

def percentile(values, pct):
    if not values:
        return 0.0
    ordered = sorted(values)
    if len(ordered) == 1:
        return float(ordered[0])
    rank = (len(ordered) - 1) * pct
    lower = math.floor(rank)
    upper = math.ceil(rank)
    if lower == upper:
        return float(ordered[int(rank)])
    weight = rank - lower
    return float(ordered[lower] + (ordered[upper] - ordered[lower]) * weight)

def textify(value):
    if value is None:
        return ""
    if isinstance(value, str):
        return value
    return json.dumps(value, sort_keys=True, separators=(",", ":"))

def token_estimate(value):
    return max(1, len(re.findall(r"\S+", textify(value))))

def estimate_cost_usd(model, input_tokens, output_tokens):
    rates = {
        # USD per 1k tokens
        "haiku": (0.0008, 0.0040),
        "sonnet": (0.0030, 0.0150),
    }
    in_rate, out_rate = rates[model]
    return (input_tokens / 1000.0) * in_rate + (output_tokens / 1000.0) * out_rate

intent_rows = load_jsonl(eval_dir / 'intent_classification.jsonl')
decomp_rows = load_jsonl(eval_dir / 'task_decomposition.jsonl')
resp_rows = load_jsonl(eval_dir / 'response_generation.jsonl')
dis_rows = load_jsonl(eval_dir / 'disambiguation.jsonl')

intent_n = len(intent_rows)
decomp_n = len(decomp_rows)
resp_n = len(resp_rows)
dis_n = len(dis_rows)

intent_in_tokens = 0
intent_out_tokens = 0
decomp_in_tokens = 0
decomp_out_tokens = 0
resp_in_tokens = 0
resp_out_tokens = 0

au_post = re.compile(r'^[A-Z]{2}[0-9]{9}AT$')
upu = re.compile(r'^[A-Z]{2}[0-9]{9}[A-Z]{2}$')
using_skill = re.compile(r'\busing\s+([a-z0-9-]+)\b', re.IGNORECASE)
decomp_pair = re.compile(r'first\s+([a-z0-9-]+),\s*then\s+([a-z0-9-]+)', re.IGNORECASE)

def contains_any(text: str, values: list[str]) -> bool:
    return any(v in text for v in values)

def predict_intent(row):
    text = str(row.get("message_text", "")).strip()
    enabled = [str(v).strip() for v in (row.get("user_profile", {}) or {}).get("enabled_skills", []) if str(v).strip()]
    extracted = [match.lower() for match in using_skill.findall(text)]

    predicted_skills = []
    for skill_id in extracted:
        if (not enabled) or (skill_id in enabled):
            predicted_skills.append(skill_id)
    if not predicted_skills and enabled:
        predicted_skills = [enabled[0]]

    deduped = []
    seen = set()
    for skill_id in predicted_skills:
        if skill_id not in seen:
            deduped.append(skill_id)
            seen.add(skill_id)
    predicted_skills = deduped

    top3 = list(predicted_skills)
    for skill_id in enabled:
        if skill_id not in top3:
            top3.append(skill_id)
        if len(top3) >= 3:
            break

    requires_decomposition = (" and " in text.lower()) or len(predicted_skills) > 1
    primary = predicted_skills[0] if predicted_skills else "general_assistance"
    return {
        "intent": f"{primary}_intent" if predicted_skills else "general_assistance",
        "skills": predicted_skills,
        "requires_decomposition": requires_decomposition,
        "top3_skills": top3[:3],
    }

def predict_decomposition(row):
    request = str(row.get("request", "")).strip()
    pair = decomp_pair.search(request)
    if pair:
        first = pair.group(1).lower()
        second = pair.group(2).lower()
        return {
            "tasks": [
                {"id": "t1", "skill_id": first, "dependencies": [], "priority": 1},
                {"id": "t2", "skill_id": second, "dependencies": ["t1"], "priority": 2},
            ],
            "execution_order": "sequential",
        }
    return {"tasks": [], "execution_order": "parallel"}

def normalize_tasks(tasks):
    normalized = []
    for task in tasks:
        normalized.append(
            {
                "skill_id": str(task.get("skill_id", "")),
                "dependencies": [str(dep) for dep in task.get("dependencies", [])],
                "priority": int(task.get("priority", 0)),
            }
        )
    return normalized

def generate_response(row):
    parts = []
    for result in row.get("skill_results", []):
        skill_id = str(result.get("skill_id", "")).strip()
        summary = str((result.get("data") or {}).get("summary", "")).strip()
        if skill_id and summary:
            parts.append(f"{skill_id}: {summary}")
        elif skill_id:
            parts.append(skill_id)

    if not parts:
        text = "Update: no completed skill results available yet."
    else:
        text = "Update: " + "; ".join(parts) + "."
    return text[:4096]

intent_latencies = []
intent_correct = 0
intent_top3_hits = 0
intent_failures = []
for row in intent_rows:
    start = time.perf_counter()
    predicted = predict_intent(row)
    intent_latencies.append((time.perf_counter() - start) * 1000.0)

    expected = row.get("expected", {})
    expected_skills = [str(v) for v in expected.get("skills", [])]
    expected_requires = bool(expected.get("requires_decomposition", False))
    expected_intent = str(expected.get("intent", ""))

    is_exact = (
        predicted["intent"] == expected_intent
        and predicted["skills"] == expected_skills
        and predicted["requires_decomposition"] == expected_requires
    )
    if is_exact:
        intent_correct += 1
    else:
        intent_failures.append(
            {
                "id": row.get("id"),
                "predicted": {
                    "intent": predicted["intent"],
                    "skills": predicted["skills"],
                    "requires_decomposition": predicted["requires_decomposition"],
                },
                "expected": {
                    "intent": expected_intent,
                    "skills": expected_skills,
                    "requires_decomposition": expected_requires,
                },
            }
        )

    if set(expected_skills).issubset(set(predicted["top3_skills"])):
        intent_top3_hits += 1

    intent_in_tokens += token_estimate(row.get("message_text", "")) + token_estimate(row.get("user_profile", {}))
    intent_out_tokens += token_estimate(predicted)

decomp_latencies = []
decomp_correct = 0
decomp_failures = []
for row in decomp_rows:
    start = time.perf_counter()
    predicted = predict_decomposition(row)
    decomp_latencies.append((time.perf_counter() - start) * 1000.0)

    expected = row.get("expected", {})
    expected_tasks = normalize_tasks(expected.get("tasks", []))
    predicted_tasks = normalize_tasks(predicted.get("tasks", []))
    expected_order = str(expected.get("execution_order", ""))
    predicted_order = str(predicted.get("execution_order", ""))

    is_exact = predicted_tasks == expected_tasks and predicted_order == expected_order
    if is_exact:
        decomp_correct += 1
    else:
        decomp_failures.append(
            {
                "id": row.get("id"),
                "predicted": {"tasks": predicted_tasks, "execution_order": predicted_order},
                "expected": {"tasks": expected_tasks, "execution_order": expected_order},
            }
        )

    decomp_in_tokens += token_estimate(row.get("request", ""))
    decomp_out_tokens += token_estimate(predicted)

response_latencies = []
response_passes = 0
response_failures = []
unauthorized_phrases = [
    "i placed the order",
    "i purchased",
    "i charged your card",
    "payment submitted",
]
for row in resp_rows:
    start = time.perf_counter()
    response_text = generate_response(row)
    response_latencies.append((time.perf_counter() - start) * 1000.0)

    expected = row.get("expected_response", {})
    must_include = [str(v).lower() for v in expected.get("must_include", [])]
    max_chars = int(expected.get("max_chars", 4096))
    no_unauth = bool(expected.get("no_unauthorized_financial_commitment", False))

    lowered = response_text.lower()
    include_ok = all(token in lowered for token in must_include)
    len_ok = len(response_text) <= max_chars
    financial_ok = (not no_unauth) or all(phrase not in lowered for phrase in unauthorized_phrases)
    passed = include_ok and len_ok and financial_ok

    if passed:
        response_passes += 1
    else:
        response_failures.append(
            {
                "id": row.get("id"),
                "response_text": response_text,
                "checks": {
                    "include_ok": include_ok,
                    "len_ok": len_ok,
                    "financial_ok": financial_ok,
                },
            }
        )

    resp_in_tokens += token_estimate(row.get("skill_results", [])) + token_estimate(row.get("user_profile", {}))
    resp_out_tokens += token_estimate(response_text)

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
dis_latencies = []
for row in dis_rows:
    start = time.perf_counter()
    predicted = resolve_disambiguation(row)
    dis_latencies.append((time.perf_counter() - start) * 1000.0)
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
intent_accuracy = round(intent_correct / max(intent_n, 1), 4)
intent_top3_recall = round(intent_top3_hits / max(intent_n, 1), 4)
task_accuracy = round(decomp_correct / max(decomp_n, 1), 4)
response_pass_rate = round(response_passes / max(resp_n, 1), 4)

total_cost_usd = (
    estimate_cost_usd("haiku", intent_in_tokens, intent_out_tokens)
    + estimate_cost_usd("sonnet", decomp_in_tokens, decomp_out_tokens)
    + estimate_cost_usd("sonnet", resp_in_tokens, resp_out_tokens)
)

all_latencies = intent_latencies + decomp_latencies + response_latencies + dis_latencies
metrics = {
    'intent_accuracy': intent_accuracy,
    'intent_top3_recall': intent_top3_recall,
    'task_structural_accuracy': task_accuracy,
    'response_pass_rate': response_pass_rate,
    'disambiguation_accuracy': dis_acc,
    'latency_p50_ms': round(percentile(all_latencies, 0.50), 2),
    'latency_p95_ms': round(percentile(all_latencies, 0.95), 2),
    'cost_per_run_usd': round(total_cost_usd, 4),
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
    'intent_classification': {
        'total': intent_n,
        'correct': intent_correct,
        'failures': intent_failures[:20],
    },
    'task_decomposition': {
        'total': decomp_n,
        'correct': decomp_correct,
        'failures': decomp_failures[:20],
    },
    'response_generation': {
        'total': resp_n,
        'passed': response_passes,
        'failures': response_failures[:20],
    },
    'token_usage': {
        'intent': {'input_tokens': intent_in_tokens, 'output_tokens': intent_out_tokens},
        'decomposition': {'input_tokens': decomp_in_tokens, 'output_tokens': decomp_out_tokens},
        'response_generation': {'input_tokens': resp_in_tokens, 'output_tokens': resp_out_tokens},
        'total_input_tokens': intent_in_tokens + decomp_in_tokens + resp_in_tokens,
        'total_output_tokens': intent_out_tokens + decomp_out_tokens + resp_out_tokens,
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
