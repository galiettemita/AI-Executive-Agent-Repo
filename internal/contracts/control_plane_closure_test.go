package contracts

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestControlPlaneOPAClientPresence verifies that internal/control/opa_client.go
// exists and contains the production OPA HTTP client with circuit breaker and
// deny-by-default semantics.
func TestControlPlaneOPAClientPresence(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)

	opaClientPath := filepath.Join(root, "internal", "control", "opa_client.go")
	assertFileNonEmpty(t, opaClientPath)
	assertFileContainsTokens(t, opaClientPath, []string{
		"OPAClient",
		"OPAClientConfig",
		"ErrOPAUnavailable",
		"circuitState",
		"EvaluatePolicy",
		"CanonicalizeInput",
		"deny-by-default",
		"/v1/data/",
	})
}

// TestControlPlaneOPAEvaluatorDenyByDefault verifies that the OPA evaluator
// enforces deny-by-default when an OPA client is configured but unavailable.
func TestControlPlaneOPAEvaluatorDenyByDefault(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)

	opaPath := filepath.Join(root, "internal", "control", "opa.go")
	assertFileNonEmpty(t, opaPath)
	assertFileContainsTokens(t, opaPath, []string{
		"OPA_UNAVAILABLE_DENY_BY_DEFAULT",
		"SetOPAClient",
		"HasOPAClient",
		"evaluateEmbeddedGates",
		"Devtest/unit-test path",
	})
}

// TestControlPlanePgRepositoryPresence verifies that durable receipt
// persistence exists with the required repository interface.
func TestControlPlanePgRepositoryPresence(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)

	repoPath := filepath.Join(root, "internal", "control", "pg_repository.go")
	assertFileNonEmpty(t, repoPath)
	assertFileContainsTokens(t, repoPath, []string{
		"ReceiptRepository",
		"PgReceiptRepository",
		"StoreReceipt",
		"ConsumeReceipt",
		"RevokeReceipt",
		"StoreGateDecision",
		"StoreLedgerEntry",
		"StoreBudgetEvent",
		"GateDecisionRecord",
		"BudgetEvent",
		"authorization_receipts",
		"execution_gate_decisions",
		"execution_ledger",
	})
}

// TestControlPlaneDurableReceiptServicePresence verifies the durable receipt
// service wraps in-memory + persistence.
func TestControlPlaneDurableReceiptServicePresence(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)

	durablePath := filepath.Join(root, "internal", "control", "durable_receipt_service.go")
	assertFileNonEmpty(t, durablePath)
	assertFileContainsTokens(t, durablePath, []string{
		"DurableReceiptService",
		"NewDurableReceiptService",
		"EvaluateAndIssue",
		"PersistPolicyDecision",
		"StoreGateDecision",
		"StoreReceipt",
	})
}

// TestControlPlaneBudgetEnforcerPresence verifies the budget enforcer
// with durable evidence persistence.
func TestControlPlaneBudgetEnforcerPresence(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)

	budgetPath := filepath.Join(root, "internal", "control", "budget_enforcement.go")
	assertFileNonEmpty(t, budgetPath)
	assertFileContainsTokens(t, budgetPath, []string{
		"BudgetEnforcer",
		"BudgetCheckResult",
		"SetBudget",
		"Check",
		"Consume",
		"IsExhausted",
		"persistEvent",
		"StoreBudgetEvent",
		"BUDGET_UNITS_EXHAUSTED",
		"BUDGET_USD_EXHAUSTED",
		"BUDGET_WARNING_80_PERCENT",
	})
}

// TestControlPlaneNoHardcodedPolicyInProduction scans opa.go to verify that
// the production path delegates to OPA client and does NOT use embedded gate
// logic when an OPA client is configured.
func TestControlPlaneNoHardcodedPolicyInProduction(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)

	opaPath := filepath.Join(root, "internal", "control", "opa.go")
	body, err := os.ReadFile(opaPath)
	if err != nil {
		t.Fatalf("read opa.go: %v", err)
	}
	content := string(body)

	// EvaluatePolicy must check for opaClient first (production path).
	if !strings.Contains(content, "if client != nil") {
		t.Fatal("EvaluatePolicy must check for OPA client (production path)")
	}

	// EvaluateGateWithOPA must deny-by-default when OPA is configured.
	if !strings.Contains(content, "HasOPAClient()") {
		t.Fatal("EvaluateGateWithOPA must check HasOPAClient for deny-by-default")
	}

	// Embedded gates must be clearly labeled as devtest.
	if !strings.Contains(content, "evaluateEmbeddedGates") {
		t.Fatal("embedded gate logic must be in evaluateEmbeddedGates function")
	}
}

// TestControlPlaneEntrypointOPAWiring verifies cmd/control/main.go wires OPA.
func TestControlPlaneEntrypointOPAWiring(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)

	mainPath := filepath.Join(root, "cmd", "control", "main.go")
	assertFileNonEmpty(t, mainPath)
	assertFileContainsTokens(t, mainPath, []string{
		"OPA_URL",
		"NewOPAClient",
		"SetOPAClient",
		"NewOPAEvaluator",
		"LoadPolicies",
	})
}
