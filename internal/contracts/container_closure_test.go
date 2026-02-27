package contracts

import (
	"path/filepath"
	"testing"
)

func TestContainerBaselineClosure(t *testing.T) {
	t.Parallel()
	root := repositoryRoot(t)
	dockerfilePath := filepath.Join(root, "Dockerfile")

	assertFileContainsTokens(t, dockerfilePath, []string{
		"FROM golang:1.22 AS build",
		"FROM gcr.io/distroless/static:nonroot",
		"USER 65532:65532",
		"read-only filesystem is enforced by runtime security context",
	})
}
