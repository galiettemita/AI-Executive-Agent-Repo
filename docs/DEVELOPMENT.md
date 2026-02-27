# DEVELOPMENT

## Prerequisites
- Go 1.22
- Docker (optional for local parity build)

## Local Validation
Run full local validation:

```bash
gofmt -w .
go build ./...
go vet ./...
go test ./... -count=1
```

Run containerized validation (Go 1.22 image):

```bash
/Applications/Docker.app/Contents/Resources/bin/docker run --rm -v "$PWD":/src -w /src golang:1.22 sh -lc 'export PATH="/usr/local/go/bin:/go/bin:$PATH"; gofmt -w . && go build ./... && go vet ./... && go test ./... -count=1'
```

## Project Conventions
- IDs: UUIDv7 for new writes
- Naming: snake_case for identifiers
- Migrations: forward-only, no down scripts
- Policies: OPA Rego under `policies/`
