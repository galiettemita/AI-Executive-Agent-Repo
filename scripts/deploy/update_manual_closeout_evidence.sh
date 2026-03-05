#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT_DIR"

if ! command -v python3 >/dev/null 2>&1; then
  echo "python3 is required to update manual closeout evidence" >&2
  exit 1
fi

ITEM_ID="${1:-}"
CONFIRMED_BY="${2:-}"
NOTE="${3:-}"
MANUAL_EVIDENCE_PATH="${MANUAL_EVIDENCE_PATH:-artifacts/deploy/manual_closeout_evidence.json}"
ITEM_CATALOG_PATH="${ITEM_CATALOG_PATH:-config/external-closeout-required-item-ids.txt}"

if [[ -z "$ITEM_ID" ]]; then
  echo "usage: update_manual_closeout_evidence.sh <item_id> <confirmed_by> [note]" >&2
  exit 1
fi

if [[ -z "$CONFIRMED_BY" ]]; then
  echo "confirmed_by is required" >&2
  exit 1
fi

if [[ ! -f "$ITEM_CATALOG_PATH" ]]; then
  echo "missing item catalog: $ITEM_CATALOG_PATH" >&2
  exit 1
fi

if ! grep -qx "$ITEM_ID" "$ITEM_CATALOG_PATH"; then
  echo "unsupported item_id: $ITEM_ID" >&2
  echo "allowed item_id values:" >&2
  cat "$ITEM_CATALOG_PATH" >&2
  exit 1
fi

mkdir -p "$(dirname "$MANUAL_EVIDENCE_PATH")"

python3 - "$MANUAL_EVIDENCE_PATH" "$ITEM_ID" "$CONFIRMED_BY" "$NOTE" <<'PY'
import json
import os
import sys
from datetime import datetime, timezone

path, item_id, confirmed_by, note = sys.argv[1:5]

payload = {"items": {}}
if os.path.exists(path):
    try:
        with open(path, "r", encoding="utf-8") as fh:
            data = json.load(fh)
            if isinstance(data, dict):
                payload = data
    except Exception:
        payload = {"items": {}}

items = payload.setdefault("items", {})
items[item_id] = {
    "confirmed": True,
    "confirmed_by": confirmed_by,
    "confirmed_at_utc": datetime.now(timezone.utc).isoformat(),
    "note": note,
}
events = payload.setdefault("events", [])
if not isinstance(events, list):
    events = []
    payload["events"] = events
events.append(
    {
        "item_id": item_id,
        "action": "confirm",
        "actor": confirmed_by,
        "at_utc": datetime.now(timezone.utc).isoformat(),
        "note": note,
    }
)
payload["updated_at_utc"] = datetime.now(timezone.utc).isoformat()

with open(path, "w", encoding="utf-8") as fh:
    json.dump(payload, fh, indent=2)
    fh.write("\n")

print(json.dumps({"item_id": item_id, "status": "confirmed", "path": path}, indent=2))
PY
