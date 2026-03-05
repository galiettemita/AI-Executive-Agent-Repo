#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT_DIR"

if ! command -v python3 >/dev/null 2>&1; then
  echo "python3 is required to generate manual closeout TODO" >&2
  exit 1
fi

SIGNOFF_PATH="${SIGNOFF_PATH:-artifacts/deploy/go_live_signoff_status.json}"
OUTPUT_PATH="${OUTPUT_PATH:-artifacts/deploy/manual_closeout_todo.md}"

if [[ ! -f "$SIGNOFF_PATH" ]]; then
  echo "missing go-live signoff artifact: $SIGNOFF_PATH" >&2
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

status = signoff.get("status", "UNKNOWN")
git_head = signoff.get("git_head", "unknown")
manual_items = signoff.get("manual_required_items", [])
blocked_items = signoff.get("blocking_required_items", [])
next_action = signoff.get("next_action", "Review signoff artifact and continue per runbook.")
manual_evidence_path = signoff.get("manual_evidence_path", "")
manual_evidence_confirmed = int(signoff.get("manual_evidence_confirmed", 0))

section_map = {
    "partner_applications_submitted": "Section 1",
    "plaid_secret_prod": "Section 2",
    "plaid_webhook_secret": "Section 2",
    "stripe_billing_keys": "Section 3",
    "unstructured_api_key": "Section 4",
    "pagerduty_routing_key": "Section 5",
    "analytics_event_bus": "Section 6",
    "remote_catalog_signing_keys": "Section 7",
}

lines = []
lines.append("# Manual Closeout TODO")
lines.append("")
lines.append(f"- Generated at (UTC): {datetime.now(timezone.utc).isoformat()}")
lines.append(f"- Git head: `{git_head}`")
lines.append(f"- Signoff status: `{status}`")
lines.append(f"- Source artifact: `{signoff_path}`")
if manual_evidence_path:
    lines.append(f"- Manual evidence file: `{manual_evidence_path}`")
lines.append(f"- Manual evidence confirmed items: `{manual_evidence_confirmed}`")
lines.append("")
lines.append("Runbook reference:")
lines.append("- `docs/EXTERNAL_CLOSEOUT.md`")
lines.append("")

if blocked_items:
    lines.append("## Blocking Required Items")
    lines.append("")
    for item in blocked_items:
        item_id = item.get("id", "unknown")
        detail = item.get("detail", "")
        section = section_map.get(item_id, "See runbook")
        lines.append(f"- [ ] `{item_id}` ({section})")
        lines.append(f"  - Detail: {detail}")
    lines.append("")

if manual_items:
    lines.append("## Manual Required Items")
    lines.append("")
    for item in manual_items:
        item_id = item.get("id", "unknown")
        detail = item.get("detail", "")
        section = section_map.get(item_id, "See runbook")
        lines.append(f"- [ ] `{item_id}` ({section})")
        lines.append(f"  - Detail: {detail}")
        lines.append(
            f"  - Confirm command: `make manual-closeout-confirm ITEM_ID={item_id} CONFIRMED_BY=<name> NOTE=\"<evidence>\"`"
        )
        lines.append(
            f"  - Revoke command: `make manual-closeout-unconfirm ITEM_ID={item_id} REVOKED_BY=<name> NOTE=\"<reason>\"`"
        )
    lines.append("")

if not manual_items and not blocked_items:
    lines.append("## Status")
    lines.append("")
    lines.append("- No pending required manual/blocking items. Go-live manual closeout is complete.")
    lines.append("")

lines.append("## Next Action")
lines.append("")
lines.append(f"- {next_action}")
lines.append("- After updates, rerun:")
lines.append("  - `make external-phase-sync`")
lines.append("")

with open(output_path, "w", encoding="utf-8") as fh:
    fh.write("\n".join(lines))

print(output_path)
PY
