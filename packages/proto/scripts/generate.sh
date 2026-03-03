#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROTO_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
BUF_IMAGE="${BUF_IMAGE:-bufbuild/buf:1.57.2}"

mkdir -p "$PROTO_DIR/gen"

if command -v buf >/dev/null 2>&1; then
  exec buf generate "$PROTO_DIR"
fi

if command -v docker >/dev/null 2>&1; then
  exec docker run --rm -v "$PROTO_DIR":/workspace -w /workspace "$BUF_IMAGE" generate
fi

echo "error: buf or docker is required to generate protobuf artifacts" >&2
exit 1
