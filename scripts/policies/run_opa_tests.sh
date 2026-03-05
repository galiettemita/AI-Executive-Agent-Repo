#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT_DIR"

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

if command -v opa >/dev/null 2>&1; then
  echo "[opa-test] running with local opa binary"
  opa test policies/brevio policies/tests
  exit 0
fi

docker_bin="$(resolve_docker_bin || true)"
if [[ -z "${docker_bin}" ]]; then
  echo "[opa-test] neither opa nor docker is available"
  exit 1
fi

echo "[opa-test] running with dockerized OPA"
"$docker_bin" run --rm -v "$ROOT_DIR":/src -w /src openpolicyagent/opa:0.68.0 \
  test policies/brevio policies/tests
