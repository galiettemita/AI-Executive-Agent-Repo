#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT_DIR"

if ! command -v python3 >/dev/null 2>&1; then
  echo "python3 is required to generate production deployment todo" >&2
  exit 1
fi

SIGNOFF_CHECK_PATH="${SIGNOFF_CHECK_PATH:-artifacts/deploy/production_deployment_signoff_check.json}"
OUTPUT_PATH="${OUTPUT_PATH:-artifacts/deploy/production_deployment_todo.md}"

if [[ ! -f "$SIGNOFF_CHECK_PATH" ]]; then
  echo "missing production deployment signoff artifact: $SIGNOFF_CHECK_PATH" >&2
  exit 1
fi

mkdir -p "$(dirname "$OUTPUT_PATH")"

python3 - "$SIGNOFF_CHECK_PATH" "$OUTPUT_PATH" <<'PY'
import json
import sys
from datetime import datetime, timezone

signoff_path, output_path = sys.argv[1:3]

with open(signoff_path, "r", encoding="utf-8") as fh:
    signoff = json.load(fh)

pass_signoff = bool(signoff.get("pass_signoff", False))
signoff_mode = str(signoff.get("signoff_mode", "unknown"))
manual_items = signoff.get("manual_required_items", [])
blocking = signoff.get("blocking_conditions", [])

lines = []
lines.append("# Production Deployment TODO")
lines.append("")
lines.append(f"Generated (UTC): {datetime.now(timezone.utc).isoformat()}")
lines.append(f"Source: `{signoff_path}`")
lines.append(f"Pass Signoff: `{pass_signoff}`")
lines.append(f"Signoff Mode: `{signoff_mode}`")
lines.append("")

if not pass_signoff:
    lines.append("## Status")
    lines.append("Deployment is blocked. Resolve blockers before starting production rollout.")
    lines.append("")
    lines.append("## Blocking Conditions")
    if blocking:
        for item in blocking:
            lines.append(f"- {item}")
    else:
        lines.append("- Unknown blocking condition (inspect signoff artifact).")
    lines.append("")
else:
    lines.append("## Execution Steps")
    lines.append("1. Confirm final gates are green: `make ci-full`.")
    lines.append("2. Deploy charts with rollout waits: `WAIT_FOR_ROLLOUT=true NAMESPACE=default bash scripts/deploy/helm_rollout.sh`.")
    lines.append("3. Run service health sweeps: `kubectl get pods -n default` and probe `/health` + `/health/deep` on gateway/brain/hands.")
    lines.append("4. Execute canary window (10% traffic for 15 minutes) and watch SLOs (error rate <= 1%, P99 <= 2x baseline).")
    lines.append("5. Promote to 100% only if canary is stable; otherwise execute rollback runbook immediately.")
    lines.append("6. Record deployment evidence in release ticket and attach artifact snapshots.")
    lines.append("")

lines.append("## Manual Required Items Snapshot")
if manual_items:
    for item in manual_items:
        item_id = str(item.get("id", "unknown"))
        detail = str(item.get("detail", ""))
        lines.append(f"- `{item_id}`: {detail}")
else:
    lines.append("- None")

with open(output_path, "w", encoding="utf-8") as fh:
    fh.write("\n".join(lines) + "\n")

print(output_path)

if pass_signoff:
    sys.exit(0)
sys.exit(1)
PY

echo "$OUTPUT_PATH"
