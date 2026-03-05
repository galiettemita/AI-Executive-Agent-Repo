package contracts

import (
	"path/filepath"
	"testing"
)

func TestExternalProductionTodoClosure(t *testing.T) {
	t.Parallel()

	root := repositoryRoot(t)

	makefilePath := filepath.Join(root, "Makefile")
	assertFileContainsTokens(t, makefilePath, []string{
		"production-deployment-todo:",
		"generate_production_deployment_todo.sh",
	})

	scriptPath := filepath.Join(root, "scripts", "deploy", "generate_production_deployment_todo.sh")
	assertFileNonEmpty(t, scriptPath)
	assertFileContainsTokens(t, scriptPath, []string{
		"production_deployment_signoff_check.json",
		"production_deployment_todo.md",
		"make ci-full",
		"helm_rollout.sh",
		"Pass Signoff",
	})

	docPath := filepath.Join(root, "docs", "EXTERNAL_CLOSEOUT.md")
	assertFileContainsTokens(t, docPath, []string{
		"make production-deployment-todo",
		"production_deployment_todo.md",
	})
}
