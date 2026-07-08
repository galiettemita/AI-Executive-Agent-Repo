#!/bin/bash
set -u

# Brevio GitHub auth preflight.
# Prints sanitized diagnostics and a final classification:
#   LOCAL_GITHUB_AUTH_OK
#   LOCAL_GITHUB_AUTH_FAIL_MCP_OK
#   GITHUB_WRITE_BLOCKED
#
# Defaults to the durable Brevio operator HOME/PATH instead of the current
# Hermes profile HOME, because launchd/brevio-cycle.sh runs with HOME=/Users/galiettemita.

BREVIO_GITHUB_HOME="${BREVIO_GITHUB_HOME:-/Users/galiettemita}"
export HOME="$BREVIO_GITHUB_HOME"
export USER="${USER:-galiettemita}"
export LOGNAME="${LOGNAME:-galiettemita}"
export PATH="/Users/galiettemita/.hermes/profiles/brevio-project-manager/bin:/Users/galiettemita/.nvm/versions/node/v24.13.0/bin:/opt/homebrew/bin:/usr/local/bin:/usr/bin:/bin:/usr/sbin:/sbin"

sanitize() {
  sed -E \
    -e 's/(Token: )[A-Za-z0-9_]+/\1[REDACTED]/g' \
    -e 's#https://[^/@[:space:]]+@github.com/#https://[REDACTED]@github.com/#g'
}

run_capture() {
  label="$1"
  shift
  tmp="$(mktemp /tmp/brevio-gh-preflight.XXXXXX)"
  "$@" >"$tmp" 2>&1
  status=$?
  echo "## $label exit=$status"
  sanitize <"$tmp"
  rm -f "$tmp"
  return "$status"
}

repo_root="$(git rev-parse --show-toplevel 2>/dev/null || pwd)"
cd "$repo_root" || exit 2

echo "BREVIO_GITHUB_PREFLIGHT repo=$repo_root"
echo "home=$HOME"
echo "gh=$(command -v gh 2>/dev/null || echo MISSING)"
echo "remote=$(git remote get-url origin 2>/dev/null | sanitize || echo MISSING)"
echo "mcp_fallback=${BREVIO_GITHUB_MCP_FALLBACK_AVAILABLE:-unknown}"

gh_ok=0
ls_ok=0
push_ok=0

run_capture "gh auth status" gh auth status && gh_ok=1
run_capture "git ls-remote origin HEAD" git ls-remote origin HEAD && ls_ok=1
run_capture "git push --dry-run origin main" git push --dry-run origin main && push_ok=1

if [ "$gh_ok" -eq 1 ] && [ "$ls_ok" -eq 1 ] && [ "$push_ok" -eq 1 ]; then
  echo "LOCAL_GITHUB_AUTH_OK"
  exit 0
fi

if [ "${BREVIO_GITHUB_MCP_FALLBACK_AVAILABLE:-}" = "1" ]; then
  echo "LOCAL_GITHUB_AUTH_FAIL_MCP_OK"
  exit 0
fi

echo "GITHUB_WRITE_BLOCKED"
exit 1
