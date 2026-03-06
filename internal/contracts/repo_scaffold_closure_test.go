package contracts

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPhase1RepoScaffoldClosure(t *testing.T) {
	t.Parallel()

	root := repositoryRoot(t)

	requiredDirs := []string{
		"cmd/gateway",
		"cmd/brain",
		"cmd/control",
		"cmd/executor",
		"cmd/canvas",
		"cmd/temporal-worker",
		"internal/determinism",
		"internal/contracts",
		"internal/integration",
		"internal/security",
		"internal/provisioning",
		"internal/onboarding",
		"db/migrations",
		"api/openapi",
		"schemas",
		"prompts",
		"policies",
		"terraform/modules",
		"helm",
		"evals",
	}
	for _, dir := range requiredDirs {
		info, err := os.Stat(filepath.Join(root, dir))
		if err != nil {
			t.Fatalf("required scaffold directory missing %s: %v", dir, err)
		}
		if !info.IsDir() {
			t.Fatalf("required scaffold path is not a directory: %s", dir)
		}
	}

	for _, mainPath := range []string{
		"cmd/gateway/main.go",
		"cmd/brain/main.go",
		"cmd/control/main.go",
		"cmd/executor/main.go",
		"cmd/canvas/main.go",
		"cmd/temporal-worker/main.go",
	} {
		assertFileNonEmpty(t, filepath.Join(root, mainPath))
	}

	assertFileContainsTokens(t, filepath.Join(root, ".golangci.yml"), []string{
		"gofmt",
		"govet",
		"staticcheck",
		"errcheck",
	})

	assertFileContainsTokens(t, filepath.Join(root, "Makefile"), []string{
		"build:",
		"test:",
		"lint:",
		"migrate:",
		"docker-build:",
	})

	assertFileContainsTokens(t, filepath.Join(root, "Dockerfile"), []string{
		"FROM golang:1.23 AS build",
		"FROM gcr.io/distroless/static:nonroot",
		"USER 65532:65532",
		"ENTRYPOINT [\"/app/service\"]",
	})
}
