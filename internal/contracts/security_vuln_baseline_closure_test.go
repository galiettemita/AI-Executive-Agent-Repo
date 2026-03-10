package contracts

import (
	"path/filepath"
	"regexp"
	"strconv"
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
	assertFileContainsTokens(t, filepath.Join(root, ".github", "workflows", "ci.yml"), []string{
		"Security Scan",
		"run_security_validation.sh",
	})
	assertGovulnAllowlistFormat(t, filepath.Join(root, "scripts", "security", "govuln_allowlist.txt"))
	assertTrivyAllowlistFormat(t, filepath.Join(root, "scripts", "security", "trivy_allowlist.txt"))
	assertFileContainsTokens(t, filepath.Join(root, "docs", "SECURITY_VULNERABILITY_BASELINE.md"), []string{
		"govuln_allowlist.txt",
		"trivy_allowlist.txt",
		"Go 1.23",
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

func TestGoToolchainCryptoCompatibilityConstraint(t *testing.T) {
	t.Parallel()

	root := repositoryRoot(t)
	goModPath := filepath.Join(root, "go.mod")
	goMod := readFileString(t, goModPath)

	if !strings.Contains(goMod, "\ngo 1.23") {
		t.Fatalf("go.mod toolchain changed; expected go 1.23 baseline for this release line: %s", goModPath)
	}

	versionPattern := regexp.MustCompile(`golang\.org/x/crypto\s+v(\d+)\.(\d+)\.(\d+)`)
	match := versionPattern.FindStringSubmatch(goMod)
	if len(match) != 4 {
		t.Fatalf("missing golang.org/x/crypto entry in go.mod: %s", goModPath)
	}

	major, _ := strconv.Atoi(match[1])
	minor, _ := strconv.Atoi(match[2])
	patch, _ := strconv.Atoi(match[3])
	if major != 0 {
		t.Fatalf("unexpected golang.org/x/crypto major version in go.mod: %s", match[0])
	}
	if minor >= 40 {
		t.Fatalf("golang.org/x/crypto %d.%d.%d may require Go >= 1.24; Go 1.23 release line must pin below v0.40.0", major, minor, patch)
	}
}
