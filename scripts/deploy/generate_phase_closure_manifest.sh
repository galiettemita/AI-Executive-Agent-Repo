#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT_DIR"

if ! command -v python3 >/dev/null 2>&1; then
  echo "python3 is required to generate phase closure manifest" >&2
  exit 1
fi

EXTERNAL_STATUS_PATH="${EXTERNAL_STATUS_PATH:-artifacts/deploy/external_closeout_status.json}"
GO_LIVE_SIGNOFF_PATH="${GO_LIVE_SIGNOFF_PATH:-artifacts/deploy/go_live_signoff_status.json}"
TRANSITION_PATH="${TRANSITION_PATH:-artifacts/deploy/external_phase_transition_check.json}"
PROD_SIGNOFF_PATH="${PROD_SIGNOFF_PATH:-artifacts/deploy/production_deployment_signoff_check.json}"
CANARY_PATH="${CANARY_PATH:-artifacts/deploy/production_canary_check.json}"
POST_DEPLOY_PATH="${POST_DEPLOY_PATH:-artifacts/deploy/production_post_deploy_validation.json}"
REGRESSION_PATH="${REGRESSION_PATH:-artifacts/deploy/external_closeout_regression_report.json}"
OUTPUT_PATH="${OUTPUT_PATH:-artifacts/deploy/phase_closure_manifest.json}"

for required_file in \
  "$EXTERNAL_STATUS_PATH" \
  "$GO_LIVE_SIGNOFF_PATH" \
  "$TRANSITION_PATH" \
  "$PROD_SIGNOFF_PATH" \
  "$CANARY_PATH" \
  "$POST_DEPLOY_PATH" \
  "$REGRESSION_PATH"; do
  if [[ ! -f "$required_file" ]]; then
    echo "missing required artifact: $required_file" >&2
    exit 1
  fi
done

mkdir -p "$(dirname "$OUTPUT_PATH")"

python3 - "$EXTERNAL_STATUS_PATH" "$GO_LIVE_SIGNOFF_PATH" "$TRANSITION_PATH" "$PROD_SIGNOFF_PATH" "$CANARY_PATH" "$POST_DEPLOY_PATH" "$REGRESSION_PATH" "$OUTPUT_PATH" <<'PY'
import json
import sys
from datetime import datetime, timezone

(
    external_status_path,
    go_live_signoff_path,
    transition_path,
    prod_signoff_path,
    canary_path,
    post_deploy_path,
    regression_path,
    output_path,
) = sys.argv[1:9]


def load_json(path: str):
    with open(path, "r", encoding="utf-8") as fh:
        return json.load(fh)

external_status = load_json(external_status_path)
go_live_signoff = load_json(go_live_signoff_path)
transition = load_json(transition_path)
prod_signoff = load_json(prod_signoff_path)
canary = load_json(canary_path)
post_deploy = load_json(post_deploy_path)
regression = load_json(regression_path)

required_failed = int(external_status.get("required_failed", 0))
required_manual = int(external_status.get("required_manual", 0))
regression_status = str(regression.get("status", "UNKNOWN"))
transition_pass = bool(transition.get("pass_transition", False))
prod_signoff_pass = bool(prod_signoff.get("pass_signoff", False))
canary_pass = bool(canary.get("pass_canary", False))
canary_status = str(canary.get("status", "UNKNOWN"))
post_deploy_pass = bool(post_deploy.get("pass_validation", False))
post_deploy_status = str(post_deploy.get("status", "UNKNOWN"))

overall_status = "READY"
if required_failed > 0 or regression_status != "PASS" or not transition_pass or not prod_signoff_pass or not canary_pass or not post_deploy_pass:
    overall_status = "BLOCKED"
elif required_manual > 0 or canary_status == "CONDITIONAL_MANUAL" or post_deploy_status == "CONDITIONAL_MANUAL":
    overall_status = "CONDITIONAL_MANUAL"

payload = {
    "generated_at_utc": datetime.now(timezone.utc).isoformat(),
    "sources": {
        "external_status": external_status_path,
        "go_live_signoff": go_live_signoff_path,
        "transition": transition_path,
        "production_signoff": prod_signoff_path,
        "canary": canary_path,
        "post_deploy": post_deploy_path,
        "regression": regression_path,
    },
    "summary": {
        "overall_status": overall_status,
        "required_failed": required_failed,
        "required_manual": required_manual,
        "regression_status": regression_status,
        "transition_pass": transition_pass,
        "production_signoff_pass": prod_signoff_pass,
        "canary_pass": canary_pass,
        "canary_status": canary_status,
        "post_deploy_pass": post_deploy_pass,
        "post_deploy_status": post_deploy_status,
    },
    "artifacts": {
        "external_status": external_status,
        "go_live_signoff": go_live_signoff,
        "transition": transition,
        "production_signoff": prod_signoff,
        "canary": canary,
        "post_deploy": post_deploy,
        "regression": regression,
    },
    "next_action": (
        "Proceed with final go-live approval and operational handoff."
        if overall_status == "READY"
        else "Resolve blocking/conditional items and regenerate manifest."
    ),
}

with open(output_path, "w", encoding="utf-8") as fh:
    json.dump(payload, fh, indent=2)
    fh.write("\n")

print(json.dumps(payload["summary"], indent=2))
print(output_path)
PY

echo "$OUTPUT_PATH"
