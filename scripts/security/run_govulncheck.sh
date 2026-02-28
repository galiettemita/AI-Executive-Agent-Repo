#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT_DIR"

ALLOWLIST_FILE="scripts/security/govuln_allowlist.txt"
REPORT_DIR="artifacts/security"
REPORT_FILE="$REPORT_DIR/govulncheck_verbose.txt"

mkdir -p "$REPORT_DIR"

if ! command -v govulncheck >/dev/null 2>&1; then
  echo "[govulncheck] installing golang.org/x/vuln/cmd/govulncheck@v1.1.4"
  go install golang.org/x/vuln/cmd/govulncheck@v1.1.4
fi

echo "[govulncheck] scanning ./..."
set +e
govulncheck -show verbose ./... >"$REPORT_FILE" 2>&1
SCAN_EXIT=$?
set -e

mapfile -t FOUND_IDS < <(grep -E '^Vulnerability #[0-9]+:' "$REPORT_FILE" | awk '{print $3}' | tr -d ':' | sort -u)

if [[ ${#FOUND_IDS[@]} -eq 0 ]]; then
  echo "[govulncheck] no vulnerabilities found"
  exit 0
fi

echo "[govulncheck] found IDs: ${FOUND_IDS[*]}"

UNKNOWN_IDS=()
for id in "${FOUND_IDS[@]}"; do
  if ! grep -qx "$id" "$ALLOWLIST_FILE"; then
    UNKNOWN_IDS+=("$id")
  fi
done

if [[ ${#UNKNOWN_IDS[@]} -gt 0 ]]; then
  echo "[govulncheck] new/unallowlisted vulnerability IDs detected: ${UNKNOWN_IDS[*]}"
  echo "[govulncheck] full report: $REPORT_FILE"
  exit 1
fi

if [[ $SCAN_EXIT -ne 0 ]]; then
  echo "[govulncheck] only allowlisted vulnerabilities were reported"
  echo "[govulncheck] full report: $REPORT_FILE"
fi

exit 0
