package contracts

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInfraDockerLayoutClosure(t *testing.T) {
	t.Parallel()

	root := repositoryRoot(t)
	dockerRoot := filepath.Join(root, "infra", "docker")

	requiredDockerfiles := []string{
		"Dockerfile.gateway",
		"Dockerfile.brain",
		"Dockerfile.hands",
		"Dockerfile.control",
		"Dockerfile.executor",
		"Dockerfile.canvas",
		"Dockerfile.temporal-worker",
	}

	for _, name := range requiredDockerfiles {
		path := filepath.Join(dockerRoot, name)
		assertFileContainsTokens(t, path, []string{
			"FROM golang:1.22 AS build",
			"go mod download",
			"go build -trimpath",
			"FROM gcr.io/distroless/static:nonroot",
			"USER 65532:65532",
			"ENTRYPOINT [\"/app/service\"]",
		})

		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read dockerfile %s: %v", path, err)
		}
		if strings.Contains(strings.ToLower(string(body)), "scaffold") {
			t.Fatalf("dockerfile still contains scaffold marker: %s", path)
		}
	}

	assertFileContainsTokens(t, filepath.Join(root, "Makefile"), []string{
		"docker-build:",
		"docker-build-infra:",
		"for svc in gateway brain control executor canvas temporal-worker",
		"for svc in gateway brain hands control executor canvas temporal-worker",
		"docker build -f infra/docker/Dockerfile.$$svc -t brevio-$$svc:local .",
	})

	assertFileContainsTokens(t, filepath.Join(dockerRoot, "README.md"), []string{
		"Brevio Service Dockerfiles",
		"Dockerfile.hands",
		"cmd/executor",
		"distroless non-root runtime image",
	})
}
