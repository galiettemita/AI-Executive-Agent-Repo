#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT_DIR"

required_terraform_modules=(
  "vpc"
  "eks"
  "rds"
  "elasticache"
  "sqs"
  "s3"
  "secrets"
  "temporal"
  "observability"
  "opensearch"
  "admin-frontend"
  "feature-flags-cache"
)

required_terraform_environments=(
  "staging"
  "production"
)

required_helm_charts=(
  "BREVIO-gateway"
  "BREVIO-brain"
  "BREVIO-control"
  "BREVIO-executor"
  "BREVIO-canvas"
  "BREVIO-temporal-worker"
  "BREVIO-admin-api"
  "BREVIO-admin-frontend"
  "BREVIO-rag-worker"
  "BREVIO-guardrails"
  "BREVIO-health-checker"
)

should_require_tooling() {
  if [[ "${REQUIRE_INFRA_TOOLS:-0}" == "1" ]]; then
    return 0
  fi
  case "${CI:-}" in
    1|true|TRUE|yes|YES) return 0 ;;
  esac
  return 1
}

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

run_terraform() {
  if command -v terraform >/dev/null 2>&1; then
    terraform "$@"
    return 0
  fi
  local docker_bin
  docker_bin="$(resolve_docker_bin || true)"
  if [[ -z "${docker_bin}" ]]; then
    return 127
  fi
  "$docker_bin" run --rm -v "$ROOT_DIR":/src -w /src hashicorp/terraform:1.9.8 "$@"
}

run_helm() {
  if command -v helm >/dev/null 2>&1; then
    helm "$@"
    return 0
  fi
  local docker_bin
  docker_bin="$(resolve_docker_bin || true)"
  if [[ -z "${docker_bin}" ]]; then
    return 127
  fi
  "$docker_bin" run --rm -v "$ROOT_DIR":/src -w /src alpine/helm:3.16.4 "$@"
}

array_contains() {
  local needle="$1"
  shift
  local item
  for item in "$@"; do
    if [[ "$item" == "$needle" ]]; then
      return 0
    fi
  done
  return 1
}

assert_exact_dir_set() {
  local base_dir="$1"
  shift
  local expected=("$@")
  local actual=()
  while IFS= read -r dir; do
    actual+=("$(basename "$dir")")
  done < <(find "$base_dir" -mindepth 1 -maxdepth 1 -type d | sort)

  local name

  local missing=()
  local extra=()
  for name in "${expected[@]}"; do
    if ! array_contains "$name" "${actual[@]}"; then
      missing+=("$name")
    fi
  done
  for name in "${actual[@]}"; do
    if ! array_contains "$name" "${expected[@]}"; then
      extra+=("$name")
    fi
  done

  if (( ${#missing[@]} > 0 || ${#extra[@]} > 0 )); then
    echo "[infra] directory-set mismatch for ${base_dir}"
    echo "[infra] missing: ${missing[*]:-(none)}"
    echo "[infra] extra: ${extra[*]:-(none)}"
    return 1
  fi
}

assert_exact_dir_set "terraform/modules" "${required_terraform_modules[@]}"
assert_exact_dir_set "terraform/environments" "${required_terraform_environments[@]}"
assert_exact_dir_set "helm" "${required_helm_charts[@]}"

if command -v terraform >/dev/null 2>&1 || resolve_docker_bin >/dev/null 2>&1; then
  if command -v terraform >/dev/null 2>&1; then
    echo "[infra] terraform binary detected on host"
  else
    echo "[infra] terraform missing on host; using dockerized terraform:1.9.8"
  fi
  echo "[infra] terraform fmt check"
  run_terraform fmt -check -recursive

  echo "[infra] terraform validate modules"
  for module in "${required_terraform_modules[@]}"; do
    dir="terraform/modules/${module}"
    run_terraform -chdir="$dir" init -backend=false -input=false >/dev/null
    run_terraform -chdir="$dir" validate
  done

  echo "[infra] terraform validate environments"
  for env in "${required_terraform_environments[@]}"; do
    dir="terraform/environments/${env}"
    run_terraform -chdir="$dir" init -backend=false -input=false >/dev/null
    run_terraform -chdir="$dir" validate
  done
else
  if should_require_tooling; then
    echo "[infra] terraform is required in CI/strict mode but not installed"
    exit 1
  fi
  echo "[infra] terraform not installed; skipped"
fi

if command -v helm >/dev/null 2>&1 || resolve_docker_bin >/dev/null 2>&1; then
  if command -v helm >/dev/null 2>&1; then
    echo "[infra] helm binary detected on host"
  else
    echo "[infra] helm missing on host; using dockerized alpine/helm:3.16.4"
  fi
  echo "[infra] helm lint charts"
  for chart in "${required_helm_charts[@]}"; do
    chart_dir="helm/${chart}"
    run_helm lint "$chart_dir"
    rendered="$(run_helm template "$chart_dir")"
    if [[ -z "${rendered}" ]]; then
      echo "[infra] helm template produced empty output for ${chart_dir}"
      exit 1
    fi
  done
else
  if should_require_tooling; then
    echo "[infra] helm is required in CI/strict mode but not installed"
    exit 1
  fi
  echo "[infra] helm not installed; skipped"
fi

echo "[infra] validation complete"
