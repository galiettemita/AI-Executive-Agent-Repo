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

assert_exact_dir_set() {
  local base_dir="$1"
  shift
  local expected=("$@")
  local actual=()
  while IFS= read -r dir; do
    actual+=("$(basename "$dir")")
  done < <(find "$base_dir" -mindepth 1 -maxdepth 1 -type d | sort)

  local -A expected_map=()
  local -A actual_map=()
  local name
  for name in "${expected[@]}"; do
    expected_map["$name"]=1
  done
  for name in "${actual[@]}"; do
    actual_map["$name"]=1
  done

  local missing=()
  local extra=()
  for name in "${expected[@]}"; do
    if [[ -z "${actual_map[$name]:-}" ]]; then
      missing+=("$name")
    fi
  done
  for name in "${actual[@]}"; do
    if [[ -z "${expected_map[$name]:-}" ]]; then
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

if command -v terraform >/dev/null 2>&1; then
  echo "[infra] terraform validate modules"
  for module in "${required_terraform_modules[@]}"; do
    dir="terraform/modules/${module}"
    terraform -chdir="$dir" init -backend=false -input=false >/dev/null
    terraform -chdir="$dir" validate
  done

  echo "[infra] terraform validate environments"
  for env in "${required_terraform_environments[@]}"; do
    dir="terraform/environments/${env}"
    terraform -chdir="$dir" init -backend=false -input=false >/dev/null
    terraform -chdir="$dir" validate
  done
else
  if should_require_tooling; then
    echo "[infra] terraform is required in CI/strict mode but not installed"
    exit 1
  fi
  echo "[infra] terraform not installed; skipped"
fi

if command -v helm >/dev/null 2>&1; then
  echo "[infra] helm lint charts"
  for chart in "${required_helm_charts[@]}"; do
    chart_dir="helm/${chart}"
    helm lint "$chart_dir"
    rendered="$(helm template "$chart_dir")"
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
