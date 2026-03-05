#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROTO_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
BUF_IMAGE="${BUF_IMAGE:-bufbuild/buf:1.57.2}"

if command -v buf >/dev/null 2>&1; then
  exec buf lint "$PROTO_DIR"
fi

if command -v docker >/dev/null 2>&1; then
  exec docker run --rm -v "$PROTO_DIR":/workspace -w /workspace "$BUF_IMAGE" lint
fi

echo "error: buf or docker is required to lint protobuf contracts" >&2
exit 1
