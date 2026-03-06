#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT_DIR"

ALLOWLIST_FILE="scripts/security/govuln_allowlist.txt"
REPORT_DIR="artifacts/security"
REPORT_FILE="$REPORT_DIR/govulncheck_verbose.txt"

mkdir -p "$REPORT_DIR"

resolve_docker_bin() {
  if [[ -x "/Applications/Docker.app/Contents/Resources/bin/docker" ]]; then
    echo "/Applications/Docker.app/Contents/Resources/bin/docker"
    return 0
  fi
  if command -v docker >/dev/null 2>&1; then
    command -v docker
    return 0
  fi
  return 1
}

echo "[govulncheck] scanning ./..."
SCAN_EXIT=0
if command -v govulncheck >/dev/null 2>&1; then
  set +e
  govulncheck -show verbose ./... >"$REPORT_FILE" 2>&1
  SCAN_EXIT=$?
  set -e
elif command -v go >/dev/null 2>&1; then
  echo "[govulncheck] installing golang.org/x/vuln/cmd/govulncheck@v1.1.4"
  go install golang.org/x/vuln/cmd/govulncheck@v1.1.4
  set +e
  govulncheck -show verbose ./... >"$REPORT_FILE" 2>&1
  SCAN_EXIT=$?
  set -e
else
  docker_bin="$(resolve_docker_bin || true)"
  if [[ -z "${docker_bin}" ]]; then
    echo "[govulncheck] neither govulncheck/go nor docker is available"
    exit 1
  fi
  echo "[govulncheck] go toolchain unavailable; using dockerized go1.23 scanner"
  set +e
  "$docker_bin" run --rm -v "$ROOT_DIR":/src -w /src golang:1.23 sh -lc \
    'export PATH="/usr/local/go/bin:/go/bin:$PATH"; go install golang.org/x/vuln/cmd/govulncheck@v1.1.4 && /go/bin/govulncheck -show verbose ./...' \
    >"$REPORT_FILE" 2>&1
  SCAN_EXIT=$?
  set -e
fi

FOUND_IDS=()
while IFS= read -r id; do
  if [[ -n "$id" ]]; then
    FOUND_IDS+=("$id")
  fi
done < <(grep -E '^Vulnerability #[0-9]+:' "$REPORT_FILE" | awk '{print $3}' | tr -d ':' | sort -u)

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
