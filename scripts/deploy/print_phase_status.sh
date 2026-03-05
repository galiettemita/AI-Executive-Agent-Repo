#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT_DIR"

if ! command -v python3 >/dev/null 2>&1; then
  echo "python3 is required to print phase status" >&2
  exit 1
fi

MANIFEST_PATH="${MANIFEST_PATH:-artifacts/deploy/phase_closure_manifest.json}"
HANDOFF_PATH="${HANDOFF_PATH:-artifacts/deploy/phase_handoff_bundle.json}"
OUTPUT_PATH="${OUTPUT_PATH:-artifacts/deploy/phase_status.txt}"

if [[ ! -f "$MANIFEST_PATH" ]]; then
  echo "missing phase closure manifest: $MANIFEST_PATH" >&2
  exit 1
fi

mkdir -p "$(dirname "$OUTPUT_PATH")"

python3 - "$MANIFEST_PATH" "$HANDOFF_PATH" "$OUTPUT_PATH" <<'PY'
import json
import os
import sys

manifest_path, handoff_path, output_path = sys.argv[1:4]

with open(manifest_path, "r", encoding="utf-8") as fh:
    manifest = json.load(fh)

summary = manifest.get("summary", {})
overall_status = str(summary.get("overall_status", "UNKNOWN"))
required_manual = int(summary.get("required_manual", 0))
required_failed = int(summary.get("required_failed", 0))
transition_pass = bool(summary.get("transition_pass", False))
prod_signoff_pass = bool(summary.get("production_signoff_pass", False))
canary_pass = bool(summary.get("canary_pass", False))
canary_status = str(summary.get("canary_status", "UNKNOWN"))
post_deploy_status = str(summary.get("post_deploy_status", "UNKNOWN"))

handoff_bundle = None
if os.path.isfile(handoff_path):
    with open(handoff_path, "r", encoding="utf-8") as fh:
        handoff_bundle = json.load(fh)

lines = []
lines.append("BREVIO Phase Status")
lines.append("===================")
lines.append(f"overall_status: {overall_status}")
lines.append(f"required_failed: {required_failed}")
lines.append(f"required_manual: {required_manual}")
lines.append(f"transition_pass: {transition_pass}")
lines.append(f"production_signoff_pass: {prod_signoff_pass}")
lines.append(f"canary_pass: {canary_pass}")
lines.append(f"canary_status: {canary_status}")
lines.append(f"post_deploy_status: {post_deploy_status}")

if handoff_bundle:
    lines.append(f"handoff_bundle: {handoff_bundle.get('bundle_path', 'unknown')}")
    lines.append(f"handoff_generated_at_utc: {handoff_bundle.get('generated_at_utc', 'unknown')}")

lines.append("")
if overall_status == "READY":
    lines.append("next_action: proceed with final go-live approval")
elif overall_status == "CONDITIONAL_MANUAL":
    lines.append("next_action: complete manual external confirmations and rerun phase sync")
else:
    lines.append("next_action: resolve blocking failures before progression")

report = "\n".join(lines) + "\n"
with open(output_path, "w", encoding="utf-8") as fh:
    fh.write(report)

print(report)
print(output_path)

if overall_status == "BLOCKED":
    sys.exit(1)
sys.exit(0)
PY
