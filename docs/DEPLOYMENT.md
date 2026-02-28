# DEPLOYMENT

## Infrastructure
Terraform artifacts are structured in modules and env compositions:
- `terraform/modules/*`
- `terraform/environments/staging/main.tf`
- `terraform/environments/production/main.tf`

Typical sequence:

```bash
cd terraform/environments/staging
terraform init
terraform plan
terraform apply
```

Repeat for production after approval.

## Workload Deployment
Helm charts are defined in `helm/`.

Example:

```bash
helm lint helm/BREVIO-gateway
helm template helm/BREVIO-gateway
helm install brevio-gateway helm/BREVIO-gateway
```

Deploy each chart once cluster prerequisites and secrets are ready.

## Canonical Sequence
Production deployment flow is intentionally forward-only:

```bash
cd terraform/environments/production
terraform init
terraform apply

helm install brevio-gateway helm/BREVIO-gateway
helm install brevio-brain helm/BREVIO-brain
helm install brevio-control helm/BREVIO-control
helm install brevio-executor helm/BREVIO-executor
helm install brevio-canvas helm/BREVIO-canvas
helm install brevio-temporal-worker helm/BREVIO-temporal-worker
```
