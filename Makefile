.PHONY: opa-sync opa-test opa-verify dev build test lint migrate db-verify docker-build docker-build-infra contracts acceptance policy-validate ci ci-full load-test security-validate infra-validate api-docs api-docs-check tools-md tools-md-check skills-scaffolds-check proto-validate evals eval generate-remote-catalog-keys mcp-wave1-checklist mcp-wave56-checklist mcp-fleet-validate mcp-runtime-rollout deploy-helm staging-smoke-tests external-closeout-check external-closeout-regression-check external-phase-transition-check production-deployment-signoff-check production-canary-check production-deployment-todo production-post-deploy-validation production-phase-sync phase-closure-manifest phase-handoff-bundle phase-status go-live-signoff go-live-approval-packet go-live-approval-confirm manual-closeout-todo manual-provider-steps manual-closeout-batch-commands manual-closeout-confirm manual-closeout-unconfirm external-phase-sync local-verify audit

GO_EXEC := ./scripts/dev/go_exec.sh

opa-sync:
	@mkdir -p internal/policy/rego
	@cp policies/brevio/authz.rego         internal/policy/rego/authz.rego
	@cp policies/tool_write_gate.rego      internal/policy/rego/tool_write_gate.rego
	@cp policies/autonomy.rego             internal/policy/rego/autonomy.rego
	@cp policies/budget_enforcement.rego   internal/policy/rego/budget_enforcement.rego
	@cp policies/v10_gates.rego            internal/policy/rego/v10_gates.rego
	@echo "OPA policies synced."

opa-test:
	$(GO_EXEC) test -race -count=1 ./internal/policy/...

opa-verify: opa-sync opa-test
	@echo "OPA policies verified."
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
	test -f db/migrations/006_BREVIO_v93_addendum_specification_closure.sql
	test -f db/migrations/007_BREVIO_uuidv7_reconciliation.sql
	test -f db/migrations/008_BREVIO_v10_gap_closure.sql
	test -f db/migrations/009_BREVIO_v10_authorization_receipts.sql
	test -f db/migrations/010_BREVIO_v101_admin_intelligence.sql
	test -f db/migrations/011_BREVIO_v102_v103_intelligence.sql
	test -f db/migrations/012_BREVIO_v104_voice_calls.sql
	test -f db/migrations/013_BREVIO_openclaw_adoption.sql
	$(GO_EXEC) test ./internal/database -run TestMigration -count=1

db-verify:
	bash scripts/database/verify_postgres_migrations.sh

contracts:
	$(GO_EXEC) test ./internal/contracts -count=1

acceptance:
	$(GO_EXEC) test ./internal/contracts -run "TestAcceptanceGates" -count=1

policy-validate:
	bash scripts/policies/run_opa_tests.sh

docker-build:
	@for svc in gateway brain control executor canvas temporal-worker hands brevioctl; do \
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

# local-verify: fast pre-push gate — vet, build, test (includes contracts).
# No Docker, no external services, no staticcheck download required.
# For full lint (gofmt + staticcheck), use `make lint`.
local-verify:
	@echo "==> [1/3] go vet"
	$(GO_EXEC) vet ./...
	@echo "==> [2/3] build"
	$(GO_EXEC) build ./...
	@echo "==> [3/3] tests + contracts"
	$(GO_EXEC) test ./... -count=1
	@echo "==> local-verify passed"

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

staging-smoke-tests:
	bash scripts/deploy/run_staging_smoke_tests.sh

external-closeout-check:
	bash scripts/deploy/external_closeout_check.sh

external-closeout-regression-check:
	bash scripts/deploy/check_external_closeout_regressions.sh

external-phase-transition-check:
	bash scripts/deploy/check_external_phase_transition.sh

production-deployment-signoff-check:
	bash scripts/deploy/check_production_deployment_signoff.sh

production-canary-check:
	bash scripts/deploy/check_production_canary_window.sh

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

go-live-approval-packet:
	bash scripts/deploy/generate_final_go_live_approval_packet.sh

go-live-approval-confirm:
	test -n "$(ROLE)"
	test -n "$(APPROVED_BY)"
	bash scripts/deploy/confirm_final_go_live_approval.sh "$(ROLE)" "$(APPROVED_BY)" "$(NOTE)"

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

## eval: Run LLM pipeline evaluation harness against golden test cases
eval:
	$(GO_EXEC) test ./internal/llm/... -run TestEval -v -count=1

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

# audit: definitive pass/fail gate for production readiness.
# Runs tests, vet, and placeholder scan. Exits non-zero on any failure.
audit:
	@echo "==> [1/4] go vet"
	$(GO_EXEC) vet ./...
	@echo "==> [2/4] go build"
	$(GO_EXEC) build -mod=vendor ./...
	@echo "==> [3/4] go test"
	$(GO_EXEC) test ./... -count=1
	@echo "==> [4/4] placeholder scan (must be zero)"
	@if grep -rn --include='*.go' -E '//\s*(TO''DO|FIX''ME|ST''UB|PLACE''HOLDER|NOT_IMPL''EMENTED)\b' internal/ cmd/; then echo "FAIL: placeholder markers found"; exit 1; fi
	@echo "==> audit PASSED"
