#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT_DIR"

if ! command -v tar >/dev/null 2>&1; then
  echo "tar is required to create phase handoff bundle" >&2
  exit 1
fi

if ! command -v python3 >/dev/null 2>&1; then
  echo "python3 is required to write handoff bundle metadata" >&2
  exit 1
fi

TIMESTAMP_UTC="$(date -u +%Y%m%dT%H%M%SZ)"
BUNDLE_DIR="${BUNDLE_DIR:-artifacts/deploy/handoff}"
BUNDLE_PATH="${BUNDLE_PATH:-${BUNDLE_DIR}/phase-handoff-${TIMESTAMP_UTC}.tar.gz}"
METADATA_PATH="${METADATA_PATH:-artifacts/deploy/phase_handoff_bundle.json}"

REQUIRED_FILES=(
  "artifacts/deploy/external_closeout_status.json"
  "artifacts/deploy/go_live_signoff_status.json"
  "artifacts/deploy/external_phase_transition_check.json"
  "artifacts/deploy/production_deployment_signoff_check.json"
  "artifacts/deploy/production_deployment_todo.md"
  "artifacts/deploy/production_post_deploy_validation.json"
  "artifacts/deploy/phase_closure_manifest.json"
  "docs/FINAL_VALIDATION_brevio_openclaw.md"
)

for required_file in "${REQUIRED_FILES[@]}"; do
  if [[ ! -f "$required_file" ]]; then
    echo "missing required artifact for handoff bundle: $required_file" >&2
    exit 1
  fi
done

mkdir -p "$BUNDLE_DIR"
mkdir -p "$(dirname "$METADATA_PATH")"

tar -czf "$BUNDLE_PATH" "${REQUIRED_FILES[@]}"

python3 - "$BUNDLE_PATH" "$METADATA_PATH" "$TIMESTAMP_UTC" <<'PY'
import json
import os
import sys

bundle_path, metadata_path, timestamp_utc = sys.argv[1:4]

payload = {
    "generated_at_utc": timestamp_utc,
    "bundle_path": bundle_path,
    "bundle_size_bytes": os.path.getsize(bundle_path),
    "included_artifacts": [
        "artifacts/deploy/external_closeout_status.json",
        "artifacts/deploy/go_live_signoff_status.json",
        "artifacts/deploy/external_phase_transition_check.json",
        "artifacts/deploy/production_deployment_signoff_check.json",
        "artifacts/deploy/production_deployment_todo.md",
        "artifacts/deploy/production_post_deploy_validation.json",
        "artifacts/deploy/phase_closure_manifest.json",
        "docs/FINAL_VALIDATION_brevio_openclaw.md",
    ],
}

with open(metadata_path, "w", encoding="utf-8") as fh:
    json.dump(payload, fh, indent=2)
    fh.write("\n")

print(json.dumps(payload, indent=2))
PY

echo "$BUNDLE_PATH"
