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
		"terraform plan environments",
		"plan -refresh=false -lock=false -input=false -detailed-exitcode",
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
		"trivy_scan_args=(",
		"TRIVY_REPORT_PATH",
		"TRIVY_ALLOWLIST_PATH",
		"python3 scripts/security/check_trivy_report.py",
		"--skip-dirs .git",
		"trufflehog_scan_args=(",
		"--exclude-paths \"$TRUFFLEHOG_EXCLUDE_PATHS_FILE\"",
		"syft_scan_args=(",
		"\"**/.terraform/**\"",
		"syft \"${syft_scan_args[@]}\" > sbom.spdx.json",
		"bash scripts/security/run_govulncheck.sh",
	})

	excludePath := filepath.Join(root, "scripts", "security", "trufflehog_exclude_paths.txt")
	assertFileNonEmpty(t, excludePath)
	assertFileContainsTokens(t, excludePath, []string{
		"^\\.git/",
		"(^|/)\\.terraform/",
		"^classmate-ai/",
		"^artifacts/",
	})

	trivyAllowlistPath := filepath.Join(root, "scripts", "security", "trivy_allowlist.txt")
	assertFileNonEmpty(t, trivyAllowlistPath)
	assertFileContainsTokens(t, trivyAllowlistPath, []string{
		"CVE-2025-22869",
	})

	trivyCheckScriptPath := filepath.Join(root, "scripts", "security", "check_trivy_report.py")
	assertFileNonEmpty(t, trivyCheckScriptPath)
	assertFileContainsTokens(t, trivyCheckScriptPath, []string{
		"usage: check_trivy_report.py",
		"[trivy-check] blocking HIGH/CRITICAL vulnerabilities detected",
		"only allowlisted HIGH/CRITICAL vulnerabilities found",
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

func TestDeploymentRolloutScriptClosure(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)
	path := filepath.Join(root, "scripts", "deploy", "helm_rollout.sh")
	assertFileNonEmpty(t, path)
	assertFileContainsTokens(t, path, []string{
		"WAIT_FOR_ROLLOUT",
		"GATEWAY_IMAGE_REPOSITORY",
		"ADMIN_FRONTEND_IMAGE_REPOSITORY",
		"deploy_chart \"brevio-gateway\"",
		"deploy_chart \"brevio-brain\"",
		"deploy_chart \"brevio-control\"",
		"deploy_chart \"brevio-executor\"",
		"deploy_chart \"brevio-canvas\"",
		"deploy_chart \"brevio-temporal-worker\"",
		"deploy_chart \"brevio-admin-api\"",
		"deploy_chart \"brevio-admin-frontend\"",
		"deploy_chart \"brevio-rag-worker\"",
		"deploy_chart \"brevio-guardrails\"",
		"deploy_chart \"brevio-health-checker\"",
		"kubectl get ingress",
	})

	makefilePath := filepath.Join(root, "Makefile")
	assertFileContainsTokens(t, makefilePath, []string{
		"deploy-helm:",
		"bash scripts/deploy/helm_rollout.sh",
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
