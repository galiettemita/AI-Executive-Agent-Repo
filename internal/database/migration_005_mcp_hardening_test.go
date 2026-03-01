package database

import (
	"strings"
	"testing"
)

func TestMigration005MCPExecutionOAuthHardeningClosure(t *testing.T) {
	t.Parallel()

	sql := readMigrationSQL(t, "005_BREVIO_mcp_execution_oauth_hardening.sql")
	expectedTokens := []string{
		"ALTER TABLE tool_executions",
		"ADD COLUMN IF NOT EXISTS is_mcp",
		"ADD COLUMN IF NOT EXISTS mcp_server_id",
		"ADD COLUMN IF NOT EXISTS content_provenance",
		"tool_executions_content_provenance_check",
		"native_result",
		"mcp_result",
		"ALTER TABLE user_oauth_tokens",
		"ADD COLUMN IF NOT EXISTS provider",
		"idx_tool_executions_is_mcp",
		"idx_tool_executions_mcp_server_id",
		"idx_user_oauth_tokens_provider",
	}
	for _, token := range expectedTokens {
		if !strings.Contains(strings.ToLower(sql), strings.ToLower(token)) {
			t.Fatalf("migration 005 missing token %q", token)
		}
	}
}
