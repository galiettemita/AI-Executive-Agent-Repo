package contracts

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInfraHelmBrevioLayoutClosure(t *testing.T) {
	t.Parallel()

	root := repositoryRoot(t)
	chartRoot := filepath.Join(root, "infra", "helm", "brevio")

	requiredFiles := []string{
		"Chart.yaml",
		"values.yaml",
		"values-staging.yaml",
		"values-production.yaml",
		"templates/_helpers.tpl",
		"templates/serviceaccount.yaml",
		"templates/additional-services-deployments.yaml",
		"templates/README.md",
	}
	for _, rel := range requiredFiles {
		assertFileNonEmpty(t, filepath.Join(chartRoot, rel))
	}

	chartPath := filepath.Join(chartRoot, "Chart.yaml")
	assertFileContainsTokens(t, chartPath, []string{
		"name: brevio",
		"dependencies:",
		"name: BREVIO-gateway",
		"name: BREVIO-brain",
		"name: BREVIO-executor",
		"name: BREVIO-temporal-worker",
		"repository: \"file://../../../helm/BREVIO-gateway\"",
	})

	valuesPath := filepath.Join(chartRoot, "values.yaml")
	assertFileContainsTokens(t, valuesPath, []string{
		"gateway:",
		"brain:",
		"hands:",
		"temporalWorker:",
		"additionalServices:",
		"scheduler:",
		"metrics:",
		"edge-relay:",
		"servicePort: 8086",
	})

	templatesPath := filepath.Join(chartRoot, "templates", "additional-services-deployments.yaml")
	assertFileContainsTokens(t, templatesPath, []string{
		"kind: Deployment",
		"kind: Service",
		"kind: HorizontalPodAutoscaler",
		"path: /health",
		"readOnlyRootFilesystem: true",
	})

	chartBody, err := os.ReadFile(chartPath)
	if err != nil {
		t.Fatalf("read chart yaml: %v", err)
	}
	if strings.Contains(strings.ToLower(string(chartBody)), "scaffold") {
		t.Fatalf("infra helm chart still contains scaffold marker: %s", chartPath)
	}
}
