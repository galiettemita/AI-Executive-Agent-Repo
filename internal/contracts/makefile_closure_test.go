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
