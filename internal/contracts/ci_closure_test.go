package contracts

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestCIWorkflowClosure validates the authoritative CI workflow (ci.yml)
// contains all required pipeline stages. The workflow delegates to Makefile
// targets and scripts, so we validate the job structure rather than inline
// tool invocations.
func TestCIWorkflowClosure(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)
	ciPath := filepath.Join(root, ".github", "workflows", "ci.yml")

	assertFileContainsTokens(t, ciPath, []string{
		// Core pipeline stages
		"Lint & Format",
		"Schema Validate",
		"Unit Tests",
		"Integration Tests",
		"Contract Tests",
		"Security Scan",
		"Migration Safety",
		// V10+ gates
		"V10+ Acceptance Gates",
		// Build & deploy stages
		"Build & Push",
		"Deploy Staging",
		"Deploy Production",
		// Key test commands
		"go test ./internal/contracts",
		"go test ./internal/integration",
		"go test ./... -count=1",
		// Security tooling delegation
		"run_security_validation.sh",
		"verify_postgres_migrations.sh",
		// Lint via Makefile
		"make lint",
	})
}

// TestCIWorkflowNoSecuritySkipPaths ensures the authoritative CI does not
// contain tokens that allow silently skipping security checks.
func TestCIWorkflowNoSecuritySkipPaths(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)
	ciPath := filepath.Join(root, ".github", "workflows", "ci.yml")
	body, err := os.ReadFile(ciPath)
	if err != nil {
		t.Fatalf("read ci workflow: %v", err)
	}
	content := string(body)

	disallowed := []string{
		"trivy not installed on runner; skip",
		"trufflehog not installed on runner; skip",
		"syft not installed on runner; skip",
	}
	for _, token := range disallowed {
		if strings.Contains(content, token) {
			t.Fatalf("ci workflow contains disallowed security skip token: %q", token)
		}
	}
}
