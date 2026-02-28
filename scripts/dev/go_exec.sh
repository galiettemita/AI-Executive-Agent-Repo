#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"

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

if command -v go >/dev/null 2>&1; then
  exec go "$@"
fi

docker_bin="$(resolve_docker_bin || true)"
if [[ -z "${docker_bin}" ]]; then
  echo "[go_exec] go command not found and docker is unavailable"
  exit 1
fi

if [[ $# -eq 0 ]]; then
  echo "[go_exec] expected go command arguments"
  exit 1
fi

quoted_args=()
for arg in "$@"; do
  quoted_args+=("$(printf '%q' "$arg")")
done
joined_args="${quoted_args[*]}"

exec "$docker_bin" run --rm -v "$ROOT_DIR":/src -w /src golang:1.22 sh -lc \
  "export PATH=\"/usr/local/go/bin:/go/bin:\$PATH\"; go ${joined_args}"
