#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT_DIR"

if ! command -v python3 >/dev/null 2>&1; then
  echo "python3 is required to check external phase transition readiness" >&2
  exit 1
fi

SIGNOFF_PATH="${SIGNOFF_PATH:-artifacts/deploy/go_live_signoff_status.json}"
OUTPUT_PATH="${OUTPUT_PATH:-artifacts/deploy/external_phase_transition_check.json}"
ALLOW_CONDITIONAL_MANUAL="${ALLOW_CONDITIONAL_MANUAL:-0}"

if [[ ! -f "$SIGNOFF_PATH" ]]; then
  echo "missing go-live signoff artifact: $SIGNOFF_PATH" >&2
  exit 1
fi

mkdir -p "$(dirname "$OUTPUT_PATH")"

python3 - "$SIGNOFF_PATH" "$OUTPUT_PATH" "$ALLOW_CONDITIONAL_MANUAL" <<'PY'
import json
import sys
from datetime import datetime, timezone

signoff_path, output_path, allow_conditional_manual = sys.argv[1:4]
allow_conditional_manual = allow_conditional_manual == "1"

with open(signoff_path, "r", encoding="utf-8") as fh:
    signoff = json.load(fh)

status = str(signoff.get("status", "UNKNOWN"))
manual_items = signoff.get("manual_required_items", [])
failed_items = signoff.get("blocking_required_items", [])

if status == "READY":
    pass_transition = True
    reason = "External closeout complete; proceed to production deployment sign-off phase."
elif status == "CONDITIONAL_MANUAL" and allow_conditional_manual:
    pass_transition = True
    reason = "Conditional manual override enabled; proceed with explicit acceptance of pending manual items."
else:
    pass_transition = False
    reason = "External closeout not fully complete."

payload = {
    "generated_at_utc": datetime.now(timezone.utc).isoformat(),
    "signoff_source": signoff_path,
    "signoff_status": status,
    "allow_conditional_manual": allow_conditional_manual,
    "pass_transition": pass_transition,
    "reason": reason,
    "manual_required_items": manual_items,
    "blocking_required_items": failed_items,
    "next_phase": "production-deployment-signoff" if pass_transition else "external-closeout",
}

with open(output_path, "w", encoding="utf-8") as fh:
    json.dump(payload, fh, indent=2)
    fh.write("\n")

print(json.dumps(payload, indent=2))

if pass_transition:
    sys.exit(0)
sys.exit(1)
PY
