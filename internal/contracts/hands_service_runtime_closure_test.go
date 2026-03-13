package contracts

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHandsServiceRuntimeClosure(t *testing.T) {
	t.Parallel()

	root := repositoryRoot(t)
	handsSource := filepath.Join(root, "services", "hands-runtime", "src", "index.ts")
	handsReadme := filepath.Join(root, "services", "hands-runtime", "README.md")

	assertFileContainsTokens(t, handsSource, []string{
		"/api/v1/hands/execute",
		"/api/v1/hands/tool/execute",
		"/api/v1/hands/circuit-breakers",
		"CIRCUIT_OPEN",
		"EXTERNAL_TIMEOUT",
		"checkCircuitBeforeExecution",
		"markExecutionFailure",
		"SkillExecutionTimeoutError",
		"hands.execute.complete",
		"createHandsRuntime",
	})

	assertFileContainsTokens(t, handsReadme, []string{
		"Execution-plane service",
		"POST /api/v1/hands/execute",
		"POST /api/v1/hands/tool/execute",
		"circuit breaker",
		"BREVIO_HANDS_EXECUTION_TIMEOUT_MS",
	})

	body, err := os.ReadFile(handsReadme)
	if err != nil {
		t.Fatalf("read hands readme: %v", err)
	}
	if strings.Contains(strings.ToLower(string(body)), "scaffold directory") {
		t.Fatalf("hands README still contains scaffold marker")
	}
}
