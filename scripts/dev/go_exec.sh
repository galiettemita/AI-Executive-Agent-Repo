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

go_mod_cache="/tmp/brevio-go-mod-cache"
go_build_cache="/tmp/brevio-go-build-cache"
mkdir -p "${go_mod_cache}" "${go_build_cache}"

quoted_args=()
for arg in "$@"; do
  quoted_args+=("$(printf '%q' "$arg")")
done
joined_args="${quoted_args[*]}"

exec "$docker_bin" run --rm \
  -v "$ROOT_DIR":/src \
  -v "${go_mod_cache}":/go/pkg/mod \
  -v "${go_build_cache}":/root/.cache/go-build \
  -w /src golang:1.22 sh -lc \
  "export PATH=\"/usr/local/go/bin:/go/bin:\$PATH\"; git config --global --add safe.directory /src >/dev/null 2>&1 || true; export GOFLAGS=\"\${GOFLAGS:-} -buildvcs=false\"; go ${joined_args}"
