package contracts

import (
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestSecurityVulnBaselineClosure(t *testing.T) {
	t.Parallel()

	root := repositoryRoot(t)
	assertFileNonEmpty(t, filepath.Join(root, "scripts", "security", "govuln_allowlist.txt"))
	assertFileNonEmpty(t, filepath.Join(root, "scripts", "security", "trivy_allowlist.txt"))
	assertFileContainsTokens(t, filepath.Join(root, "scripts", "security", "run_govulncheck.sh"), []string{
		"govuln_allowlist.txt",
		"govulncheck -show verbose ./...",
		"new/unallowlisted vulnerability IDs detected",
	})
	assertFileContainsTokens(t, filepath.Join(root, "scripts", "security", "run_security_validation.sh"), []string{
		"govulncheck baseline",
		"bash scripts/security/run_govulncheck.sh",
		"TRIVY_ALLOWLIST_PATH",
		"python3 scripts/security/check_trivy_report.py",
	})
	assertFileContainsTokens(t, filepath.Join(root, ".github", "workflows", "ci.yaml"), []string{
		"govulncheck baseline",
		"bash scripts/security/run_govulncheck.sh",
	})
	assertGovulnAllowlistFormat(t, filepath.Join(root, "scripts", "security", "govuln_allowlist.txt"))
	assertTrivyAllowlistFormat(t, filepath.Join(root, "scripts", "security", "trivy_allowlist.txt"))
	assertFileContainsTokens(t, filepath.Join(root, "docs", "SECURITY_VULNERABILITY_BASELINE.md"), []string{
		"govuln_allowlist.txt",
		"trivy_allowlist.txt",
		"Go 1.22",
		"govulncheck",
		"CVE-2025-22869",
	})
}

func assertGovulnAllowlistFormat(t *testing.T, path string) {
	t.Helper()

	content := readFileString(t, path)
	lines := strings.Split(content, "\n")
	pattern := regexp.MustCompile(`^GO-\d{4}-\d{4,5}$`)

	seen := map[string]struct{}{}
	count := 0
	for lineNum, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		count++
		if !pattern.MatchString(trimmed) {
			t.Fatalf("invalid govuln allowlist id at %s:%d: %q", path, lineNum+1, trimmed)
		}
		if _, exists := seen[trimmed]; exists {
			t.Fatalf("duplicate govuln allowlist id %q in %s", trimmed, path)
		}
		seen[trimmed] = struct{}{}
	}
	if count == 0 {
		t.Fatalf("govuln allowlist has no vulnerability ids: %s", path)
	}
}

func assertTrivyAllowlistFormat(t *testing.T, path string) {
	t.Helper()

	content := readFileString(t, path)
	lines := strings.Split(content, "\n")
	pattern := regexp.MustCompile(`^CVE-\d{4}-\d{4,}$`)

	seen := map[string]struct{}{}
	count := 0
	for lineNum, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		count++
		if !pattern.MatchString(trimmed) {
			t.Fatalf("invalid trivy allowlist id at %s:%d: %q", path, lineNum+1, trimmed)
		}
		if _, exists := seen[trimmed]; exists {
			t.Fatalf("duplicate trivy allowlist id %q in %s", trimmed, path)
		}
		seen[trimmed] = struct{}{}
	}
	if count == 0 {
		t.Fatalf("trivy allowlist has no vulnerability ids: %s", path)
	}
}
