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
		"agents",
		"brain",
		"brevioctl",
		"browser",
		"canvas",
		"control",
		"cron",
		"executor",
		"gateway",
		"marketing",
		"memory",
		"router",
		"temporal-worker",
	}

	assertExactDirectorySet(t, filepath.Join(root, "cmd"), expectedServices)

	makefilePath := filepath.Join(root, "Makefile")
	assertFileContainsTokens(t, makefilePath, []string{
		"docker build",
	})

	ciPath := filepath.Join(root, ".github", "workflows", "ci.yaml")
	assertFileContainsTokens(t, ciPath, []string{
		"docker build",
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
		"agents",
		"brain",
		"brevioctl",
		"browser",
		"canvas",
		"control",
		"cron",
		"executor",
		"gateway",
		"marketing",
		"memory",
		"router",
		"temporal-worker",
	}, "cmd_service_set")
}
