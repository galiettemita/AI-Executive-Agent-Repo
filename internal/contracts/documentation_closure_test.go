package contracts

import (
	"path/filepath"
	"testing"
)

func TestDocumentationClosure(t *testing.T) {
	t.Parallel()

	root := repositoryRoot(t)

	requiredDocs := map[string][]string{
		filepath.Join(root, "README.md"): {
			"# BREVIO Monorepo",
			"## Services",
			"## Quick Start",
			"## Key Directories",
			"api/openapi/v9.yaml",
			"db/migrations/001_BREVIO_v9_init.sql",
		},
		filepath.Join(root, "docs", "ARCHITECTURE.md"): {
			"# ARCHITECTURE",
			"## Runtime Planes",
			"## Data and Workflows",
			"## Infrastructure",
			"internal/workflows/",
		},
		filepath.Join(root, "docs", "DEVELOPMENT.md"): {
			"# DEVELOPMENT",
			"## Prerequisites",
			"## Local Validation",
			"## Project Conventions",
			"go test ./... -count=1",
		},
		filepath.Join(root, "docs", "DEPLOYMENT.md"): {
			"# DEPLOYMENT",
			"## Infrastructure",
			"## Workload Deployment",
			"## Canonical Sequence",
			"terraform plan",
			"terraform apply",
			"helm lint",
			"helm install",
		},
	}

	for path, tokens := range requiredDocs {
		assertFileNonEmpty(t, path)
		assertFileContainsTokens(t, path, tokens)
	}
}
