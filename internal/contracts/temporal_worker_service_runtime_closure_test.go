package contracts

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTemporalWorkerServiceRuntimeClosure(t *testing.T) {
	t.Parallel()

	root := repositoryRoot(t)
	workerSource := filepath.Join(root, "deprecated", "brevio-temporal-worker", "src", "index.ts")
	workerReadme := filepath.Join(root, "deprecated", "brevio-temporal-worker", "README.md")

	assertFileContainsTokens(t, workerSource, []string{
		"RECEIVED",
		"CLASSIFYING",
		"DECOMPOSING",
		"EXECUTING",
		"AGGREGATING",
		"FORMATTING",
		"DELIVERING",
		"COMPLETED",
		"FAILED",
		"DEAD_LETTER",
		"INIT",
		"COMPOSING",
		"deterministicJitterMs",
		"message-processing",
		"daily-rhythm",
		"createTemporalWorkerRuntime",
	})

	assertFileContainsTokens(t, workerReadme, []string{
		"Temporal workflow runtime service",
		"POST /api/v1/temporal-worker/workflows/message-processing",
		"POST /api/v1/temporal-worker/workflows/daily-rhythm",
		"DEAD_LETTER",
		"fnv1a",
	})

	body, err := os.ReadFile(workerReadme)
	if err != nil {
		t.Fatalf("read temporal worker readme: %v", err)
	}
	if strings.Contains(strings.ToLower(string(body)), "scaffold directory") {
		t.Fatalf("temporal-worker README still contains scaffold marker")
	}
}
