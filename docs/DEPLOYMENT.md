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
```

Deploy each chart once cluster prerequisites and secrets are ready.
