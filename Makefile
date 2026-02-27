.PHONY: build test lint migrate docker-build contracts acceptance ci load-test security-validate infra-validate

build:
	go build ./...

test:
	go test ./... -count=1

lint:
	test -z "$$(gofmt -l .)"
	go vet ./...
	go install honnef.co/go/tools/cmd/staticcheck@latest
	staticcheck ./...

migrate:
	test -f db/migrations/001_BREVIO_v9_init.sql
	test -f db/migrations/002_BREVIO_v91_soft_intelligence.sql
	test -f db/migrations/003_BREVIO_v92_production_hardening.sql
	go test ./internal/database -run TestMigration -count=1

contracts:
	go test ./internal/contracts -count=1

acceptance:
	go test ./internal/contracts -run "TestAcceptanceGatesV9|TestAcceptanceGatesV91|TestAcceptanceGatesV92" -count=1

docker-build:
	docker build -t brevio:local .

ci: lint build test migrate contracts acceptance

load-test:
	@echo "Run: k6 run evals/load/k6_interactive_turn.js"

security-validate:
	bash scripts/security/run_security_validation.sh

infra-validate:
	bash scripts/infra/validate.sh
