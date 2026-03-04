package contracts

import (
	"path/filepath"
	"testing"
)

func TestSecurityScanWorkflowClosure(t *testing.T) {
	t.Parallel()

	root := repositoryRoot(t)
	workflowPath := filepath.Join(root, ".github", "workflows", "security-scan.yml")

	assertFileContainsTokens(t, workflowPath, []string{
		"name: security-scan",
		"pull_request:",
		"schedule:",
		"security-events: write",
		"actions/setup-go@v5",
		"actions/setup-node@v4",
		"actions/setup-python@v5",
		"semgrep scan --config auto --severity ERROR --sarif",
		"pnpm audit --audit-level high",
		"bash scripts/security/run_security_validation.sh",
		"bash scripts/security/run_govulncheck.sh",
		"github/codeql-action/upload-sarif@v3",
		"actions/upload-artifact@v4",
	})
}
