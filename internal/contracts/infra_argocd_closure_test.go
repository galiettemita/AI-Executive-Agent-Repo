package contracts

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInfraArgoCDApplicationsClosure(t *testing.T) {
	t.Parallel()

	root := repositoryRoot(t)
	stagingPath := filepath.Join(root, "infra", "argocd", "application-staging.yaml")
	productionPath := filepath.Join(root, "infra", "argocd", "application-production.yaml")

	assertFileContainsTokens(t, stagingPath, []string{
		"kind: Application",
		"name: brevio-staging",
		"repoURL: https://github.com/galiettemita/AI-Executive-Agent-Repo.git",
		"path: infra/helm/brevio",
		"- values-staging.yaml",
		"namespace: brevio-system",
		"automated:",
		"selfHeal: true",
		"CreateNamespace=true",
	})

	assertFileContainsTokens(t, productionPath, []string{
		"kind: Application",
		"name: brevio-production",
		"repoURL: https://github.com/galiettemita/AI-Executive-Agent-Repo.git",
		"path: infra/helm/brevio",
		"- values-production.yaml",
		"namespace: brevio-system",
		"retry:",
		"limit: 3",
		"CreateNamespace=true",
	})

	for _, path := range []string{stagingPath, productionPath} {
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read argocd manifest %s: %v", path, err)
		}
		if strings.Contains(strings.ToLower(string(body)), "placeholder") {
			t.Fatalf("argocd manifest still contains placeholder marker: %s", path)
		}
	}
}
