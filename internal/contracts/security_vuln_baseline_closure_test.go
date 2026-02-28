package contracts

import "path/filepath"

import "testing"

func TestSecurityVulnBaselineClosure(t *testing.T) {
	t.Parallel()

	root := repositoryRoot(t)
	assertFileNonEmpty(t, filepath.Join(root, "scripts", "security", "govuln_allowlist.txt"))
	assertFileContainsTokens(t, filepath.Join(root, "scripts", "security", "run_govulncheck.sh"), []string{
		"govuln_allowlist.txt",
		"govulncheck -show verbose ./...",
		"new/unallowlisted vulnerability IDs detected",
	})
	assertFileContainsTokens(t, filepath.Join(root, "scripts", "security", "run_security_validation.sh"), []string{
		"govulncheck baseline",
		"bash scripts/security/run_govulncheck.sh",
	})
	assertFileContainsTokens(t, filepath.Join(root, ".github", "workflows", "ci.yaml"), []string{
		"govulncheck baseline",
		"bash scripts/security/run_govulncheck.sh",
	})
}
