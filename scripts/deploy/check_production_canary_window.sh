#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT_DIR"

if ! command -v python3 >/dev/null 2>&1; then
  echo "python3 is required to check production canary window" >&2
  exit 1
fi

OUTPUT_PATH="${OUTPUT_PATH:-artifacts/deploy/production_canary_check.json}"
ALLOW_CONDITIONAL_MANUAL="${ALLOW_CONDITIONAL_MANUAL:-1}"

CANARY_TRAFFIC_PCT="${CANARY_TRAFFIC_PCT:-10}"
CANARY_DURATION_MINUTES="${CANARY_DURATION_MINUTES:-15}"
CANARY_ERROR_RATE_PCT="${CANARY_ERROR_RATE_PCT:-}"
CANARY_P99_RATIO="${CANARY_P99_RATIO:-}"

mkdir -p "$(dirname "$OUTPUT_PATH")"

python3 - "$OUTPUT_PATH" "$ALLOW_CONDITIONAL_MANUAL" "$CANARY_TRAFFIC_PCT" "$CANARY_DURATION_MINUTES" "$CANARY_ERROR_RATE_PCT" "$CANARY_P99_RATIO" <<'PY'
import json
import sys
from datetime import datetime, timezone

(
    output_path,
    allow_conditional_manual_raw,
    traffic_pct_raw,
    duration_minutes_raw,
    error_rate_raw,
    p99_ratio_raw,
) = sys.argv[1:7]

allow_conditional_manual = allow_conditional_manual_raw == "1"

status = "FAIL"
pass_canary = False
reason = "canary check failed"
checks = []

# traffic check
try:
    traffic_pct = float(traffic_pct_raw)
    if traffic_pct >= 10.0:
        checks.append({"id": "traffic_pct", "status": "PASS", "detail": f"{traffic_pct}%"})
    else:
        checks.append({"id": "traffic_pct", "status": "FAIL", "detail": f"{traffic_pct}% (<10%)"})
except ValueError:
    checks.append({"id": "traffic_pct", "status": "FAIL", "detail": "invalid numeric value"})

# duration check
try:
    duration_minutes = float(duration_minutes_raw)
    if duration_minutes >= 15.0:
        checks.append({"id": "duration_minutes", "status": "PASS", "detail": f"{duration_minutes}m"})
    else:
        checks.append({"id": "duration_minutes", "status": "FAIL", "detail": f"{duration_minutes}m (<15m)"})
except ValueError:
    checks.append({"id": "duration_minutes", "status": "FAIL", "detail": "invalid numeric value"})

# metric checks
if error_rate_raw.strip() == "" or p99_ratio_raw.strip() == "":
    checks.append({
        "id": "slo_metrics",
        "status": "MANUAL",
        "detail": "CANARY_ERROR_RATE_PCT or CANARY_P99_RATIO not provided",
    })
else:
    try:
        error_rate = float(error_rate_raw)
        p99_ratio = float(p99_ratio_raw)
        if error_rate <= 1.0 and p99_ratio <= 2.0:
            checks.append({
                "id": "slo_metrics",
                "status": "PASS",
                "detail": f"error_rate_pct={error_rate}, p99_ratio={p99_ratio}",
            })
        else:
            checks.append({
                "id": "slo_metrics",
                "status": "FAIL",
                "detail": f"SLO breach: error_rate_pct={error_rate}, p99_ratio={p99_ratio}",
            })
    except ValueError:
        checks.append({"id": "slo_metrics", "status": "FAIL", "detail": "invalid numeric metric value"})

fail_count = sum(1 for item in checks if item["status"] == "FAIL")
manual_count = sum(1 for item in checks if item["status"] == "MANUAL")

if fail_count == 0 and manual_count == 0:
    status = "PASS"
    pass_canary = True
    reason = "canary window checks passed"
elif fail_count == 0 and manual_count > 0 and allow_conditional_manual:
    status = "CONDITIONAL_MANUAL"
    pass_canary = True
    reason = "canary checks passed with manual metric acceptance"
elif fail_count == 0 and manual_count > 0:
    status = "FAIL"
    pass_canary = False
    reason = "canary checks missing metrics in strict mode"
else:
    status = "FAIL"
    pass_canary = False
    reason = "canary checks failed"

payload = {
    "generated_at_utc": datetime.now(timezone.utc).isoformat(),
    "allow_conditional_manual": allow_conditional_manual,
    "status": status,
    "pass_canary": pass_canary,
    "reason": reason,
    "inputs": {
        "canary_traffic_pct": traffic_pct_raw,
        "canary_duration_minutes": duration_minutes_raw,
        "canary_error_rate_pct": error_rate_raw,
        "canary_p99_ratio": p99_ratio_raw,
    },
    "checks": checks,
}

with open(output_path, "w", encoding="utf-8") as fh:
    json.dump(payload, fh, indent=2)
    fh.write("\n")

print(json.dumps(payload, indent=2))

if pass_canary:
    sys.exit(0)
sys.exit(1)
PY

echo "$OUTPUT_PATH"
