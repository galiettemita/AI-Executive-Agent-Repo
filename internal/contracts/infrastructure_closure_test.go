package contracts

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestV9InfrastructureArtifactsExist(t *testing.T) {
	t.Parallel()

	root := repositoryRoot(t)

	requiredTerraformModules := []string{
		"vpc",
		"eks",
		"rds",
		"elasticache",
		"sqs",
		"s3",
		"secrets",
		"temporal",
		"observability",
	}
	for _, module := range requiredTerraformModules {
		assertFileNonEmpty(t, filepath.Join(root, "terraform", "modules", module, "main.tf"))
	}

	requiredModuleBlocks := []string{
		`module "vpc"`,
		`module "eks"`,
		`module "rds"`,
		`module "elasticache"`,
		`module "sqs"`,
		`module "s3"`,
		`module "secrets"`,
		`module "temporal"`,
		`module "observability"`,
		`module "opensearch"`,
		`module "admin_frontend"`,
		`module "feature_flags_cache"`,
	}
	assertFileContainsTokens(t, filepath.Join(root, "terraform", "environments", "staging", "main.tf"), requiredModuleBlocks)
	assertFileContainsTokens(t, filepath.Join(root, "terraform", "environments", "production", "main.tf"), requiredModuleBlocks)

	coreChartsWithHPA := []string{
		"BREVIO-gateway",
		"BREVIO-brain",
		"BREVIO-control",
		"BREVIO-executor",
		"BREVIO-temporal-worker",
	}
	for _, chart := range coreChartsWithHPA {
		assertFileNonEmpty(t, filepath.Join(root, "helm", chart, "Chart.yaml"))
		assertFileNonEmpty(t, filepath.Join(root, "helm", chart, "values.yaml"))
		assertFileNonEmpty(t, filepath.Join(root, "helm", chart, "templates", "deployment.yaml"))
		assertFileNonEmpty(t, filepath.Join(root, "helm", chart, "templates", "service.yaml"))
		assertFileNonEmpty(t, filepath.Join(root, "helm", chart, "templates", "hpa.yaml"))
	}

	canvasChart := filepath.Join(root, "helm", "BREVIO-canvas")
	assertFileNonEmpty(t, filepath.Join(canvasChart, "Chart.yaml"))
	assertFileNonEmpty(t, filepath.Join(canvasChart, "values.yaml"))
	assertFileNonEmpty(t, filepath.Join(canvasChart, "templates", "deployment.yaml"))
	assertFileNonEmpty(t, filepath.Join(canvasChart, "templates", "service.yaml"))

	assertFileNonEmpty(t, filepath.Join(root, "helm", "BREVIO-gateway", "templates", "pdb.yaml"))
}

func TestV92InfrastructureArtifactsExist(t *testing.T) {
	t.Parallel()

	root := repositoryRoot(t)

	requiredTerraformModules := []string{
		"opensearch",
		"admin-frontend",
		"feature-flags-cache",
	}
	for _, module := range requiredTerraformModules {
		path := filepath.Join(root, "terraform", "modules", module, "main.tf")
		assertFileNonEmpty(t, path)
	}

	requiredHelmCharts := []string{
		"BREVIO-admin-api",
		"BREVIO-admin-frontend",
		"BREVIO-rag-worker",
		"BREVIO-guardrails",
		"BREVIO-health-checker",
	}
	for _, chart := range requiredHelmCharts {
		assertFileNonEmpty(t, filepath.Join(root, "helm", chart, "Chart.yaml"))
		assertFileNonEmpty(t, filepath.Join(root, "helm", chart, "values.yaml"))
		assertFileNonEmpty(t, filepath.Join(root, "helm", chart, "templates", "deployment.yaml"))
		assertFileNonEmpty(t, filepath.Join(root, "helm", chart, "templates", "service.yaml"))
	}
}

func TestV92RunbooksExist(t *testing.T) {
	t.Parallel()

	root := repositoryRoot(t)
	for i := 1; i <= 9; i++ {
		path := filepath.Join(root, "runbooks", formatV92RunbookName(i))
		assertFileNonEmpty(t, path)
	}
}

func formatV92RunbookName(index int) string {
	return fmt.Sprintf("RB-V92-%03d.md", index)
}

func assertFileNonEmpty(t *testing.T, path string) {
	t.Helper()

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("required artifact missing %s: %v", path, err)
	}
	if info.Size() == 0 {
		t.Fatalf("required artifact is empty: %s", path)
	}
}

func assertFileContainsTokens(t *testing.T, path string, required []string) {
	t.Helper()

	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file %s: %v", path, err)
	}
	content := string(body)
	for _, token := range required {
		if !strings.Contains(content, token) {
			t.Fatalf("missing token %q in %s", token, path)
		}
	}
}
