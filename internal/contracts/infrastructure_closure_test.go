package contracts

import (
	"os"
	"path/filepath"
	"testing"
)

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
	if index < 10 {
		return "RB-V92-00" + string(rune('0'+index)) + ".md"
	}
	return "RB-V92-0" + string(rune('0'+index)) + ".md"
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
