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

### 3) Route53 DNS
Create alias records pointing to ALB targets created by AWS Load Balancer Controller:
- `api.testing-orbit.com` -> gateway ingress ALB
- `admin.testing-orbit.com` -> admin frontend ingress ALB

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
```
