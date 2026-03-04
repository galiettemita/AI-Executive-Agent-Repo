package contracts

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDockerComposeLocalStackClosure(t *testing.T) {
	t.Parallel()

	root := repositoryRoot(t)
	composePath := filepath.Join(root, "docker-compose.yml")

	assertFileContainsTokens(t, composePath, []string{
		"postgres:",
		"redis:",
		"temporal:",
		"temporal-ui:",
		"gateway:",
		"brain:",
		"control:",
		"executor:",
		"canvas:",
		"temporal-worker:",
		"x-common-env:",
		"DATABASE_URL:",
		"REDIS_URL:",
		"TEMPORAL_HOST:",
		"GATEWAY_WEBHOOK_SECRET:",
		"profiles: ['openclaw-extension']",
		"brevio-auth:",
		"brevio-profile:",
		"brevio-scheduler:",
		"brevio-metrics:",
		"brevio-edge-relay:",
	})

	body, err := os.ReadFile(composePath)
	if err != nil {
		t.Fatalf("read docker-compose file: %v", err)
	}
	if strings.Contains(strings.ToLower(string(body)), "sleep\", \"infinity") {
		t.Fatalf("docker-compose still contains placeholder sleep infinity command")
	}
}
