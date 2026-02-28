#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT_DIR"

resolve_docker_bin() {
  if command -v docker >/dev/null 2>&1; then
    command -v docker
    return 0
  fi
  if [[ -x "/Applications/Docker.app/Contents/Resources/bin/docker" ]]; then
    echo "/Applications/Docker.app/Contents/Resources/bin/docker"
    return 0
  fi
  return 1
}

should_require_security_tooling() {
  if [[ "${REQUIRE_SECURITY_TOOLS:-0}" == "1" ]]; then
    return 0
  fi
  case "${CI:-}" in
    1|true|TRUE|yes|YES) return 0 ;;
  esac
  return 1
}

run_go_cmd() {
  local command_text="$1"
  if command -v go >/dev/null 2>&1; then
    bash -lc "$command_text"
    return 0
  fi

  local docker_bin
  if ! docker_bin="$(resolve_docker_bin)"; then
    if should_require_security_tooling; then
      echo "[security] Go toolchain unavailable and docker fallback missing in CI/strict mode"
      exit 1
    fi
    echo "[security] Go toolchain unavailable and docker fallback missing; skipped: ${command_text}"
    return 0
  fi

  "$docker_bin" run --rm -v "$ROOT_DIR":/src -w /src golang:1.22 sh -lc \
    "export PATH=\"/usr/local/go/bin:/go/bin:\$PATH\"; ${command_text}"
}

echo "[security] prompt injection tests"
run_go_cmd "go test ./internal/control -count=1"

echo "[security] webhook signature tests"
run_go_cmd "go test ./internal/gateway -count=1"

echo "[security] ssrf protection tests"
run_go_cmd "go test ./internal/executor -count=1"

echo "[security] pii encryption tests"
run_go_cmd "go test ./internal/security/pii -count=1"

echo "[security] sandbox enforcement tests"
run_go_cmd "go test ./internal/security/sandbox -count=1"

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
