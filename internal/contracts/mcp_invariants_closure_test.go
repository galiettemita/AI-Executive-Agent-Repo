package contracts

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestBrainPlaneRemainsMCPAgnostic(t *testing.T) {
	t.Parallel()

	root := repositoryRoot(t)
	brainPath := filepath.Join(root, "cmd", "brain", "main.go")
	llmPath := filepath.Join(root, "internal", "llm", "service.go")
	assertFileNonEmpty(t, brainPath)
	assertFileNonEmpty(t, llmPath)

	brainBody := readFileString(t, brainPath)
	if strings.Contains(strings.ToLower(brainBody), "mcp") {
		t.Fatalf("brain service must remain mcp-agnostic: %s", brainPath)
	}
	llmBody := readFileString(t, llmPath)
	if strings.Contains(strings.ToLower(llmBody), "mcp") {
		t.Fatalf("llm service must remain mcp-agnostic: %s", llmPath)
	}
}

func TestSharedToolRegistryAndMCPAuthMatrixClosure(t *testing.T) {
	t.Parallel()

	root := repositoryRoot(t)
	registryPath := filepath.Join(root, "internal", "mcp", "service.go")
	assertFileNonEmpty(t, registryPath)
	assertFileContainsTokens(t, registryPath, []string{
		"type ToolSpec struct",
		"type Service struct",
		"RegisterTool",
		"ResolveTool",
		"AuthMatrixCoverage",
		"AuthOAuth2",
		"AuthAPIKey",
		"AuthPAT",
		"AuthIntegrationToken",
	})
}

func TestMCPExecutionProvenanceAndToolExecutionPathClosure(t *testing.T) {
	t.Parallel()

	root := repositoryRoot(t)
	executorPath := filepath.Join(root, "internal", "executor", "service.go")
	assertFileContainsTokens(t, executorPath, []string{
		"IsMCP",
		"MCPServerID",
		"ContentProvenance",
		"mcp_result",
		"native_result",
		"Executions() []ToolExecution",
	})
	integrationPath := filepath.Join(root, "internal", "integration", "service.go")
	assertFileContainsTokens(t, integrationPath, []string{
		"ResolveTool(toolKey)",
		"isMCPInvocation",
		"ContentProvenance: provenance",
		"s.mcp.RecordInvocation",
		"local-model",
	})
}

func TestMCPAndOAuthMigrationHardeningClosure(t *testing.T) {
	t.Parallel()

	root := repositoryRoot(t)
	migrationPath := filepath.Join(root, "db", "migrations", "005_BREVIO_mcp_execution_oauth_hardening.sql")
	assertFileNonEmpty(t, migrationPath)
	assertFileContainsTokens(t, migrationPath, []string{
		"ADD COLUMN IF NOT EXISTS is_mcp",
		"ADD COLUMN IF NOT EXISTS mcp_server_id",
		"ADD COLUMN IF NOT EXISTS content_provenance",
		"ADD COLUMN IF NOT EXISTS provider",
		"idx_tool_executions_is_mcp",
		"idx_user_oauth_tokens_provider",
	})
}
