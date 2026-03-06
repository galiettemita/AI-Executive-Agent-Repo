#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"

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

if command -v gofmt >/dev/null 2>&1; then
  exec gofmt "$@"
fi

if command -v go >/dev/null 2>&1; then
  go_bin="$(command -v go)"
  gofmt_bin="$(dirname "$go_bin")/gofmt"
  if [[ -x "$gofmt_bin" ]]; then
    exec "$gofmt_bin" "$@"
  fi
fi

docker_bin="$(resolve_docker_bin || true)"
if [[ -z "${docker_bin}" ]]; then
  echo "[gofmt_exec] gofmt command not found and docker is unavailable"
  exit 1
fi

quoted_args=()
for arg in "$@"; do
  quoted_args+=("$(printf '%q' "$arg")")
done
joined_args="${quoted_args[*]}"

exec "$docker_bin" run --rm -v "$ROOT_DIR":/src -w /src golang:1.23 sh -lc \
  "export PATH=\"/usr/local/go/bin:/go/bin:\$PATH\"; gofmt ${joined_args}"
