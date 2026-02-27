#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT_DIR"

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
  echo "[security] trivy not installed; skipped"
fi

if command -v trufflehog >/dev/null 2>&1; then
  echo "[security] trufflehog scan"
  trufflehog filesystem . --fail
else
  echo "[security] trufflehog not installed; skipped"
fi

if command -v syft >/dev/null 2>&1; then
  echo "[security] syft sbom"
  syft dir:. -o spdx-json > sbom.spdx.json
  test -s sbom.spdx.json
else
  echo "[security] syft not installed; skipped"
fi

echo "[security] complete"
