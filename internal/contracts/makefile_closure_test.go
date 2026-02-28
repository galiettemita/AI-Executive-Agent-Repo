package contracts

import (
	"path/filepath"
	"testing"
)

func TestMakefileDockerBuildClosure(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)
	makefilePath := filepath.Join(root, "Makefile")
	assertFileContainsTokens(t, makefilePath, []string{
		"docker-build:",
		"for svc in gateway brain control executor canvas temporal-worker",
		"docker build --build-arg SERVICE=$$svc -t brevio-$$svc:local .",
	})
}

func TestMakefileGoFallbackClosure(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)
	makefilePath := filepath.Join(root, "Makefile")
	assertFileContainsTokens(t, makefilePath, []string{
		"GO_EXEC := ./scripts/dev/go_exec.sh",
		"$(GO_EXEC) build ./...",
		"$(GO_EXEC) test ./... -count=1",
		"$(GO_EXEC) tool gofmt -l .",
		"$(GO_EXEC) vet ./...",
		"$(GO_EXEC) run honnef.co/go/tools/cmd/staticcheck@v0.5.1 ./...",
		"$(GO_EXEC) test ./internal/contracts -count=1",
	})
	assertFileNonEmpty(t, filepath.Join(root, "scripts", "dev", "go_exec.sh"))
}
