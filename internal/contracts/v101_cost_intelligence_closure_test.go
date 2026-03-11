package contracts

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestNNR104_CostEventsViaOutbox verifies that hot-path activities never perform
// blocking writes to cost ledger tables. Cost events must flow through the outbox.
func TestNNR104_CostEventsViaOutbox(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)

	// The outbox producers must exist and use the outbox service.
	activitiesV101 := filepath.Join(root, "internal", "temporal", "activities_v101.go")
	assertFileNonEmpty(t, activitiesV101)
	assertFileContainsTokens(t, activitiesV101, []string{
		"EnqueueLLMCostActivity",
		"EnqueueConnectorCostActivity",
		"outboxSvc",
		"NNR-104",
	})

	// Cost event types must be defined.
	costEvents := filepath.Join(root, "internal", "admin", "cost_events.go")
	assertFileNonEmpty(t, costEvents)
	assertFileContainsTokens(t, costEvents, []string{
		"BREVIO.cost.llm.v1",
		"BREVIO.cost.connector.v1",
		"BREVIO.cost.tool.v1",
	})

	// The hot-path workflow (MessageProcessingWorkflow) must NOT contain direct
	// INSERT INTO llm_cost_ledger or connector_cost_ledger statements.
	workflowsFile := filepath.Join(root, "internal", "temporal", "workflows.go")
	workflowContent, err := os.ReadFile(workflowsFile)
	if err != nil {
		t.Fatalf("failed to read workflows.go: %v", err)
	}
	forbidden := []string{
		"INSERT INTO llm_cost_ledger",
		"INSERT INTO connector_cost_ledger",
		"INSERT INTO user_cost_daily_rollup",
		"INSERT INTO task_cost_rollup",
	}
	for _, pattern := range forbidden {
		if strings.Contains(string(workflowContent), pattern) {
			t.Errorf("NNR-104 violation: workflows.go contains blocking write pattern %q", pattern)
		}
	}
}

// TestNNR105_OnlyTemporalWritesRollups verifies that rollup tables are only
// written by Temporal activities, never by admin handlers or HTTP services.
func TestNNR105_OnlyTemporalWritesRollups(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)

	// The AggregateCostsActivity must write to rollup tables.
	activitiesPath := filepath.Join(root, "internal", "temporal", "activities.go")
	assertFileContainsTokens(t, activitiesPath, []string{
		"AggregateCostsActivity",
		"user_cost_daily_rollup",
		"NNR-105",
	})

	// Admin handlers must NOT contain INSERT/UPDATE to rollup tables.
	handlersFile := filepath.Join(root, "internal", "admin", "handlers.go")
	handlersContent, err := os.ReadFile(handlersFile)
	if err != nil {
		t.Fatalf("failed to read handlers.go: %v", err)
	}
	rollupForbidden := []string{
		"INSERT INTO user_cost_daily_rollup",
		"INSERT INTO task_cost_rollup",
		"UPDATE user_cost_daily_rollup",
		"UPDATE task_cost_rollup",
	}
	for _, pattern := range rollupForbidden {
		if strings.Contains(string(handlersContent), pattern) {
			t.Errorf("NNR-105 violation: admin handlers.go contains rollup write %q", pattern)
		}
	}

	// The cost_attribution.go in-memory service must NOT write to rollup tables.
	costAttrFile := filepath.Join(root, "internal", "admin", "cost_attribution.go")
	costAttrContent, err := os.ReadFile(costAttrFile)
	if err != nil {
		t.Fatalf("failed to read cost_attribution.go: %v", err)
	}
	for _, pattern := range rollupForbidden {
		if strings.Contains(string(costAttrContent), pattern) {
			t.Errorf("NNR-105 violation: cost_attribution.go contains rollup write %q", pattern)
		}
	}
}

// TestNNR106_AdminReadsPreAggregatedOnly verifies that admin read endpoints
// query pre-aggregated rollup tables, never raw ledger tables.
func TestNNR106_AdminReadsPreAggregatedOnly(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)

	// The PgCostRepository must exist and implement read-only methods.
	pgCostRepo := filepath.Join(root, "internal", "admin", "pg_cost_repository.go")
	assertFileNonEmpty(t, pgCostRepo)
	assertFileContainsTokens(t, pgCostRepo, []string{
		"CostReadRepository",
		"GetCostSummary",
		"GetDailyRollups",
		"GetTaskRollups",
		"GetCostProjections",
		"NNR-106",
	})

	// The read repository must query rollup tables, not raw ledgers.
	pgCostContent, err := os.ReadFile(pgCostRepo)
	if err != nil {
		t.Fatalf("failed to read pg_cost_repository.go: %v", err)
	}

	// Read methods must reference rollup tables.
	rollupTables := []string{
		"user_cost_daily_rollup",
		"task_cost_rollup",
	}
	for _, table := range rollupTables {
		if !strings.Contains(string(pgCostContent), table) {
			t.Errorf("NNR-106 violation: pg_cost_repository.go does not query rollup table %q", table)
		}
	}

	// The PgRevenueRepository must exist with read methods.
	pgRevRepo := filepath.Join(root, "internal", "admin", "pg_revenue_repository.go")
	assertFileNonEmpty(t, pgRevRepo)
	assertFileContainsTokens(t, pgRevRepo, []string{
		"RevenueReadRepository",
		"GetMRRSnapshot",
		"GetMarginReport",
	})
}

// TestV101_CostRollupWorkflowRegistered verifies the CostRollupWorkflow is
// registered on the worker and wired to AggregateCostsActivity.
func TestV101_CostRollupWorkflowRegistered(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)

	workerFile := filepath.Join(root, "internal", "temporal", "worker.go")
	assertFileContainsTokens(t, workerFile, []string{
		"CostRollupWorkflow",
		"AggregateCostsActivity",
		"SubscriptionReconciliationWorkflow",
	})
}

// TestV101_OutboxProducerActivitiesRegistered verifies V10.1 activities are registered.
func TestV101_OutboxProducerActivitiesRegistered(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)

	workerFile := filepath.Join(root, "internal", "temporal", "worker.go")
	assertFileContainsTokens(t, workerFile, []string{
		"EnqueueLLMCostActivity",
		"EnqueueConnectorCostActivity",
		"IngestSubscriptionEventActivity",
		"ReconcileMRRActivity",
		"WriteLedgerFromOutboxActivity",
	})
}

// TestV101_SubscriptionEventIdempotency verifies the subscription event
// ingestion activity handles idempotency via stripe_event_id.
func TestV101_SubscriptionEventIdempotency(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)

	activitiesV101 := filepath.Join(root, "internal", "temporal", "activities_v101.go")
	assertFileContainsTokens(t, activitiesV101, []string{
		"IngestSubscriptionEventActivity",
		"stripe_event_id",
		"ON CONFLICT",
		"Duplicate",
	})
}

// TestV101_MigrationTablesExist verifies migration 015 defines required tables.
func TestV101_MigrationTablesExist(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)

	migrationPath := filepath.Join(root, "db", "migrations", "015_BREVIO_v101_cost_revenue_intelligence.sql")
	assertFileContainsTokens(t, migrationPath, []string{
		"llm_cost_ledger",
		"connector_cost_ledger",
		"task_cost_rollup",
		"user_cost_daily_rollup",
		"subscription_events",
		"mrr_snapshots",
		"operator_margin_report",
		"NUMERIC(18,8)",
	})
}

// TestV101_OutboxProducerDoesNotBlockWorkflow verifies the outbox producer
// activities don't perform direct ledger writes.
func TestV101_OutboxProducerDoesNotBlockWorkflow(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)

	// The EnqueueLLMCostActivity and EnqueueConnectorCostActivity must NOT
	// contain direct INSERT INTO ledger statements.
	activitiesV101 := filepath.Join(root, "internal", "temporal", "activities_v101.go")
	content, err := os.ReadFile(activitiesV101)
	if err != nil {
		t.Fatalf("failed to read activities_v101.go: %v", err)
	}

	// The file should reference outboxSvc.Enqueue, not direct ledger inserts.
	if !strings.Contains(string(content), "outboxSvc.Enqueue") {
		t.Error("NNR-104 violation: activities_v101.go does not use outboxSvc.Enqueue for cost events")
	}

	// WriteLedgerFromOutboxActivity is allowed to write (it's the consumer, not hot path).
	// But the Enqueue* activities must not.
	lines := strings.Split(string(content), "\n")
	inEnqueueFunc := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "func (a *Activities) Enqueue") {
			inEnqueueFunc = true
		}
		if inEnqueueFunc && strings.HasPrefix(trimmed, "func ") && !strings.HasPrefix(trimmed, "func (a *Activities) Enqueue") {
			inEnqueueFunc = false
		}
		if inEnqueueFunc && strings.Contains(line, "INSERT INTO llm_cost_ledger") {
			t.Error("NNR-104 violation: Enqueue* activity directly writes to llm_cost_ledger")
		}
		if inEnqueueFunc && strings.Contains(line, "INSERT INTO connector_cost_ledger") {
			t.Error("NNR-104 violation: Enqueue* activity directly writes to connector_cost_ledger")
		}
	}
}
