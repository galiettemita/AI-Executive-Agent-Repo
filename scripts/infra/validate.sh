#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT_DIR"

if command -v terraform >/dev/null 2>&1; then
  echo "[infra] terraform validate modules"
  while IFS= read -r dir; do
    [ -z "$dir" ] && continue
    terraform -chdir="$dir" init -backend=false -input=false >/dev/null
    terraform -chdir="$dir" validate
  done < <(find terraform/modules terraform/environments -mindepth 1 -maxdepth 2 -type d | sort)
else
  echo "[infra] terraform not installed; skipped"
fi

if command -v helm >/dev/null 2>&1; then
  echo "[infra] helm lint charts"
  while IFS= read -r chart; do
    [ -z "$chart" ] && continue
    helm lint "$chart"
  done < <(find helm -mindepth 1 -maxdepth 1 -type d | sort)
else
  echo "[infra] helm not installed; skipped"
fi

echo "[infra] validation complete"
