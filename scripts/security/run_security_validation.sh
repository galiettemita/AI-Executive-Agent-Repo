#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT_DIR"

TRUFFLEHOG_EXCLUDE_PATHS_FILE="scripts/security/trufflehog_exclude_paths.txt"
TRIVY_REPORT_PATH="artifacts/security/trivy_fs_report.json"
TRIVY_ALLOWLIST_PATH="scripts/security/trivy_allowlist.txt"

trivy_scan_args=(
  fs
  --scanners vuln
  --severity CRITICAL,HIGH
  --exit-code 0
  --format json
  --output "$TRIVY_REPORT_PATH"
  --skip-dirs .git
  --skip-dirs classmate-ai
  --skip-dirs artifacts
  .
)

trufflehog_scan_args=(
  filesystem
  .
  --fail
  --exclude-paths "$TRUFFLEHOG_EXCLUDE_PATHS_FILE"
  --force-skip-binaries
)

syft_scan_args=(
  dir:.
  --exclude "./.git/**"
  --exclude "**/.terraform/**"
  --exclude "./classmate-ai/**"
  --exclude "./artifacts/**"
  -o spdx-json
)

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

run_dockerized_security_tool() {
  local tool_name="$1"
  local docker_image="$2"
  shift 2

  local docker_bin
  if ! docker_bin="$(resolve_docker_bin)"; then
    if should_require_security_tooling; then
      echo "[security] ${tool_name} requires docker fallback in CI/strict mode but docker is unavailable"
      exit 1
    fi
    echo "[security] ${tool_name} missing and docker fallback unavailable; skipped"
    return 0
  fi

  echo "[security] ${tool_name} missing on host; using dockerized ${docker_image}"
  if ! "$docker_bin" run --rm -v "$ROOT_DIR":/src -w /src "$docker_image" "$@"; then
    if should_require_security_tooling; then
      echo "[security] ${tool_name} dockerized execution failed in CI/strict mode"
      exit 1
    fi
    echo "[security] ${tool_name} dockerized execution failed; skipped"
    return 0
  fi
  return 0
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

mkdir -p artifacts/security
rm -f "$TRIVY_REPORT_PATH"

if command -v trivy >/dev/null 2>&1; then
  echo "[security] trivy filesystem scan"
  trivy "${trivy_scan_args[@]}"
else
  run_dockerized_security_tool "trivy" "aquasec/trivy:0.57.1" \
    "${trivy_scan_args[@]}"
fi

if [[ -s "$TRIVY_REPORT_PATH" ]]; then
  echo "[security] trivy allowlist evaluation"
  python3 scripts/security/check_trivy_report.py "$TRIVY_REPORT_PATH" "$TRIVY_ALLOWLIST_PATH"
else
  if should_require_security_tooling; then
    echo "[security] trivy report missing in CI/strict mode: $TRIVY_REPORT_PATH"
    exit 1
  fi
  echo "[security] trivy report missing; skipped allowlist evaluation"
fi

if command -v trufflehog >/dev/null 2>&1; then
  echo "[security] trufflehog scan"
  trufflehog "${trufflehog_scan_args[@]}"
else
  run_dockerized_security_tool "trufflehog" "ghcr.io/trufflesecurity/trufflehog:3.90.4" \
    "${trufflehog_scan_args[@]}"
fi

if command -v syft >/dev/null 2>&1; then
  echo "[security] syft sbom"
  syft "${syft_scan_args[@]}" > sbom.spdx.json
  test -s sbom.spdx.json
else
  docker_bin=""
  if docker_bin="$(resolve_docker_bin)"; then
    echo "[security] syft missing on host; using dockerized anchore/syft:v1.20.0"
    if ! "$docker_bin" run --rm -v "$ROOT_DIR":/src -w /src anchore/syft:v1.20.0 "${syft_scan_args[@]}" > sbom.spdx.json; then
      if should_require_security_tooling; then
        echo "[security] syft dockerized execution failed in CI/strict mode"
        exit 1
      fi
      echo "[security] syft dockerized execution failed; skipped"
      rm -f sbom.spdx.json
    else
      test -s sbom.spdx.json
    fi
  else
    if should_require_security_tooling; then
      echo "[security] syft is required in CI/strict mode but neither host binary nor docker fallback is available"
      exit 1
    fi
    echo "[security] syft missing and docker fallback unavailable; skipped"
  fi
fi

echo "[security] govulncheck baseline"
bash scripts/security/run_govulncheck.sh

echo "[security] complete"
