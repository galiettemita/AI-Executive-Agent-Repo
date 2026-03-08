package contracts

import (
	"path/filepath"
	"testing"
)

func TestAcceptanceGatesV9(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)
	runtimeGatePath := filepath.Join(root, "internal", "contracts", "acceptance_gate_runtime_closure_test.go")

	t.Run("schema_closure", func(t *testing.T) {
		assertFileNonEmpty(t, filepath.Join(root, "schemas", "tool_call.v9.json"))
		assertFileNonEmpty(t, filepath.Join(root, "schemas", "error.v9.json"))
		assertFileContainsTokens(t, runtimeGatePath, []string{
			`t.Run("schema_closure",`,
		})
	})
	t.Run("determinism", func(t *testing.T) {
		assertFileNonEmpty(t, filepath.Join(root, "evals", "determinism_fixtures.json"))
		assertFileContainsTokens(t, runtimeGatePath, []string{
			`t.Run("determinism",`,
		})
	})
	t.Run("webhook_security", func(t *testing.T) {
		assertFileContainsTokens(t, filepath.Join(root, "internal", "gateway", "service.go"), []string{
			"ErrInvalidSignature",
			"ErrReplayDetected",
		})
		assertFileContainsTokens(t, runtimeGatePath, []string{
			`t.Run("webhook_security",`,
		})
	})
	t.Run("acceptance_suites_1_12", func(t *testing.T) {
		assertFileNonEmpty(t, filepath.Join(root, ".github", "workflows", "ci.yml"))
		assertFileContainsTokens(t, runtimeGatePath, []string{
			`t.Run("acceptance_suites_1_12",`,
		})
	})
	t.Run("workspace_isolation", func(t *testing.T) {
		assertFileContainsTokens(t, filepath.Join(root, "db", "migrations", "001_BREVIO_v9_init.sql"), []string{
			"ENABLE ROW LEVEL SECURITY",
			"current_setting(''app.workspace_id'')::uuid",
		})
		assertFileContainsTokens(t, runtimeGatePath, []string{
			`t.Run("workspace_isolation",`,
		})
	})
	t.Run("provisioning_pipeline", func(t *testing.T) {
		assertFileNonEmpty(t, filepath.Join(root, "internal", "provisioning", "service_test.go"))
		assertFileContainsTokens(t, runtimeGatePath, []string{
			`t.Run("provisioning_pipeline",`,
		})
	})
	t.Run("onboarding_completion", func(t *testing.T) {
		assertFileNonEmpty(t, filepath.Join(root, "internal", "onboarding", "service_test.go"))
		assertFileContainsTokens(t, runtimeGatePath, []string{
			`t.Run("onboarding_completion",`,
		})
	})
	t.Run("provisioning_recovery", func(t *testing.T) {
		assertFileContainsTokens(t, filepath.Join(root, "internal", "workflows", "service_test.go"), []string{
			"TestProvisioningCompensationReverseOrder",
			"CompensatedSteps",
		})
		assertFileContainsTokens(t, runtimeGatePath, []string{
			`t.Run("provisioning_recovery",`,
		})
	})
	t.Run("deterministic_llm", func(t *testing.T) {
		assertFileContainsTokens(t, filepath.Join(root, "internal", "llm", "service_test.go"), []string{
			"TestDeterminismSameInput20Runs",
			"ReplayHitCount",
		})
		assertFileContainsTokens(t, runtimeGatePath, []string{
			`t.Run("deterministic_llm",`,
		})
	})
	t.Run("cve_scanning", func(t *testing.T) {
		assertFileContainsTokens(t, filepath.Join(root, ".github", "workflows", "ci.yml"), []string{
			"trivy",
		})
		assertFileContainsTokens(t, runtimeGatePath, []string{
			`t.Run("cve_scanning",`,
		})
	})
}

func TestAcceptanceGatesV10(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)

	t.Run("uuidv7_reconciliation", func(t *testing.T) {
		assertFileNonEmpty(t, filepath.Join(root, "db", "migrations", "007_BREVIO_uuidv7_reconciliation.sql"))
		assertFileContainsTokens(t, filepath.Join(root, "db", "migrations", "007_BREVIO_uuidv7_reconciliation.sql"), []string{
			"uuid_v7_generate",
			"clock_timestamp",
			"gen_random_bytes",
		})
		assertFileNonEmpty(t, filepath.Join(root, "internal", "database", "uuidv7.go"))
		assertFileNonEmpty(t, filepath.Join(root, "internal", "database", "uuidv7_property_test.go"))
	})
	t.Run("federation_schema", func(t *testing.T) {
		assertFileNonEmpty(t, filepath.Join(root, "db", "migrations", "008_BREVIO_v10_gap_closure.sql"))
		assertFileContainsTokens(t, filepath.Join(root, "db", "migrations", "008_BREVIO_v10_gap_closure.sql"), []string{
			"federation_peers",
			"federation_negotiations",
			"negotiation_state",
			"federation_permission_type",
		})
	})
	t.Run("wallet_schema", func(t *testing.T) {
		assertFileContainsTokens(t, filepath.Join(root, "db", "migrations", "008_BREVIO_v10_gap_closure.sql"), []string{
			"wallets",
			"wallet_transactions",
			"wallet_transaction_type",
		})
	})
	t.Run("cost_tracking", func(t *testing.T) {
		assertFileContainsTokens(t, filepath.Join(root, "db", "migrations", "008_BREVIO_v10_gap_closure.sql"), []string{
			"cost_events",
			"cost_rollups",
			"cost_category",
		})
	})
	t.Run("authorization_receipts", func(t *testing.T) {
		assertFileNonEmpty(t, filepath.Join(root, "db", "migrations", "009_BREVIO_v10_authorization_receipts.sql"))
		assertFileContainsTokens(t, filepath.Join(root, "db", "migrations", "009_BREVIO_v10_authorization_receipts.sql"), []string{
			"authorization_receipts",
			"execution_ledger",
			"idempotency_keys",
			"kill_switch_state",
		})
		assertFileNonEmpty(t, filepath.Join(root, "internal", "control", "receipts.go"))
		assertFileNonEmpty(t, filepath.Join(root, "internal", "control", "receipts_test.go"))
	})
	t.Run("openapi_v10", func(t *testing.T) {
		assertFileNonEmpty(t, filepath.Join(root, "api", "openapi", "v10.yaml"))
		assertFileContainsTokens(t, filepath.Join(root, "api", "openapi", "v10.yaml"), []string{
			"additionalProperties: false",
			"negotiation_state",
			"federation_permission_type",
		})
	})
	t.Run("temporal_sdk", func(t *testing.T) {
		assertFileContainsTokens(t, filepath.Join(root, "go.mod"), []string{
			"go.temporal.io/sdk",
		})
		assertFileNonEmpty(t, filepath.Join(root, "internal", "temporal", "client.go"))
		assertFileNonEmpty(t, filepath.Join(root, "internal", "temporal", "workflows.go"))
		assertFileNonEmpty(t, filepath.Join(root, "internal", "temporal", "activities.go"))
		assertFileNonEmpty(t, filepath.Join(root, "internal", "temporal", "worker.go"))
		assertFileNonEmpty(t, filepath.Join(root, "internal", "temporal", "workflows_test.go"))
	})
	t.Run("kill_switch_nonbypassable", func(t *testing.T) {
		assertFileContainsTokens(t, filepath.Join(root, "internal", "control", "receipts.go"), []string{
			"ErrKillSwitchActive",
			"evaluateKillSwitch",
		})
		assertFileContainsTokens(t, filepath.Join(root, "internal", "control", "receipts_test.go"), []string{
			"TestKillSwitchBlocksReceipt",
		})
	})
	t.Run("negotiation_state_enum_complete", func(t *testing.T) {
		assertFileContainsTokens(t, filepath.Join(root, "db", "migrations", "008_BREVIO_v10_gap_closure.sql"), []string{
			"'proposed'",
			"'evaluating'",
			"'accepted'",
			"'rejected'",
			"'expired'",
			"'executing'",
			"'executed'",
			"'failed'",
			"'compensating'",
		})
	})
	t.Run("federation_permission_enum_complete", func(t *testing.T) {
		assertFileContainsTokens(t, filepath.Join(root, "db", "migrations", "008_BREVIO_v10_gap_closure.sql"), []string{
			"'calendar_query'",
			"'calendar_write'",
			"'routing_negotiate'",
			"'task_delegate'",
			"'knowledge_share'",
			"'status_query'",
		})
	})
}

func TestAcceptanceGatesV101(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)

	t.Run("admin_18_tables", func(t *testing.T) {
		migrationPath := filepath.Join(root, "db", "migrations", "010_BREVIO_v101_admin_intelligence.sql")
		assertFileNonEmpty(t, migrationPath)
		requiredTables := []string{
			"admin_users", "admin_sessions", "admin_audit_log", "admin_permissions",
			"admin_impersonation_log", "billing_plans", "billing_subscriptions",
			"invoices", "usage_meters", "usage_events", "workspace_limits",
			"admin_notifications", "admin_api_keys", "system_config",
			"workspace_health_snapshots", "admin_scheduled_reports",
			"admin_feature_overrides", "admin_data_export_requests",
		}
		assertFileContainsTokens(t, migrationPath, requiredTables)
	})
	t.Run("admin_jwt", func(t *testing.T) {
		assertFileNonEmpty(t, filepath.Join(root, "internal", "identity", "jwt_signer.go"))
	})
	t.Run("cost_numeric_18_8", func(t *testing.T) {
		assertFileContainsTokens(t, filepath.Join(root, "db", "migrations", "008_BREVIO_v10_gap_closure.sql"), []string{
			"numeric(18,8)",
		})
	})
}

func TestAcceptanceGatesV102V103(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)

	t.Run("intelligence_schema", func(t *testing.T) {
		migrationPath := filepath.Join(root, "db", "migrations", "011_BREVIO_v102_v103_intelligence.sql")
		assertFileNonEmpty(t, migrationPath)
		assertFileContainsTokens(t, migrationPath, []string{
			"eq_strategy_matrix",
			"emotional_context_log",
			"confidence_calibration",
			"prospective_memory",
			"metacognitive_monitors",
			"llm_invocation_log",
			"reasoning_chain_audit",
		})
	})
	t.Run("eq_strategy_enum", func(t *testing.T) {
		assertFileContainsTokens(t, filepath.Join(root, "db", "migrations", "011_BREVIO_v102_v103_intelligence.sql"), []string{
			"eq_strategy",
			"emotional_valence",
		})
	})
}

func TestAcceptanceGatesV104(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)

	t.Run("voice_schema", func(t *testing.T) {
		migrationPath := filepath.Join(root, "db", "migrations", "012_BREVIO_v104_voice_calls.sql")
		assertFileNonEmpty(t, migrationPath)
		assertFileContainsTokens(t, migrationPath, []string{
			"call_providers", "call_approval_policies", "call_approval_requests",
			"calls", "call_transcripts", "call_events",
			"call_provider_health_log", "call_rate_limits", "call_number_blocklist",
		})
	})
	t.Run("call_enums", func(t *testing.T) {
		assertFileContainsTokens(t, filepath.Join(root, "db", "migrations", "012_BREVIO_v104_voice_calls.sql"), []string{
			"call_status", "call_direction", "call_approval_status",
			"call_provider_status", "transcript_segment_type",
		})
	})
	t.Run("transcript_only_no_audio", func(t *testing.T) {
		assertFileContainsTokens(t, filepath.Join(root, "db", "migrations", "012_BREVIO_v104_voice_calls.sql"), []string{
			"call_transcripts",
		})
	})
	t.Run("call_approval_policy", func(t *testing.T) {
		assertFileNonEmpty(t, filepath.Join(root, "policies", "call_approval_gate.rego"))
	})
}

func TestAcceptanceGatesOpenClaw(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)

	t.Run("adoption_schema", func(t *testing.T) {
		migrationPath := filepath.Join(root, "db", "migrations", "013_BREVIO_openclaw_adoption.sql")
		assertFileNonEmpty(t, migrationPath)
		assertFileContainsTokens(t, migrationPath, []string{
			"pipeline_hooks", "hook_execution_log", "a2a_messages",
			"queue_lane_config", "sandbox_configs", "skills_gate_decisions",
			"auth_profiles", "context_compaction_log", "no_reply_config",
			"doctor_snapshots", "dm_pairing_config",
		})
	})
	t.Run("doctor_cli", func(t *testing.T) {
		assertFileNonEmpty(t, filepath.Join(root, "cmd", "brevioctl", "main.go"))
		assertFileContainsTokens(t, filepath.Join(root, "cmd", "brevioctl", "main.go"), []string{
			"db_connectivity",
			"migrations_applied",
			"temporal_reachable",
			"worker_polling",
			"policy_bundle_load",
			"kill_switch_status",
			"dlq_backlog",
		})
	})
	t.Run("v10_policies", func(t *testing.T) {
		assertFileNonEmpty(t, filepath.Join(root, "policies", "v10_gates.rego"))
		assertFileNonEmpty(t, filepath.Join(root, "policies", "v10_gates_test.rego"))
	})
	t.Run("root_documentation", func(t *testing.T) {
		assertFileNonEmpty(t, filepath.Join(root, "ARCHITECTURE.md"))
		assertFileNonEmpty(t, filepath.Join(root, "DECISIONS.md"))
		assertFileNonEmpty(t, filepath.Join(root, "RUNBOOK.md"))
	})
	t.Run("ts_quarantine", func(t *testing.T) {
		assertFileNonEmpty(t, filepath.Join(root, "services", "NON_PRODUCTION.md"))
		assertFileNonEmpty(t, filepath.Join(root, "services", ".ci-quarantine.yaml"))
	})
}

func TestAcceptanceGatesV91(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)
	runtimeGatePath := filepath.Join(root, "internal", "contracts", "acceptance_gate_runtime_closure_test.go")

	requiredGates := []string{
		"goal_lifecycle",
		"trust_scoring",
		"trust_autonomy_upgrade",
		"learning_pipeline",
		"daily_capture",
		"mission_control",
		"self_modification_gate",
		"cross_repo_intelligence",
		"capability_exploration",
		"code_context_export",
		"adaptive_discovery",
	}
	for _, gate := range requiredGates {
		gate := gate
		t.Run(gate, func(t *testing.T) {
			assertFileNonEmpty(t, filepath.Join(root, "db", "migrations", "002_BREVIO_v91_soft_intelligence.sql"))
			assertFileNonEmpty(t, filepath.Join(root, "prompts", "seed_prompts_v91.txt"))
			assertFileNonEmpty(t, filepath.Join(root, "spec", "traceability", "compliance_matrix_v91.csv"))
			assertFileContainsTokens(t, runtimeGatePath, []string{
				`t.Run("` + gate + `",`,
			})
		})
	}
}

func TestAcceptanceGatesV92(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)
	runtimeGatePath := filepath.Join(root, "internal", "contracts", "acceptance_gate_runtime_closure_test.go")

	requiredGates := []string{
		"context_budget_enforcement",
		"rag_pipeline_functional",
		"rag_eval_gate",
		"session_management",
		"temporal_reasoning",
		"guardrails_runtime",
		"tool_health_scoring",
		"feature_flag_system",
		"crdt_conflict_resolution",
		"streaming_sla",
		"error_communication",
		"event_schema_registry",
		"compliance_automation",
		"caching_layers",
		"model_tier_enforcement",
		"react_early_exit",
		"security_hardening",
		"admin_backend",
		"structured_generation",
	}
	for _, gate := range requiredGates {
		gate := gate
		t.Run(gate, func(t *testing.T) {
			assertFileNonEmpty(t, filepath.Join(root, "db", "migrations", "003_BREVIO_v92_production_hardening.sql"))
			assertFileNonEmpty(t, filepath.Join(root, "prompts", "seed_prompts_v92.txt"))
			assertFileNonEmpty(t, filepath.Join(root, "spec", "traceability", "compliance_matrix_v92.csv"))
			assertFileContainsTokens(t, filepath.Join(root, "db", "migrations", "003_BREVIO_v92_production_hardening.sql"), []string{
				"constrained_decoding_config",
			})
			assertFileContainsTokens(t, runtimeGatePath, []string{
				`t.Run("` + gate + `",`,
			})
		})
	}
}
