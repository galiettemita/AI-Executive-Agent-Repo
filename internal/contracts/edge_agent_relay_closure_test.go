package contracts

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEdgeAgentRelayClosure(t *testing.T) {
	t.Parallel()

	root := repositoryRoot(t)
	relaySource := filepath.Join(root, "deprecated", "brevio-edge-relay", "src", "index.ts")
	agentSource := filepath.Join(root, "edge", "brevio-edge-agent", "src", "index.ts")
	relayPackage := filepath.Join(root, "deprecated", "brevio-edge-relay", "package.json")
	agentPackage := filepath.Join(root, "edge", "brevio-edge-agent", "package.json")
	relayReadme := filepath.Join(root, "deprecated", "brevio-edge-relay", "README.md")
	agentReadme := filepath.Join(root, "edge", "brevio-edge-agent", "README.md")

	assertFileContainsTokens(t, relaySource, []string{
		"/ws/edge",
		"/v1/edge/execute",
		"I need your Mac to be online to do that.",
		"type: 'execute_skill'",
		"type: 'skill_result'",
		"EDGE_MAX_QUEUE_AGE_MS",
	})

	assertFileContainsTokens(t, agentSource, []string{
		"EDGE_RELAY_URL",
		"type: 'register'",
		"type: 'heartbeat'",
		"type: 'skill_result'",
		"/health/deep",
		"EDGE_MAX_QUEUE_AGE_MS",
	})

	assertFileContainsTokens(t, relayPackage, []string{
		"\"ws\"",
		"\"@types/ws\"",
	})
	assertFileContainsTokens(t, agentPackage, []string{
		"\"ws\"",
		"\"@types/ws\"",
	})

	for _, path := range []string{relayReadme, agentReadme} {
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read edge readme %s: %v", path, err)
		}
		if strings.Contains(strings.ToLower(string(body)), "scaffold") {
			t.Fatalf("edge readme still contains scaffold marker: %s", path)
		}
	}
}
