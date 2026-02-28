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
		"GOFMT_EXEC := ./scripts/dev/gofmt_exec.sh",
		"$(GO_EXEC) build ./...",
		"$(GO_EXEC) test ./... -count=1",
		"$(GOFMT_EXEC) -l .",
		"$(GO_EXEC) vet ./...",
		"$(GO_EXEC) run honnef.co/go/tools/cmd/staticcheck@v0.5.1 ./...",
		"db-verify:",
		"bash scripts/database/verify_postgres_migrations.sh",
		"api-docs-check:",
		"git diff --exit-code docs/API_REFERENCE.md",
		"$(GO_EXEC) test ./internal/contracts -count=1",
		"load-test:",
		"k6 run evals/load/k6_interactive_turn.js",
		"k6 run evals/load/k6_load_shedding.js",
		"k6 run evals/load/k6_streaming_first_byte.js",
	})
	assertFileNonEmpty(t, filepath.Join(root, "scripts", "dev", "go_exec.sh"))
	assertFileNonEmpty(t, filepath.Join(root, "scripts", "dev", "gofmt_exec.sh"))
	assertFileNonEmpty(t, filepath.Join(root, "scripts", "database", "verify_postgres_migrations.sh"))
}
