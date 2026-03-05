#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT_DIR"

if ! command -v python3 >/dev/null 2>&1; then
  echo "python3 is required to generate go-live signoff status" >&2
  exit 1
fi

EXTERNAL_STATUS_PATH="${EXTERNAL_STATUS_PATH:-artifacts/deploy/external_closeout_status.json}"
OUTPUT_PATH="${OUTPUT_PATH:-artifacts/deploy/go_live_signoff_status.json}"

if [[ ! -f "$EXTERNAL_STATUS_PATH" ]]; then
  echo "missing external status artifact: $EXTERNAL_STATUS_PATH" >&2
  exit 1
fi

mkdir -p "$(dirname "$OUTPUT_PATH")"

python3 - "$EXTERNAL_STATUS_PATH" "$OUTPUT_PATH" <<'PY'
import json
import subprocess
import sys
from datetime import datetime, timezone

external_path, output_path = sys.argv[1:3]

with open(external_path, "r", encoding="utf-8") as fh:
    external = json.load(fh)

required_failed = int(external.get("required_failed", 0))
required_manual = int(external.get("required_manual", 0))
required_passed = int(external.get("required_passed", 0))
required_total = int(external.get("required_total", 0))
manual_evidence_path = str(external.get("manual_evidence_path", "") or "")
manual_evidence_confirmed = int(external.get("manual_evidence_confirmed", 0))

if required_failed > 0:
    status = "BLOCKED"
elif required_manual > 0:
    status = "CONDITIONAL_MANUAL"
else:
    status = "READY"

manual_items = [
    {
        "id": r.get("id"),
        "detail": r.get("detail", ""),
    }
    for r in external.get("results", [])
    if r.get("required") and r.get("status") == "manual"
]

failed_items = [
    {
        "id": r.get("id"),
        "detail": r.get("detail", ""),
    }
    for r in external.get("results", [])
    if r.get("required") and r.get("status") == "fail"
]

try:
    git_head = (
        subprocess.check_output(["git", "rev-parse", "--short", "HEAD"], text=True, stderr=subprocess.DEVNULL)
        .strip()
    )
except Exception:
    git_head = "unknown"

payload = {
    "generated_at_utc": datetime.now(timezone.utc).isoformat(),
    "git_head": git_head,
    "status": status,
    "required_total": required_total,
    "required_passed": required_passed,
    "required_failed": required_failed,
    "required_manual": required_manual,
    "manual_evidence_path": manual_evidence_path,
    "manual_evidence_confirmed": manual_evidence_confirmed,
    "blocking_required_items": failed_items,
    "manual_required_items": manual_items,
    "external_status_source": external_path,
    "next_action": (
        "Resolve failed required items and rerun external-closeout-check."
        if status == "BLOCKED"
        else "Resolve manual required items in production-connected context and rerun external-closeout-check."
        if status == "CONDITIONAL_MANUAL"
        else "Proceed to production deployment sign-off."
    ),
}

with open(output_path, "w", encoding="utf-8") as fh:
    json.dump(payload, fh, indent=2)
    fh.write("\n")

print(json.dumps(payload, indent=2))
PY
