#!/usr/bin/env bash
set -euo pipefail

# Minimal deterministic CI gate.
# Runs: scan_markers, scan_console_log, go test, go vet, pnpm test/build (if available).
# Exit 0 = all gates pass, Exit 1 = any gate fails.

REPO_ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"

FAILURES=0
RESULTS=""

run_gate() {
  local name="$1"
  shift
  echo "=== GATE: $name ==="
  if "$@" 2>&1; then
    RESULTS="$RESULTS\n  PASS: $name"
  else
    RESULTS="$RESULTS\n  FAIL: $name"
    FAILURES=$((FAILURES + 1))
  fi
  echo ""
}

cd "$REPO_ROOT"

run_gate "scan_markers" bash "$SCRIPT_DIR/scan_markers.sh"
run_gate "scan_console_log" bash "$SCRIPT_DIR/scan_console_log.sh"

if command -v go &>/dev/null; then
  run_gate "go_test" go test ./...
  run_gate "go_vet" go vet ./...
else
  echo "go not available; skipping Go gates."
  RESULTS="$RESULTS\n  SKIP: go_test (go not available)"
  RESULTS="$RESULTS\n  SKIP: go_vet (go not available)"
fi

if [ -f "$REPO_ROOT/pnpm-lock.yaml" ] && command -v pnpm &>/dev/null; then
  run_gate "pnpm_test" pnpm -r test
  run_gate "pnpm_build" pnpm -r build
else
  echo "pnpm or pnpm-lock.yaml not available; skipping pnpm gates."
  RESULTS="$RESULTS\n  SKIP: pnpm_test"
  RESULTS="$RESULTS\n  SKIP: pnpm_build"
fi

echo ""
echo "=== CI GATE SUMMARY ==="
echo -e "$RESULTS"
echo ""

if [ "$FAILURES" -gt 0 ]; then
  echo "CI GATE RESULT: FAILED ($FAILURES gate(s) failed)"
  exit 1
else
  echo "CI GATE RESULT: PASSED"
  exit 0
fi
