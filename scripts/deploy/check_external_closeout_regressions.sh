#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT_DIR"

if ! command -v python3 >/dev/null 2>&1; then
  echo "python3 is required to check external closeout regressions" >&2
  exit 1
fi

CURRENT_STATUS_PATH="${CURRENT_STATUS_PATH:-artifacts/deploy/external_closeout_status.json}"
SNAPSHOT_PATH="${SNAPSHOT_PATH:-artifacts/deploy/external_closeout_status.last.json}"
REPORT_PATH="${REPORT_PATH:-artifacts/deploy/external_closeout_regression_report.json}"
ALLOW_EXTERNAL_REGRESSIONS="${ALLOW_EXTERNAL_REGRESSIONS:-0}"

if [[ ! -f "$CURRENT_STATUS_PATH" ]]; then
  echo "missing current external closeout status: $CURRENT_STATUS_PATH" >&2
  exit 1
fi

mkdir -p "$(dirname "$SNAPSHOT_PATH")" "$(dirname "$REPORT_PATH")"

python3 - "$CURRENT_STATUS_PATH" "$SNAPSHOT_PATH" "$REPORT_PATH" "$ALLOW_EXTERNAL_REGRESSIONS" <<'PY'
import json
import os
import shutil
import sys
from datetime import datetime, timezone

current_path, snapshot_path, report_path, allow_regressions = sys.argv[1:5]
allow_regressions = allow_regressions == "1"

with open(current_path, "r", encoding="utf-8") as fh:
    current = json.load(fh)

status_rank = {"pass": 2, "manual": 1, "fail": 0}

def required_map(payload):
    result = {}
    for row in payload.get("results", []):
        if not row.get("required"):
            continue
        item_id = row.get("id")
        if not item_id:
            continue
        result[item_id] = {
            "status": row.get("status", ""),
            "detail": row.get("detail", ""),
        }
    return result

current_map = required_map(current)
regressions = []
improvements = []
baseline_exists = os.path.exists(snapshot_path)

if baseline_exists:
    with open(snapshot_path, "r", encoding="utf-8") as fh:
        previous = json.load(fh)
    previous_map = required_map(previous)
    for item_id, cur in current_map.items():
        prev = previous_map.get(item_id)
        if not prev:
            continue
        prev_status = prev.get("status", "")
        cur_status = cur.get("status", "")
        prev_rank = status_rank.get(prev_status, -1)
        cur_rank = status_rank.get(cur_status, -1)
        if cur_rank < prev_rank:
            regressions.append(
                {
                    "id": item_id,
                    "from": prev_status,
                    "to": cur_status,
                    "previous_detail": prev.get("detail", ""),
                    "current_detail": cur.get("detail", ""),
                }
            )
        elif cur_rank > prev_rank:
            improvements.append(
                {
                    "id": item_id,
                    "from": prev_status,
                    "to": cur_status,
                    "previous_detail": prev.get("detail", ""),
                    "current_detail": cur.get("detail", ""),
                }
            )

report = {
    "generated_at_utc": datetime.now(timezone.utc).isoformat(),
    "current_status_path": current_path,
    "snapshot_path": snapshot_path,
    "baseline_exists": baseline_exists,
    "required_total": int(current.get("required_total", 0)),
    "required_passed": int(current.get("required_passed", 0)),
    "required_failed": int(current.get("required_failed", 0)),
    "required_manual": int(current.get("required_manual", 0)),
    "regressions": regressions,
    "improvements": improvements,
    "status": "PASS" if not regressions else "REGRESSION_DETECTED",
}

with open(report_path, "w", encoding="utf-8") as fh:
    json.dump(report, fh, indent=2)
    fh.write("\n")

shutil.copyfile(current_path, snapshot_path)

print(json.dumps(report, indent=2))

if regressions and not allow_regressions:
    sys.exit(1)
PY
