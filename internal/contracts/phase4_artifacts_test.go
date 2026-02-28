package contracts

import (
	"path/filepath"
	"testing"
)

func TestPhase4ReadinessArtifactsExist(t *testing.T) {
	t.Parallel()

	root := repositoryRoot(t)
	required := []string{
		filepath.Join(root, "evals", "load", "k6_interactive_turn.js"),
		filepath.Join(root, "evals", "load", "README.md"),
		filepath.Join(root, "scripts", "security", "run_security_validation.sh"),
		filepath.Join(root, "scripts", "infra", "validate.sh"),
	}
	for _, path := range required {
		assertFileNonEmpty(t, path)
	}

	assertFileContainsTokens(t, filepath.Join(root, "evals", "load", "k6_interactive_turn.js"), []string{
		"rate<0.005",
		"p(95)<2500",
		"crypto.hmac('sha256'",
		"WEBHOOK_SECRET",
		"X-Signature",
	})
	assertFileContainsTokens(t, filepath.Join(root, "scripts", "security", "run_security_validation.sh"), []string{
		"prompt injection tests",
		"webhook signature tests",
		"ssrf protection tests",
		"govulncheck baseline",
	})
	assertFileContainsTokens(t, filepath.Join(root, "scripts", "infra", "validate.sh"), []string{
		"terraform validate modules",
		"helm lint charts",
	})
}
