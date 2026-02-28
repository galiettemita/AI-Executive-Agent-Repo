package contracts

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
)

func TestServiceBuildMatrixClosure(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)

	expectedServices := []string{
		"gateway",
		"brain",
		"control",
		"executor",
		"canvas",
		"temporal-worker",
	}

	assertExactDirectorySet(t, filepath.Join(root, "cmd"), expectedServices)

	makefilePath := filepath.Join(root, "Makefile")
	assertFileContainsTokens(t, makefilePath, []string{
		"for svc in gateway brain control executor canvas temporal-worker; do",
		"docker build --build-arg SERVICE=$$svc -t brevio-$$svc:local .",
	})

	ciPath := filepath.Join(root, ".github", "workflows", "ci.yaml")
	assertFileContainsTokens(t, ciPath, []string{
		"for svc in gateway brain control executor canvas temporal-worker; do",
		"docker build --build-arg SERVICE=\"$svc\" -t \"brevio-${svc}:ci\" .",
		"trivy image --severity CRITICAL,HIGH --exit-code 1 \"brevio-${svc}:ci\"",
	})

	dockerfilePath := filepath.Join(root, "Dockerfile")
	assertFileContainsTokens(t, dockerfilePath, []string{
		"ARG SERVICE=gateway",
		"./cmd/${SERVICE}",
	})
}

func TestServiceBinaryEntryPointClosure(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)
	cmdPath := filepath.Join(root, "cmd")
	entries, err := os.ReadDir(cmdPath)
	if err != nil {
		t.Fatalf("read cmd directory: %v", err)
	}

	actual := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		actual = append(actual, entry.Name())
		assertFileNonEmpty(t, filepath.Join(cmdPath, entry.Name(), "main.go"))
	}
	sort.Strings(actual)
	assertStringSliceSetEquals(t, actual, []string{
		"brain",
		"canvas",
		"control",
		"executor",
		"gateway",
		"temporal-worker",
	}, "cmd_service_set")
}
