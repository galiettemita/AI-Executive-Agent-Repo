# DEVELOPMENT

## Prerequisites
- Go 1.22 (or Docker for toolchain fallback)
- Docker Desktop
- Make

## Local Validation
Run full local validation:

```bash
gofmt -w .
go build ./...
go vet ./...
go test ./... -count=1
```

Run full CI-parity checks:

```bash
make ci
```

Regenerate documentation artifacts:

```bash
make api-docs
make tools-md
```

## Project Conventions
- IDs: UUIDv7 for all new writes
- Naming: snake_case
- Migrations: forward-only (no down migrations)
- OPA policies under `policies/`
- Schemas under `schemas/` with `additionalProperties: false`
- Operational ownership and on-call policy in `docs/OPERATIONS_OWNERSHIP.md`
