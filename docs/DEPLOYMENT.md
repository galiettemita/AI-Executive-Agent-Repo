# DEPLOYMENT

## Infrastructure
Run full validation gates from repo root:

```bash
make ci-full
```

### 1) Provision AWS Infrastructure (Terraform)
Staging first, then production:

```bash
make infra-validate

cd terraform/environments/staging
terraform init
terraform validate
terraform plan
terraform apply

cd ../production
terraform init
terraform validate
terraform plan
terraform apply
```

### 2) Render Production Ingress Overrides (ACM + Domain)
Set deployment inputs and generate Helm values:

```bash
export ROOT_DOMAIN=testing-orbit.com
export ACM_CERT_ARN=arn:aws:acm:us-east-1:105914556507:certificate/c3f3b7e4-7c6e-4020-bae3-7f173b89ae6e
export API_HOST=api.testing-orbit.com
export ADMIN_HOST=admin.testing-orbit.com

scripts/deploy/render_prod_values.sh
```

Generated files:
- `artifacts/deploy/gateway-prod-values.yaml`
- `artifacts/deploy/admin-frontend-prod-values.yaml`

## Workload Deployment
Lint/render first:

```bash
for chart in helm/*; do
  helm lint "$chart"
  helm template "$(basename "$chart")" "$chart" >/dev/null
done
```

Install/upgrade charts:

```bash
# helm install is supported; this repo defaults to helm upgrade --install for idempotency.
helm upgrade --install brevio-gateway helm/BREVIO-gateway -f artifacts/deploy/gateway-prod-values.yaml
helm upgrade --install brevio-brain helm/BREVIO-brain
helm upgrade --install brevio-control helm/BREVIO-control
helm upgrade --install brevio-executor helm/BREVIO-executor
helm upgrade --install brevio-canvas helm/BREVIO-canvas
helm upgrade --install brevio-temporal-worker helm/BREVIO-temporal-worker
helm upgrade --install brevio-admin-api helm/BREVIO-admin-api
helm upgrade --install brevio-admin-frontend helm/BREVIO-admin-frontend -f artifacts/deploy/admin-frontend-prod-values.yaml
helm upgrade --install brevio-rag-worker helm/BREVIO-rag-worker
helm upgrade --install brevio-guardrails helm/BREVIO-guardrails
helm upgrade --install brevio-health-checker helm/BREVIO-health-checker
```

Or run one-command rollout:

```bash
make deploy-helm
```

### Wave 1 MCP Deployment Checklist (12-step gate)
Run the deterministic Wave 1 MCP checklist before production promotion:

```bash
make mcp-wave1-checklist
cat artifacts/deploy/wave1_deployment_checklist_report.json
```

The report must show `failed_servers = 0` across:
- `google_calendar`, `google_drive`, `google_gmail`, `notion`, `todoist`, `brave_search`, `github`, `apple_reminders`.

### Wave 5–6 MCP Deployment Checklist (10-server gate)
Run the deterministic Wave 5–6 MCP checklist for post-launch expansion connectors:

```bash
make mcp-wave56-checklist
cat artifacts/deploy/wave56_deployment_checklist_report.json
```

The report must show `failed_servers = 0` across:
- `duffel`, `zoom`, `calendly`, `plaid`, `crunchbase`, `booking`, `docusign`, `canva`, `instacart`, `tesla`.

### MCP Fleet Validation (Waves 1–6)
Run fleet validation gates for 40-server health, mixed concurrency, and failover recovery:

```bash
make mcp-fleet-validate
cat artifacts/deploy/mcp_fleet_validation_report.json
```

### MCP Runtime Build/Push/Deploy (Executor Path)
Generate deterministic MCP runtime rollout artifacts for the 40-server fleet:

```bash
make mcp-runtime-rollout
cat artifacts/deploy/mcp_runtime_rollout_plan.json
cat artifacts/deploy/executor-mcp-runtime-values.yaml
```

Execute build/push/deploy when credentials and cluster access are ready:

```bash
EXECUTOR_IMAGE_REPOSITORY=105914556507.dkr.ecr.us-east-1.amazonaws.com/brevio-executor \
EXECUTOR_IMAGE_TAG=v9.2.0 \
KUBE_NAMESPACE=default \
HELM_RELEASE=brevio-executor \
HELM_CHART_PATH=./helm/BREVIO-executor \
./scripts/dev/go_exec.sh run ./scripts/mcp/runtime_rollout/main.go --execute
```

Optional image/port overrides for production ECR rollout:

```bash
GATEWAY_IMAGE_REPOSITORY=105914556507.dkr.ecr.us-east-1.amazonaws.com/brevio-gateway \
GATEWAY_IMAGE_TAG=v9.2.0 \
GATEWAY_SERVICE_PORT=18080 \
ADMIN_FRONTEND_IMAGE_REPOSITORY=105914556507.dkr.ecr.us-east-1.amazonaws.com/brevio-admin-frontend \
ADMIN_FRONTEND_IMAGE_TAG=v9.2.0 \
ADMIN_FRONTEND_SERVICE_PORT=18082 \
WAIT_FOR_ROLLOUT=true \
make deploy-helm
```

### 3) Public DNS (Route53 or External Provider)
Create records pointing to ALB targets created by AWS Load Balancer Controller:
- `api.testing-orbit.com` -> gateway ingress ALB DNS name
- `admin.testing-orbit.com` -> admin frontend ingress ALB DNS name

If Route53 public hosted zone is not in this AWS account, configure records in the external DNS provider (for example, Cloudflare CNAME records).

## Canonical Sequence

```bash
cd terraform/environments/production
terraform init
terraform plan
terraform apply

helm install brevio-gateway helm/BREVIO-gateway
helm install brevio-brain helm/BREVIO-brain
helm install brevio-control helm/BREVIO-control
helm install brevio-executor helm/BREVIO-executor
helm install brevio-canvas helm/BREVIO-canvas
helm install brevio-temporal-worker helm/BREVIO-temporal-worker
helm install brevio-admin-api helm/BREVIO-admin-api
helm install brevio-admin-frontend helm/BREVIO-admin-frontend
helm install brevio-rag-worker helm/BREVIO-rag-worker
helm install brevio-guardrails helm/BREVIO-guardrails
helm install brevio-health-checker helm/BREVIO-health-checker
```

### 4) Post-Deploy Checks

```bash
kubectl get pods -A
kubectl get ingress
kubectl get svc

curl -i https://api.testing-orbit.com/healthz/live
curl -i https://api.testing-orbit.com/healthz/ready
curl -i https://admin.testing-orbit.com/healthz/live
curl -i https://admin.testing-orbit.com/healthz/ready
```

## Troubleshooting

### Cloudflare: CNAME disabled because Load Balancer exists on same hostname
If Cloudflare reports that a CNAME is disabled due to an active Cloudflare Load Balancer on the same hostname:
- Disable or update the Cloudflare Load Balancer object for that hostname.
- Keep only one DNS mapping path per hostname (either Cloudflare LB or direct CNAME/alias).
- Re-run the health checks after propagation:
  - `curl -i https://api.testing-orbit.com/healthz/live`
  - `curl -i https://admin.testing-orbit.com/healthz/live`
