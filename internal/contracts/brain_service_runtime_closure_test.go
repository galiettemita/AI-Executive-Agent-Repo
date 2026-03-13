package contracts

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBrainServiceRuntimeClosure(t *testing.T) {
	t.Parallel()

	root := repositoryRoot(t)
	brainSource := filepath.Join(root, "deprecated", "brevio-brain", "src", "index.ts")
	classifySource := filepath.Join(root, "deprecated", "brevio-brain", "src", "classify.ts")
	disambiguateSource := filepath.Join(root, "deprecated", "brevio-brain", "src", "disambiguate.ts")
	decomposeSource := filepath.Join(root, "deprecated", "brevio-brain", "src", "decompose.ts")
	brainReadme := filepath.Join(root, "deprecated", "brevio-brain", "README.md")

	assertFileContainsTokens(t, brainSource, []string{
		"/api/v1/brain/classify",
		"/api/v1/brain/disambiguate",
		"/api/v1/brain/decompose",
		"/api/v1/brain/aggregate",
		"/api/v1/brain/process",
		"loadDisambiguationRules",
		"brain.process.complete",
		"TASK_GRAPH_INVALID",
	})

	assertFileContainsTokens(t, classifySource, []string{
		"confidence",
		"requires_decomposition",
		"general.assistance",
	})

	assertFileContainsTokens(t, disambiguateSource, []string{
		"spotify-web-api",
		"apple-notes-skill",
		"smart-expense-tracker",
		"group_hits",
	})

	assertFileContainsTokens(t, decomposeSource, []string{
		"max 10 tasks",
		"cycle detected",
		"execution_order",
	})

	assertFileContainsTokens(t, brainReadme, []string{
		"POST /api/v1/brain/classify",
		"POST /api/v1/brain/disambiguate",
		"POST /api/v1/brain/decompose",
		"POST /api/v1/brain/process",
		"11 disambiguation groups",
	})

	body, err := os.ReadFile(brainReadme)
	if err != nil {
		t.Fatalf("read brain readme: %v", err)
	}
	if strings.Contains(strings.ToLower(string(body)), "scaffold directory") {
		t.Fatalf("brain README still contains scaffold marker")
	}
}
