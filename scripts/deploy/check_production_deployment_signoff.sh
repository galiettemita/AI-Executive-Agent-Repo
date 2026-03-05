#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT_DIR"

if ! command -v python3 >/dev/null 2>&1; then
  echo "python3 is required to check production deployment signoff readiness" >&2
  exit 1
fi

TRANSITION_PATH="${TRANSITION_PATH:-artifacts/deploy/external_phase_transition_check.json}"
SIGNOFF_PATH="${SIGNOFF_PATH:-artifacts/deploy/go_live_signoff_status.json}"
REGRESSION_PATH="${REGRESSION_PATH:-artifacts/deploy/external_closeout_regression_report.json}"
OUTPUT_PATH="${OUTPUT_PATH:-artifacts/deploy/production_deployment_signoff_check.json}"

for required_file in "$TRANSITION_PATH" "$SIGNOFF_PATH" "$REGRESSION_PATH"; do
  if [[ ! -f "$required_file" ]]; then
    echo "missing required artifact: $required_file" >&2
    exit 1
  fi
done

mkdir -p "$(dirname "$OUTPUT_PATH")"

python3 - "$TRANSITION_PATH" "$SIGNOFF_PATH" "$REGRESSION_PATH" "$OUTPUT_PATH" <<'PY'
import json
import sys
from datetime import datetime, timezone

transition_path, signoff_path, regression_path, output_path = sys.argv[1:5]

with open(transition_path, "r", encoding="utf-8") as fh:
    transition = json.load(fh)

with open(signoff_path, "r", encoding="utf-8") as fh:
    signoff = json.load(fh)

with open(regression_path, "r", encoding="utf-8") as fh:
    regression = json.load(fh)

transition_pass = bool(transition.get("pass_transition", False))
next_phase = str(transition.get("next_phase", ""))
allow_conditional_manual = bool(transition.get("allow_conditional_manual", False))
signoff_status = str(signoff.get("status", "UNKNOWN"))
required_failed = int(signoff.get("required_failed", 0))
required_manual = int(signoff.get("required_manual", 0))
manual_items = signoff.get("manual_required_items", [])
regression_status = str(regression.get("status", "UNKNOWN"))

blocking_conditions = []

if not transition_pass or next_phase != "production-deployment-signoff":
    blocking_conditions.append(
        "external phase transition gate has not passed for production-deployment-signoff"
    )

if regression_status != "PASS":
    blocking_conditions.append("external closeout regression report is not PASS")

if required_failed > 0:
    blocking_conditions.append("go-live signoff contains failed required items")

signoff_mode = "blocked"
if signoff_status == "READY":
    signoff_mode = "ready"
elif signoff_status == "CONDITIONAL_MANUAL" and allow_conditional_manual:
    signoff_mode = "conditional_manual_override"
else:
    blocking_conditions.append("go-live signoff status is not deployable for current transition mode")

pass_signoff = len(blocking_conditions) == 0
if pass_signoff:
    reason = "Production deployment signoff checks passed. Proceed with deployment runbook."
else:
    reason = "Production deployment signoff checks failed. Resolve blocking conditions first."

payload = {
    "generated_at_utc": datetime.now(timezone.utc).isoformat(),
    "transition_source": transition_path,
    "signoff_source": signoff_path,
    "regression_source": regression_path,
    "pass_signoff": pass_signoff,
    "reason": reason,
    "signoff_mode": signoff_mode,
    "checks": {
        "transition_pass": transition_pass,
        "transition_next_phase": next_phase,
        "regression_status": regression_status,
        "required_failed": required_failed,
        "required_manual": required_manual,
    },
    "manual_required_items": manual_items,
    "blocking_conditions": blocking_conditions,
    "next_phase": "production-deployment" if pass_signoff else "external-closeout",
}

with open(output_path, "w", encoding="utf-8") as fh:
    json.dump(payload, fh, indent=2)
    fh.write("\n")

print(json.dumps(payload, indent=2))

if pass_signoff:
    sys.exit(0)
sys.exit(1)
PY
