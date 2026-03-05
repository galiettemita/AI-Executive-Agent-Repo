#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT_DIR"

if ! command -v python3 >/dev/null 2>&1; then
  echo "python3 is required to check production post-deployment validation" >&2
  exit 1
fi

SIGNOFF_CHECK_PATH="${SIGNOFF_CHECK_PATH:-artifacts/deploy/production_deployment_signoff_check.json}"
OUTPUT_PATH="${OUTPUT_PATH:-artifacts/deploy/production_post_deploy_validation.json}"
ALLOW_CONDITIONAL_MANUAL="${ALLOW_CONDITIONAL_MANUAL:-1}"
ALLOW_CONDITIONAL_MANUAL_BOOL=false
if [[ "$ALLOW_CONDITIONAL_MANUAL" == "1" ]]; then
  ALLOW_CONDITIONAL_MANUAL_BOOL=true
fi

GATEWAY_BASE_URL="${GATEWAY_BASE_URL:-}"
BRAIN_BASE_URL="${BRAIN_BASE_URL:-}"
HANDS_BASE_URL="${HANDS_BASE_URL:-}"
CANARY_ERROR_RATE_PCT="${CANARY_ERROR_RATE_PCT:-}"
CANARY_P99_RATIO="${CANARY_P99_RATIO:-}"
SLO_WINDOW_MINUTES="${SLO_WINDOW_MINUTES:-60}"
SLO_P50_LATENCY_SECONDS="${SLO_P50_LATENCY_SECONDS:-}"
SLO_P99_LATENCY_SECONDS="${SLO_P99_LATENCY_SECONDS:-}"
SLO_SKILL_SUCCESS_RATE_PCT="${SLO_SKILL_SUCCESS_RATE_PCT:-}"
SLO_DELIVERY_SUCCESS_RATE_PCT="${SLO_DELIVERY_SUCCESS_RATE_PCT:-}"

if [[ ! -f "$SIGNOFF_CHECK_PATH" ]]; then
  echo "missing production deployment signoff artifact: $SIGNOFF_CHECK_PATH" >&2
  exit 1
fi

if ! command -v curl >/dev/null 2>&1; then
  echo "curl is required for endpoint health checks" >&2
  exit 1
fi

mkdir -p "$(dirname "$OUTPUT_PATH")"

check_endpoint() {
  local service_name="$1"
  local base_url="$2"
  local endpoint="$3"

  if [[ -z "$base_url" ]]; then
    printf '{"service":"%s","check":"%s","status":"manual","detail":"%s base URL not provided"}\n' "$service_name" "$endpoint" "$service_name"
    return 0
  fi

  local target="${base_url%/}${endpoint}"
  local http_code
  http_code="$(curl -sS -o /dev/null -w '%{http_code}' --max-time 8 "$target" || true)"

  if [[ "$http_code" == "200" ]]; then
    printf '{"service":"%s","check":"%s","status":"pass","detail":"HTTP 200"}\n' "$service_name" "$endpoint"
    return 0
  fi

  if [[ "$ALLOW_CONDITIONAL_MANUAL_BOOL" == true ]]; then
    printf '{"service":"%s","check":"%s","status":"manual","detail":"endpoint check unavailable or non-200 (http=%s)"}\n' "$service_name" "$endpoint" "$http_code"
    return 0
  fi

  printf '{"service":"%s","check":"%s","status":"fail","detail":"endpoint check failed (http=%s)"}\n' "$service_name" "$endpoint" "$http_code"
  return 0
}

ENDPOINT_RESULTS=()
ENDPOINT_RESULTS+=("$(check_endpoint "gateway" "$GATEWAY_BASE_URL" "/health")")
ENDPOINT_RESULTS+=("$(check_endpoint "gateway" "$GATEWAY_BASE_URL" "/health/deep")")
ENDPOINT_RESULTS+=("$(check_endpoint "brain" "$BRAIN_BASE_URL" "/health")")
ENDPOINT_RESULTS+=("$(check_endpoint "brain" "$BRAIN_BASE_URL" "/health/deep")")
ENDPOINT_RESULTS+=("$(check_endpoint "hands" "$HANDS_BASE_URL" "/health")")
ENDPOINT_RESULTS+=("$(check_endpoint "hands" "$HANDS_BASE_URL" "/health/deep")")

python3 - "$SIGNOFF_CHECK_PATH" "$OUTPUT_PATH" "$ALLOW_CONDITIONAL_MANUAL" "$CANARY_ERROR_RATE_PCT" "$CANARY_P99_RATIO" "$SLO_WINDOW_MINUTES" "$SLO_P50_LATENCY_SECONDS" "$SLO_P99_LATENCY_SECONDS" "$SLO_SKILL_SUCCESS_RATE_PCT" "$SLO_DELIVERY_SUCCESS_RATE_PCT" <<'PY' "${ENDPOINT_RESULTS[@]}"
import json
import sys
from datetime import datetime, timezone

signoff_path = sys.argv[1]
output_path = sys.argv[2]
allow_conditional_manual = sys.argv[3] == "1"
canary_error_rate_raw = sys.argv[4]
canary_p99_ratio_raw = sys.argv[5]
slo_window_raw = sys.argv[6]
slo_p50_raw = sys.argv[7]
slo_p99_raw = sys.argv[8]
slo_skill_success_raw = sys.argv[9]
slo_delivery_success_raw = sys.argv[10]
endpoint_json_rows = sys.argv[11:]

with open(signoff_path, "r", encoding="utf-8") as fh:
    signoff = json.load(fh)

signoff_pass = bool(signoff.get("pass_signoff", False))
blocking_conditions = []

if not signoff_pass:
    blocking_conditions.append("production deployment signoff gate did not pass")

results = []
for row in endpoint_json_rows:
    row = row.strip()
    if not row:
        continue
    results.append(json.loads(row))

canary_result = {
    "service": "canary",
    "check": "slo_window",
    "status": "manual",
    "detail": "CANARY_ERROR_RATE_PCT and CANARY_P99_RATIO not provided",
}
if canary_error_rate_raw and canary_p99_ratio_raw:
    try:
        error_rate = float(canary_error_rate_raw)
        p99_ratio = float(canary_p99_ratio_raw)
        if error_rate <= 1.0 and p99_ratio <= 2.0:
            canary_result = {
                "service": "canary",
                "check": "slo_window",
                "status": "pass",
                "detail": f"error_rate_pct={error_rate}, p99_ratio={p99_ratio}",
            }
        else:
            canary_result = {
                "service": "canary",
                "check": "slo_window",
                "status": "fail",
                "detail": f"SLO breach: error_rate_pct={error_rate}, p99_ratio={p99_ratio}",
            }
    except ValueError:
        canary_result = {
            "service": "canary",
            "check": "slo_window",
            "status": "fail",
            "detail": "invalid canary metrics format",
        }

results.append(canary_result)

slo_result = {
    "service": "slo",
    "check": "slo_window_1h",
    "status": "manual",
    "detail": "SLO metrics not fully provided",
}

slo_metric_fields = [
    slo_p50_raw,
    slo_p99_raw,
    slo_skill_success_raw,
    slo_delivery_success_raw,
]

if any(slo_metric_fields) and not all([slo_window_raw] + slo_metric_fields):
    slo_result = {
        "service": "slo",
        "check": "slo_window_1h",
        "status": "fail",
        "detail": (
            "incomplete SLO metrics; required SLO_WINDOW_MINUTES, "
            "SLO_P50_LATENCY_SECONDS, SLO_P99_LATENCY_SECONDS, "
            "SLO_SKILL_SUCCESS_RATE_PCT, SLO_DELIVERY_SUCCESS_RATE_PCT"
        ),
    }
elif all(slo_metric_fields):
    try:
        slo_window_minutes = int(slo_window_raw)
        slo_p50 = float(slo_p50_raw)
        slo_p99 = float(slo_p99_raw)
        slo_skill_success = float(slo_skill_success_raw)
        slo_delivery_success = float(slo_delivery_success_raw)

        if slo_window_minutes < 60:
            slo_result = {
                "service": "slo",
                "check": "slo_window_1h",
                "status": "fail",
                "detail": f"SLO window too short: {slo_window_minutes}m (<60m required)",
            }
        elif (
            slo_p50 < 2.0
            and slo_p99 < 10.0
            and slo_skill_success > 95.0
            and slo_delivery_success > 99.5
        ):
            slo_result = {
                "service": "slo",
                "check": "slo_window_1h",
                "status": "pass",
                "detail": (
                    f"window={slo_window_minutes}m, p50_s={slo_p50}, "
                    f"p99_s={slo_p99}, skill_success_pct={slo_skill_success}, "
                    f"delivery_success_pct={slo_delivery_success}"
                ),
            }
        else:
            slo_result = {
                "service": "slo",
                "check": "slo_window_1h",
                "status": "fail",
                "detail": (
                    "SLO breach: "
                    f"window={slo_window_minutes}m, p50_s={slo_p50}, "
                    f"p99_s={slo_p99}, skill_success_pct={slo_skill_success}, "
                    f"delivery_success_pct={slo_delivery_success}"
                ),
            }
    except ValueError:
        slo_result = {
            "service": "slo",
            "check": "slo_window_1h",
            "status": "fail",
            "detail": "invalid SLO metrics format",
        }

results.append(slo_result)

pass_count = sum(1 for item in results if item.get("status") == "pass")
manual_count = sum(1 for item in results if item.get("status") == "manual")
fail_count = sum(1 for item in results if item.get("status") == "fail")

if fail_count > 0:
    status = "BLOCKED"
    pass_validation = False
    reason = "Post-deploy validation failed. Rollback or remediation required."
elif manual_count > 0:
    if allow_conditional_manual:
        status = "CONDITIONAL_MANUAL"
        pass_validation = True
        reason = "Post-deploy validation has manual items accepted under conditional mode."
    else:
        status = "BLOCKED"
        pass_validation = False
        reason = "Post-deploy validation has unresolved manual items in strict mode."
else:
    status = "READY"
    pass_validation = True
    reason = "Post-deploy validation checks passed."

if not signoff_pass:
    status = "BLOCKED"
    pass_validation = False
    reason = "Production signoff gate has not passed."

payload = {
    "generated_at_utc": datetime.now(timezone.utc).isoformat(),
    "signoff_source": signoff_path,
    "signoff_pass": signoff_pass,
    "allow_conditional_manual": allow_conditional_manual,
    "status": status,
    "pass_validation": pass_validation,
    "reason": reason,
    "summary": {
        "total_checks": len(results),
        "passed": pass_count,
        "manual": manual_count,
        "failed": fail_count,
    },
    "results": results,
    "blocking_conditions": blocking_conditions,
    "next_phase": "deployment-complete" if pass_validation else "deployment-remediation",
}

with open(output_path, "w", encoding="utf-8") as fh:
    json.dump(payload, fh, indent=2)
    fh.write("\n")

print(json.dumps(payload, indent=2))

if pass_validation:
    sys.exit(0)
sys.exit(1)
PY
