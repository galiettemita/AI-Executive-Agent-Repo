#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT_DIR"

should_require_security_tooling() {
  if [[ "${REQUIRE_SECURITY_TOOLS:-0}" == "1" ]]; then
    return 0
  fi
  case "${CI:-}" in
    1|true|TRUE|yes|YES) return 0 ;;
  esac
  return 1
}

echo "[security] prompt injection tests"
go test ./internal/control -count=1

echo "[security] webhook signature tests"
go test ./internal/gateway -count=1

echo "[security] ssrf protection tests"
go test ./internal/executor -count=1

if command -v trivy >/dev/null 2>&1; then
  echo "[security] trivy filesystem scan"
  trivy fs --scanners vuln --severity CRITICAL,HIGH --exit-code 1 .
else
  if should_require_security_tooling; then
    echo "[security] trivy is required in CI/strict mode but not installed"
    exit 1
  fi
  echo "[security] trivy not installed; skipped"
fi

if command -v trufflehog >/dev/null 2>&1; then
  echo "[security] trufflehog scan"
  trufflehog filesystem . --fail
else
  if should_require_security_tooling; then
    echo "[security] trufflehog is required in CI/strict mode but not installed"
    exit 1
  fi
  echo "[security] trufflehog not installed; skipped"
fi

if command -v syft >/dev/null 2>&1; then
  echo "[security] syft sbom"
  syft dir:. -o spdx-json > sbom.spdx.json
  test -s sbom.spdx.json
else
  if should_require_security_tooling; then
    echo "[security] syft is required in CI/strict mode but not installed"
    exit 1
  fi
  echo "[security] syft not installed; skipped"
fi

echo "[security] govulncheck baseline"
bash scripts/security/run_govulncheck.sh

echo "[security] complete"
