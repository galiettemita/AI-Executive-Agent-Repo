#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT_DIR"

if ! command -v python3 >/dev/null 2>&1; then
  echo "python3 is required to confirm final go-live approvals" >&2
  exit 1
fi

ROLE="${1:-}"
APPROVED_BY="${2:-}"
NOTE="${3:-}"

if [[ -z "$ROLE" ]]; then
  echo "usage: confirm_final_go_live_approval.sh <ROLE> <APPROVED_BY> [NOTE]" >&2
  exit 1
fi

if [[ -z "$APPROVED_BY" ]]; then
  echo "APPROVED_BY is required" >&2
  exit 1
fi

OUTPUT_JSON="${OUTPUT_JSON:-artifacts/deploy/final_go_live_approval_packet.json}"
OUTPUT_MD="${OUTPUT_MD:-artifacts/deploy/final_go_live_approval_packet.md}"

if [[ ! -f "$OUTPUT_JSON" ]]; then
  echo "missing approval packet json: $OUTPUT_JSON" >&2
  echo "run: make go-live-approval-packet" >&2
  exit 1
fi

python3 - "$OUTPUT_JSON" "$ROLE" "$APPROVED_BY" "$NOTE" <<'PY'
import json
import sys
from datetime import datetime, timezone

path, role, approved_by, note = sys.argv[1:5]

with open(path, "r", encoding="utf-8") as fh:
    payload = json.load(fh)

approvals = payload.get("approvals_required")
if not isinstance(approvals, list):
    print("invalid approval packet format: approvals_required missing", file=sys.stderr)
    sys.exit(1)

role_norm = role.strip().lower()
matched = False
for entry in approvals:
    if not isinstance(entry, dict):
        continue
    entry_role = str(entry.get("role", "")).strip()
    if entry_role.lower() != role_norm:
        continue
    entry["status"] = "APPROVED"
    entry["approved_by"] = approved_by.strip()
    entry["approved_at_utc"] = datetime.now(timezone.utc).isoformat()
    if note.strip():
        entry["note"] = note.strip()
    matched = True
    break

if not matched:
    print(f"role not found in packet: {role}", file=sys.stderr)
    sys.exit(1)

with open(path, "w", encoding="utf-8") as fh:
    json.dump(payload, fh, indent=2)
    fh.write("\n")

print(
    json.dumps(
        {
            "role": role,
            "status": "APPROVED",
            "approved_by": approved_by,
            "path": path,
        },
        indent=2,
    )
)
PY

OUTPUT_JSON="$OUTPUT_JSON" OUTPUT_MD="$OUTPUT_MD" bash scripts/deploy/generate_final_go_live_approval_packet.sh >/dev/null

echo "$OUTPUT_JSON"
echo "$OUTPUT_MD"
