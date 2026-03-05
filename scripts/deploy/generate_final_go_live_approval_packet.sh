#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT_DIR"

if ! command -v python3 >/dev/null 2>&1; then
  echo "python3 is required to generate final go-live approval packet" >&2
  exit 1
fi

MANIFEST_PATH="${MANIFEST_PATH:-artifacts/deploy/phase_closure_manifest.json}"
PHASE_STATUS_PATH="${PHASE_STATUS_PATH:-artifacts/deploy/phase_status.txt}"
HANDOFF_BUNDLE_PATH="${HANDOFF_BUNDLE_PATH:-artifacts/deploy/phase_handoff_bundle.json}"
OUTPUT_JSON="${OUTPUT_JSON:-artifacts/deploy/final_go_live_approval_packet.json}"
OUTPUT_MD="${OUTPUT_MD:-artifacts/deploy/final_go_live_approval_packet.md}"

if [[ ! -f "$MANIFEST_PATH" ]]; then
  echo "missing phase closure manifest: $MANIFEST_PATH" >&2
  exit 1
fi

if [[ ! -f "$HANDOFF_BUNDLE_PATH" ]]; then
  echo "missing phase handoff bundle metadata: $HANDOFF_BUNDLE_PATH" >&2
  exit 1
fi

mkdir -p "$(dirname "$OUTPUT_JSON")"
mkdir -p "$(dirname "$OUTPUT_MD")"

python3 - "$MANIFEST_PATH" "$PHASE_STATUS_PATH" "$HANDOFF_BUNDLE_PATH" "$OUTPUT_JSON" "$OUTPUT_MD" <<'PY'
import json
import subprocess
import sys
from datetime import datetime, timezone
from pathlib import Path

manifest_path = Path(sys.argv[1])
phase_status_path = Path(sys.argv[2])
handoff_path = Path(sys.argv[3])
output_json_path = Path(sys.argv[4])
output_md_path = Path(sys.argv[5])

manifest = json.loads(manifest_path.read_text(encoding="utf-8"))
handoff = json.loads(handoff_path.read_text(encoding="utf-8"))

summary = manifest.get("summary", {})
overall_status = str(summary.get("overall_status", "UNKNOWN"))
required_failed = int(summary.get("required_failed", 0))
required_manual = int(summary.get("required_manual", 0))
transition_pass = bool(summary.get("transition_pass", False))
production_signoff_pass = bool(summary.get("production_signoff_pass", False))
canary_pass = bool(summary.get("canary_pass", False))
post_deploy_pass = bool(summary.get("post_deploy_pass", False))

try:
    git_head = (
        subprocess.check_output(["git", "rev-parse", "--short", "HEAD"], text=True, stderr=subprocess.DEVNULL)
        .strip()
    )
except Exception:
    git_head = "unknown"

phase_status_text = ""
if phase_status_path.exists():
    phase_status_text = phase_status_path.read_text(encoding="utf-8")

handoff_bundle = str(handoff.get("bundle_path", ""))
handoff_generated_at = str(handoff.get("generated_at_utc", ""))

final_approvals = [
    {"role": "Release Manager", "status": "PENDING"},
    {"role": "Engineering Lead", "status": "PENDING"},
    {"role": "Security Lead", "status": "PENDING"},
    {"role": "Product Owner", "status": "PENDING"},
]

ready_for_approval = (
    overall_status == "READY"
    and required_failed == 0
    and required_manual == 0
    and transition_pass
    and production_signoff_pass
    and canary_pass
    and post_deploy_pass
)

packet = {
    "generated_at_utc": datetime.now(timezone.utc).isoformat(),
    "git_head": git_head,
    "ready_for_approval": ready_for_approval,
    "overall_status": overall_status,
    "summary": {
        "required_failed": required_failed,
        "required_manual": required_manual,
        "transition_pass": transition_pass,
        "production_signoff_pass": production_signoff_pass,
        "canary_pass": canary_pass,
        "post_deploy_pass": post_deploy_pass,
    },
    "artifact_sources": {
        "manifest": str(manifest_path),
        "phase_status": str(phase_status_path),
        "handoff_bundle_metadata": str(handoff_path),
        "handoff_bundle_tarball": handoff_bundle,
    },
    "approvals_required": final_approvals,
    "next_action": (
        "Collect final human go-live approvals and execute production go-live."
        if ready_for_approval
        else "Resolve outstanding blockers before final go-live approval."
    ),
}

output_json_path.write_text(json.dumps(packet, indent=2) + "\n", encoding="utf-8")

md_lines = [
    "# Final Go-Live Approval Packet",
    "",
    f"- Generated (UTC): {packet['generated_at_utc']}",
    f"- Git Head: `{git_head}`",
    f"- Overall Status: `{overall_status}`",
    f"- Ready For Approval: `{str(ready_for_approval).lower()}`",
    "",
    "## Gate Summary",
    "",
    f"- Required failed: `{required_failed}`",
    f"- Required manual: `{required_manual}`",
    f"- Transition pass: `{transition_pass}`",
    f"- Production signoff pass: `{production_signoff_pass}`",
    f"- Canary pass: `{canary_pass}`",
    f"- Post-deploy pass: `{post_deploy_pass}`",
    "",
    "## Approval Signoff",
    "",
]

for entry in final_approvals:
    md_lines.append(f"- [ ] {entry['role']} approval")

md_lines.extend(
    [
        "",
        "## Handoff Artifacts",
        "",
        f"- Manifest: `{manifest_path}`",
        f"- Phase status: `{phase_status_path}`",
        f"- Handoff metadata: `{handoff_path}`",
        f"- Handoff tarball: `{handoff_bundle}`",
        f"- Handoff generated (UTC): `{handoff_generated_at}`",
        "",
        "## Next Action",
        "",
        f"{packet['next_action']}",
    ]
)

if phase_status_text:
    md_lines.extend(["", "## Phase Status Snapshot", "", "```text", phase_status_text.rstrip("\n"), "```"])

output_md_path.write_text("\n".join(md_lines) + "\n", encoding="utf-8")

print(json.dumps(packet, indent=2))

if ready_for_approval:
    sys.exit(0)
sys.exit(1)
PY

echo "$OUTPUT_JSON"
echo "$OUTPUT_MD"
