#!/usr/bin/env bash
set -euo pipefail

SECONDARY_REGION="eu-west-1"
PRIMARY_REGION="us-east-1"
GLOBAL_CLUSTER_ID="brevio-global"
TEMPORAL_NAMESPACE="brevio-prod"

echo "[FAILOVER] Starting Brevio failover to eu-west-1..."

# Step 1: Promote Aurora secondary.
echo "[FAILOVER] Step 1: Promoting Aurora secondary in eu-west-1..."
aws rds failover-global-cluster \
  --global-cluster-identifier "${GLOBAL_CLUSTER_ID}" \
  --target-db-cluster-identifier "brevio-secondary" \
  --region "${PRIMARY_REGION}"

# Wait for promotion.
echo "[FAILOVER] Waiting for Aurora promotion (up to 60s)..."
for i in $(seq 1 12); do
  STATUS=$(aws rds describe-db-clusters \
    --db-cluster-identifier "brevio-secondary" \
    --region "${SECONDARY_REGION}" \
    --query 'DBClusters[0].Status' --output text 2>/dev/null || echo "unknown")
  if [ "${STATUS}" = "available" ]; then
    echo "[FAILOVER] Aurora promotion complete."
    break
  fi
  echo "[FAILOVER] Aurora status: ${STATUS}, waiting 5s..."
  sleep 5
done

# Step 2: Update Route53 to 100% eu-west-1.
echo "[FAILOVER] Step 2: Updating Route53 to 100% eu-west-1..."
if [ -d "infra/terraform/environments/failover" ]; then
  cd infra/terraform/environments/failover
  terraform init -input=false
  terraform apply -var="failover_active=true" -auto-approve
  cd -
else
  echo "[FAILOVER] WARNING: Failover terraform directory not found."
fi

# Step 3: Temporal namespace failover (if using Temporal Cloud).
echo "[FAILOVER] Step 3: Temporal namespace failover..."
if command -v tctl &>/dev/null; then
  tctl --namespace "${TEMPORAL_NAMESPACE}" namespace update --active-cluster eu-west-1 || true
else
  echo "[FAILOVER] WARNING: tctl not found. Temporal failover must be performed manually."
fi

# Step 4: Send P1 alert.
echo "[FAILOVER] Step 4: Sending P1 failover alert..."
WEBHOOK_URL="${ALERT_WEBHOOK_URL:-}"
if [ -n "${WEBHOOK_URL}" ]; then
  curl -s -X POST "${WEBHOOK_URL}" \
    -H "Content-Type: application/json" \
    -d "{\"text\": \"[P1 FAILOVER] Brevio has failed over to eu-west-1. Aurora promoted. Route53 updated. Time: $(date -u +%Y-%m-%dT%H:%M:%SZ)\"}" || true
else
  echo "[FAILOVER] WARNING: ALERT_WEBHOOK_URL not set. Skipping alert."
fi

echo "[FAILOVER] Failover complete. Verify services in eu-west-1."
