package contracts

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestInfraValidationScriptCanonicalSets(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)
	path := filepath.Join(root, "scripts", "infra", "validate.sh")
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read infra validation script: %v", err)
	}
	content := string(body)

	assertScriptArraySetEquals(t, content, "required_terraform_modules", []string{
		"vpc",
		"eks",
		"rds",
		"elasticache",
		"sqs",
		"s3",
		"secrets",
		"temporal",
		"observability",
		"opensearch",
		"admin-frontend",
		"feature-flags-cache",
	})
	assertScriptArraySetEquals(t, content, "required_terraform_environments", []string{
		"staging",
		"production",
	})
	assertScriptArraySetEquals(t, content, "required_helm_charts", []string{
		"BREVIO-gateway",
		"BREVIO-brain",
		"BREVIO-control",
		"BREVIO-executor",
		"BREVIO-canvas",
		"BREVIO-temporal-worker",
		"BREVIO-admin-api",
		"BREVIO-admin-frontend",
		"BREVIO-rag-worker",
		"BREVIO-guardrails",
		"BREVIO-health-checker",
	})

	assertFileContainsTokens(t, path, []string{
		"REQUIRE_INFRA_TOOLS",
		"CI/strict mode",
		"terraform fmt -check -recursive",
		"terraform validate modules",
		"terraform validate environments",
		"helm lint charts",
		"helm template",
	})
}

func TestSecurityValidationScriptClosure(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)
	path := filepath.Join(root, "scripts", "security", "run_security_validation.sh")
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read security validation script: %v", err)
	}
	content := string(body)

	requiredGoTestTargets := []string{
		"go test ./internal/control -count=1",
		"go test ./internal/gateway -count=1",
		"go test ./internal/executor -count=1",
	}
	for _, token := range requiredGoTestTargets {
		if !strings.Contains(content, token) {
			t.Fatalf("security validation script missing go test target: %q", token)
		}
	}

	assertFileContainsTokens(t, path, []string{
		"REQUIRE_SECURITY_TOOLS",
		"CI/strict mode",
		"trivy fs --scanners vuln --severity CRITICAL,HIGH --exit-code 1 .",
		"trufflehog filesystem . --fail",
		"syft dir:. -o spdx-json > sbom.spdx.json",
		"bash scripts/security/run_govulncheck.sh",
	})
}

func TestScriptPortabilityAndFallbackClosure(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)

	infraScriptPath := filepath.Join(root, "scripts", "infra", "validate.sh")
	infraContent := readFileString(t, infraScriptPath)
	if strings.Contains(infraContent, "local -A") {
		t.Fatalf("infra validation script must remain Bash 3 compatible: %s", infraScriptPath)
	}
	assertFileContainsTokens(t, infraScriptPath, []string{
		"array_contains()",
		"assert_exact_dir_set()",
		"using dockerized terraform:1.9.8",
		"using dockerized alpine/helm:3.16.4",
		"hashicorp/terraform:1.9.8",
		"alpine/helm:3.16.4",
	})

	govulnScriptPath := filepath.Join(root, "scripts", "security", "run_govulncheck.sh")
	govulnContent := readFileString(t, govulnScriptPath)
	if strings.Contains(govulnContent, "mapfile ") {
		t.Fatalf("govulncheck script must remain Bash 3 compatible (no mapfile): %s", govulnScriptPath)
	}
	assertFileContainsTokens(t, govulnScriptPath, []string{
		"resolve_docker_bin()",
		"go toolchain unavailable; using dockerized go1.22 scanner",
		"run --rm -v \"$ROOT_DIR\":/src -w /src golang:1.22",
	})

	securityScriptPath := filepath.Join(root, "scripts", "security", "run_security_validation.sh")
	assertFileContainsTokens(t, securityScriptPath, []string{
		"run_go_cmd()",
		"run --rm -v \"$ROOT_DIR\":/src -w /src golang:1.22",
	})
}

func TestDatabaseVerificationScriptClosure(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)
	path := filepath.Join(root, "scripts", "database", "verify_postgres_migrations.sh")
	assertFileNonEmpty(t, path)
	assertFileContainsTokens(t, path, []string{
		"pgvector/pgvector:pg16",
		"001_BREVIO_v9_init.sql",
		"002_BREVIO_v91_soft_intelligence.sql",
		"003_BREVIO_v92_production_hardening.sql",
		"enum count mismatch",
		"RLS coverage failure",
		"SET ROLE brevio_app",
		"expected current_setting('app.workspace_id') to fail when unset",
		"cross-workspace isolation failed",
		"success: migrations apply cleanly",
	})
}

func assertScriptArraySetEquals(t *testing.T, content, variable string, expected []string) {
	t.Helper()
	pattern := regexp.MustCompile(`(?s)` + regexp.QuoteMeta(variable) + `=\((.*?)\)`)
	match := pattern.FindStringSubmatch(content)
	if len(match) < 2 {
		t.Fatalf("script array variable not found: %s", variable)
	}
	valueBlock := match[1]
	itemPattern := regexp.MustCompile(`"([^"]+)"`)
	actual := make([]string, 0)
	for _, hit := range itemPattern.FindAllStringSubmatch(valueBlock, -1) {
		if len(hit) < 2 {
			continue
		}
		actual = append(actual, hit[1])
	}
	assertStringSliceSetEquals(t, actual, expected, "script_array:"+variable)
}
