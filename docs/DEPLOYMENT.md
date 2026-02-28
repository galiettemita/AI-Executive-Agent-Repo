# DEPLOYMENT

## Infrastructure
Validate and plan Terraform before apply:

```bash
make infra-validate
cd terraform/environments/staging
terraform init
terraform validate
terraform plan
terraform apply
```

Repeat `terraform plan` and `terraform apply` for production only after staging is green.

## Workload Deployment
Lint and render all charts first:

```bash
for chart in helm/*; do
  helm lint "$chart"
  helm template "$(basename "$chart")" "$chart" >/dev/null
done
```

Deploy workloads with `helm install` after infra is ready.

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
