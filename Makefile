.PHONY: dev build test lint migrate db-verify docker-build docker-build-infra contracts acceptance policy-validate ci ci-full load-test security-validate infra-validate api-docs api-docs-check tools-md tools-md-check skills-scaffolds-check proto-validate evals generate-remote-catalog-keys mcp-wave1-checklist mcp-wave56-checklist mcp-fleet-validate mcp-runtime-rollout deploy-helm external-closeout-check external-closeout-regression-check external-phase-transition-check production-deployment-signoff-check production-deployment-todo production-post-deploy-validation production-phase-sync phase-closure-manifest phase-handoff-bundle phase-status go-live-signoff manual-closeout-todo manual-provider-steps manual-closeout-batch-commands manual-closeout-confirm manual-closeout-unconfirm external-phase-sync

GO_EXEC := ./scripts/dev/go_exec.sh
GOFMT_EXEC := ./scripts/dev/gofmt_exec.sh

dev:
	bash scripts/setup-local.sh

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
	test -f db/migrations/004_BREVIO_ops_operational_systems.sql
	test -f db/migrations/005_BREVIO_mcp_execution_oauth_hardening.sql
	$(GO_EXEC) test ./internal/database -run TestMigration -count=1

db-verify:
	bash scripts/database/verify_postgres_migrations.sh

contracts:
	$(GO_EXEC) test ./internal/contracts -count=1

acceptance:
	$(GO_EXEC) test ./internal/contracts -run "TestAcceptanceGatesV9|TestAcceptanceGatesV91|TestAcceptanceGatesV92" -count=1

policy-validate:
	bash scripts/policies/run_opa_tests.sh

docker-build:
	@for svc in gateway brain control executor canvas temporal-worker; do \
		echo "building $$svc"; \
		docker build --build-arg SERVICE=$$svc -t brevio-$$svc:local .; \
	done

docker-build-infra:
	@for svc in gateway brain hands control executor canvas temporal-worker; do \
		echo "building $$svc from infra/docker"; \
		docker build -f infra/docker/Dockerfile.$$svc -t brevio-$$svc:local .; \
	done
	@for svc in auth profile scheduler metrics edge-relay; do \
		echo "building $$svc from infra/docker"; \
		docker build -f infra/docker/Dockerfile.$$svc -t brevio-$$svc:local .; \
	done

ci: proto-validate lint build test migrate api-docs-check tools-md-check skills-scaffolds-check mcp-wave1-checklist mcp-wave56-checklist mcp-fleet-validate mcp-runtime-rollout policy-validate contracts acceptance evals

ci-full: ci security-validate infra-validate db-verify

load-test:
	@echo "Run: k6 run evals/load/k6_interactive_turn.js"
	@echo "Run: k6 run evals/load/k6_load_shedding.js"
	@echo "Run: k6 run evals/load/k6_streaming_first_byte.js"

security-validate:
	bash scripts/security/run_security_validation.sh

infra-validate:
	bash scripts/infra/validate.sh

deploy-helm:
	bash scripts/deploy/helm_rollout.sh

external-closeout-check:
	bash scripts/deploy/external_closeout_check.sh

external-closeout-regression-check:
	bash scripts/deploy/check_external_closeout_regressions.sh

external-phase-transition-check:
	bash scripts/deploy/check_external_phase_transition.sh

production-deployment-signoff-check:
	bash scripts/deploy/check_production_deployment_signoff.sh

production-deployment-todo:
	bash scripts/deploy/generate_production_deployment_todo.sh

production-post-deploy-validation:
	bash scripts/deploy/check_production_post_deploy_validation.sh

production-phase-sync:
	bash scripts/deploy/sync_production_phase_artifacts.sh

phase-closure-manifest:
	bash scripts/deploy/generate_phase_closure_manifest.sh

phase-handoff-bundle:
	bash scripts/deploy/create_phase_handoff_bundle.sh

phase-status:
	bash scripts/deploy/print_phase_status.sh

go-live-signoff:
	bash scripts/deploy/generate_go_live_signoff.sh

manual-closeout-todo:
	bash scripts/deploy/generate_manual_closeout_todo.sh

manual-provider-steps:
	bash scripts/deploy/generate_manual_provider_steps.sh

manual-closeout-batch-commands:
	bash scripts/deploy/generate_manual_closeout_batch_commands.sh

manual-closeout-confirm:
	test -n "$(ITEM_ID)"
	test -n "$(CONFIRMED_BY)"
	bash scripts/deploy/update_manual_closeout_evidence.sh "$(ITEM_ID)" "$(CONFIRMED_BY)" "$(NOTE)"

manual-closeout-unconfirm:
	test -n "$(ITEM_ID)"
	test -n "$(REVOKED_BY)"
	bash scripts/deploy/revoke_manual_closeout_evidence.sh "$(ITEM_ID)" "$(REVOKED_BY)" "$(NOTE)"

external-phase-sync:
	bash scripts/deploy/sync_external_phase_artifacts.sh

api-docs:
	$(GO_EXEC) run ./scripts/docs/generate_api_reference.go

api-docs-check:
	$(GO_EXEC) run ./scripts/docs/generate_api_reference.go
	git diff --exit-code docs/API_REFERENCE.md

tools-md:
	$(GO_EXEC) run ./scripts/tools/generate_tools_md.go

tools-md-check:
	$(GO_EXEC) run ./scripts/tools/generate_tools_md.go
	git diff --exit-code TOOLS.md

skills-scaffolds-check:
	bash scripts/skills/check_hands_skill_scaffold_parity.sh

proto-validate:
	bash packages/proto/scripts/lint.sh

evals:
	bash scripts/run-evals.sh

generate-remote-catalog-keys:
	$(GO_EXEC) run ./scripts/tools/remote_catalog_keys/main.go

mcp-wave1-checklist:
	$(GO_EXEC) run ./scripts/mcp/wave1_checklist/main.go

mcp-wave56-checklist:
	$(GO_EXEC) run ./scripts/mcp/wave56_checklist/main.go

mcp-fleet-validate:
	$(GO_EXEC) run ./scripts/mcp/fleet_validation/main.go

mcp-runtime-rollout:
	$(GO_EXEC) run ./scripts/mcp/runtime_rollout/main.go
