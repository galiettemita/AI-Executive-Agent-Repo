#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT_DIR"

if ! command -v python3 >/dev/null 2>&1; then
  echo "python3 is required to generate manual closeout batch commands" >&2
  exit 1
fi

SIGNOFF_PATH="${SIGNOFF_PATH:-artifacts/deploy/go_live_signoff_status.json}"
OUTPUT_PATH="${OUTPUT_PATH:-artifacts/deploy/manual_closeout_batch_commands.sh}"

if [[ ! -f "$SIGNOFF_PATH" ]]; then
  echo "missing signoff artifact: $SIGNOFF_PATH" >&2
  exit 1
fi

mkdir -p "$(dirname "$OUTPUT_PATH")"

python3 - "$SIGNOFF_PATH" "$OUTPUT_PATH" <<'PY'
import json
import sys
from datetime import datetime, timezone

signoff_path, output_path = sys.argv[1:3]

with open(signoff_path, "r", encoding="utf-8") as fh:
    signoff = json.load(fh)

manual_items = signoff.get("manual_required_items", [])

lines = []
lines.append("#!/usr/bin/env bash")
lines.append("set -euo pipefail")
lines.append("")
lines.append(f"# generated_at_utc={datetime.now(timezone.utc).isoformat()}")
lines.append(f"# signoff_source={signoff_path}")
lines.append("# usage:")
lines.append("#   ./artifacts/deploy/manual_closeout_batch_commands.sh <actor-name>")
lines.append("# example:")
lines.append("#   ./artifacts/deploy/manual_closeout_batch_commands.sh ops")
lines.append("")
lines.append('if [[ $# -lt 1 ]]; then')
lines.append('  echo "usage: $0 <actor-name>" >&2')
lines.append("  exit 1")
lines.append("fi")
lines.append('ACTOR="$1"')
lines.append("")
lines.append('ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"')
lines.append('cd "$ROOT_DIR"')
lines.append("")

if not manual_items:
    lines.append('echo "No manual required items found in signoff artifact."')
else:
    lines.append('echo "Applying manual confirmations for pending required items..."')
    for item in manual_items:
        item_id = str(item.get("id", "")).strip()
        if not item_id:
            continue
        lines.append(
            f'make manual-closeout-confirm ITEM_ID={item_id} CONFIRMED_BY="$ACTOR" NOTE="manual-closeout batch confirmation"'
        )
    lines.append('echo "Regenerating phase artifacts..."')
    lines.append("make external-phase-sync")

with open(output_path, "w", encoding="utf-8") as fh:
    fh.write("\n".join(lines) + "\n")

print(output_path)
PY

chmod +x "$OUTPUT_PATH"
echo "$OUTPUT_PATH"
