.PHONY: build test lint migrate docker-build contracts acceptance ci load-test security-validate infra-validate

GO_EXEC := ./scripts/dev/go_exec.sh
GOFMT_EXEC := ./scripts/dev/gofmt_exec.sh

build:
	$(GO_EXEC) build ./...

test:
	$(GO_EXEC) test ./... -count=1

lint:
	test -z "$$($(GOFMT_EXEC) -l .)"
	$(GO_EXEC) vet ./...
	$(GO_EXEC) run honnef.co/go/tools/cmd/staticcheck@v0.5.1 ./...

migrate:
	test -f db/migrations/001_BREVIO_v9_init.sql
	test -f db/migrations/002_BREVIO_v91_soft_intelligence.sql
	test -f db/migrations/003_BREVIO_v92_production_hardening.sql
	$(GO_EXEC) test ./internal/database -run TestMigration -count=1

contracts:
	$(GO_EXEC) test ./internal/contracts -count=1

acceptance:
	$(GO_EXEC) test ./internal/contracts -run "TestAcceptanceGatesV9|TestAcceptanceGatesV91|TestAcceptanceGatesV92" -count=1

docker-build:
	@for svc in gateway brain control executor canvas temporal-worker; do \
		echo "building $$svc"; \
		docker build --build-arg SERVICE=$$svc -t brevio-$$svc:local .; \
	done

ci: lint build test migrate contracts acceptance

load-test:
	@echo "Run: k6 run evals/load/k6_interactive_turn.js"

security-validate:
	bash scripts/security/run_security_validation.sh

infra-validate:
	bash scripts/infra/validate.sh
