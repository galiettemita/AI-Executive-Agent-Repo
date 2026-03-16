#!/usr/bin/env bash
# bootstrap-aws.sh — Provision Brevio AWS infrastructure from zero.
# Prerequisites: aws CLI configured, kubectl installed, helm 3, terraform 1.7+
# Usage: ./scripts/bootstrap-aws.sh [staging|production] [--dry-run]
set -euo pipefail

ENVIRONMENT="${1:-staging}"
DRY_RUN="${2:-}"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
INFRA_DIR="${SCRIPT_DIR}/../infra/terraform/environments/${ENVIRONMENT}"

echo "=== Brevio AWS Bootstrap: ${ENVIRONMENT} ==="

# Step 1: Validate prerequisites
command -v aws       >/dev/null || { echo "ERROR: aws CLI not found"; exit 1; }
command -v kubectl   >/dev/null || { echo "ERROR: kubectl not found"; exit 1; }
command -v helm      >/dev/null || { echo "ERROR: helm not found"; exit 1; }
command -v terraform >/dev/null || { echo "ERROR: terraform not found"; exit 1; }

if [ ! -d "${INFRA_DIR}" ]; then
  echo "ERROR: Environment directory not found: ${INFRA_DIR}"
  exit 1
fi

# Step 2: Validate AWS credentials
aws sts get-caller-identity >/dev/null || { echo "ERROR: AWS credentials not configured"; exit 1; }
ACCOUNT_ID=$(aws sts get-caller-identity --query Account --output text)
REGION=${AWS_REGION:-us-east-1}
echo "Account: ${ACCOUNT_ID}, Region: ${REGION}"

# Step 3: Create Terraform state bucket if it doesn't exist
STATE_BUCKET="brevio-tfstate-${ACCOUNT_ID}-${REGION}"
if ! aws s3api head-bucket --bucket "${STATE_BUCKET}" 2>/dev/null; then
  echo "Creating Terraform state bucket: ${STATE_BUCKET}"
  if [ -z "${DRY_RUN}" ]; then
    aws s3api create-bucket \
      --bucket "${STATE_BUCKET}" \
      --region "${REGION}" \
      --create-bucket-configuration LocationConstraint="${REGION}" 2>/dev/null || true
    aws s3api put-bucket-versioning \
      --bucket "${STATE_BUCKET}" \
      --versioning-configuration Status=Enabled
    aws s3api put-bucket-encryption \
      --bucket "${STATE_BUCKET}" \
      --server-side-encryption-configuration \
      '{"Rules":[{"ApplyServerSideEncryptionByDefault":{"SSEAlgorithm":"AES256"}}]}'
  else
    echo "[DRY RUN] Would create S3 bucket: ${STATE_BUCKET}"
  fi
fi

# Step 4: Create DynamoDB lock table if it doesn't exist
LOCK_TABLE="brevio-tfstate-lock-${ENVIRONMENT}"
if ! aws dynamodb describe-table --table-name "${LOCK_TABLE}" 2>/dev/null; then
  echo "Creating DynamoDB lock table: ${LOCK_TABLE}"
  if [ -z "${DRY_RUN}" ]; then
    aws dynamodb create-table \
      --table-name "${LOCK_TABLE}" \
      --attribute-definitions AttributeName=LockID,AttributeType=S \
      --key-schema AttributeName=LockID,KeyType=HASH \
      --billing-mode PAY_PER_REQUEST \
      --region "${REGION}"
  else
    echo "[DRY RUN] Would create DynamoDB table: ${LOCK_TABLE}"
  fi
fi

# Step 5: Terraform init and apply
echo "=== Terraform: ${ENVIRONMENT} ==="
cd "${INFRA_DIR}"
if [ -z "${DRY_RUN}" ]; then
  terraform init \
    -backend-config="bucket=${STATE_BUCKET}" \
    -backend-config="key=${ENVIRONMENT}/terraform.tfstate" \
    -backend-config="region=${REGION}" \
    -backend-config="dynamodb_table=${LOCK_TABLE}"
  terraform plan -out=tfplan
  terraform apply -auto-approve tfplan
else
  echo "[DRY RUN] Would run: terraform init + plan + apply in ${INFRA_DIR}"
fi

# Step 6: Configure kubectl
CLUSTER_NAME="brevio-${ENVIRONMENT}"
echo "=== Configuring kubectl for cluster: ${CLUSTER_NAME} ==="
if [ -z "${DRY_RUN}" ]; then
  aws eks update-kubeconfig \
    --name "${CLUSTER_NAME}" \
    --region "${REGION}"
else
  echo "[DRY RUN] Would configure kubectl for ${CLUSTER_NAME}"
fi

# Step 7: Install Karpenter
echo "=== Installing Karpenter ==="
KARPENTER_VERSION="v0.36.0"
if [ -z "${DRY_RUN}" ]; then
  helm upgrade --install karpenter oci://public.ecr.aws/karpenter/karpenter \
    --version "${KARPENTER_VERSION}" \
    --namespace karpenter --create-namespace \
    --set "settings.clusterName=${CLUSTER_NAME}" \
    --wait || echo "WARN: Karpenter install failed (may already exist)"
else
  echo "[DRY RUN] Would install Karpenter ${KARPENTER_VERSION}"
fi

# Step 8: Install KEDA (for Temporal queue depth HPA)
echo "=== Installing KEDA ==="
if [ -z "${DRY_RUN}" ]; then
  helm repo add kedacore https://kedacore.github.io/charts 2>/dev/null || true
  helm repo update 2>/dev/null || true
  helm upgrade --install keda kedacore/keda \
    --namespace keda --create-namespace \
    --version 2.14.0 --wait || echo "WARN: KEDA install failed (may already exist)"
else
  echo "[DRY RUN] Would install KEDA 2.14.0"
fi

# Step 9: Install Brevio services
echo "=== Installing Brevio Helm chart ==="
HELM_DIR="${SCRIPT_DIR}/../infra/helm/brevio"
VALUES_FILE="${HELM_DIR}/values-${ENVIRONMENT}.yaml"
if [ ! -f "${VALUES_FILE}" ]; then
  echo "WARN: No values file for ${ENVIRONMENT}, using defaults only"
  VALUES_FILE=""
fi
if [ -z "${DRY_RUN}" ]; then
  HELM_ARGS=(upgrade --install brevio "${HELM_DIR}" -f "${HELM_DIR}/values.yaml")
  if [ -n "${VALUES_FILE}" ]; then
    HELM_ARGS+=(-f "${VALUES_FILE}")
  fi
  HELM_ARGS+=(--namespace brevio --create-namespace --wait --timeout=10m)
  helm "${HELM_ARGS[@]}"
else
  echo "[DRY RUN] Would install Brevio Helm chart with values-${ENVIRONMENT}.yaml"
fi

# Step 10: Run smoke tests
echo "=== Running post-deploy smoke tests ==="
if [ -z "${DRY_RUN}" ]; then
  kubectl -n brevio wait --for=condition=ready pod \
    -l app.kubernetes.io/name=brevio --timeout=300s || echo "WARN: Some pods not ready"
  echo "Smoke test: checking gateway health endpoint..."
  kubectl -n brevio port-forward svc/brevio-gateway 8080:8080 &
  PF_PID=$!
  sleep 3
  curl -sf http://localhost:8080/health || echo "WARN: Gateway health check failed"
  kill $PF_PID 2>/dev/null || true
else
  echo "[DRY RUN] Would run post-deploy smoke tests"
fi

echo ""
echo "=== Bootstrap complete: ${ENVIRONMENT} ==="
echo "Cluster:    ${CLUSTER_NAME}"
echo "Region:     ${REGION}"
echo "Account:    ${ACCOUNT_ID}"
