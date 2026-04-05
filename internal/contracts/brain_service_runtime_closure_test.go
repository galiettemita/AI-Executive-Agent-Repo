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
	brainSource := filepath.Join(root, "services", "brevio-brain", "src", "index.ts")
	classifySource := filepath.Join(root, "services", "brevio-brain", "src", "classify.ts")
	disambiguateSource := filepath.Join(root, "services", "brevio-brain", "src", "disambiguate.ts")
	decomposeSource := filepath.Join(root, "services", "brevio-brain", "src", "decompose.ts")
	plannerSource := filepath.Join(root, "services", "brevio-brain", "src", "planner.ts")
	verifySource := filepath.Join(root, "services", "brevio-brain", "src", "verify.ts")
	protoSource := filepath.Join(root, "packages", "proto", "brevio", "brain", "v1", "brain.proto")
	brainReadme := filepath.Join(root, "services", "brevio-brain", "README.md")

	assertFileContainsTokens(t, brainSource, []string{
		"/api/v1/brain/classify",
		"/api/v1/brain/disambiguate",
		"/api/v1/brain/decompose",
		"/api/v1/brain/aggregate",
		"/api/v1/brain/process",
		"loadDisambiguationRules",
		"brain.process.complete",
		"TASK_GRAPH_INVALID",
		"invalid_json",
		"dispatch_ready",
	})

	assertFileContainsTokens(t, classifySource, []string{
		"confidence",
		"requires_decomposition",
		"general.assistance",
		"blocked_skills",
		"clarification_required",
	})

	assertFileContainsTokens(t, disambiguateSource, []string{
		"resolveFromRule",
		"group_hits",
		"blocked_skills",
		"clarification_required",
	})

	assertFileContainsTokens(t, decomposeSource, []string{
		"max 10 tasks",
		"execution_order",
		"request_required",
	})

	assertFileContainsTokens(t, plannerSource, []string{
		"planner_provider",
		"requires_clarification",
		"OPENAI_API_KEY",
	})

	assertFileContainsTokens(t, verifySource, []string{
		"process_response_is_dispatch_only_until_real_skill_results_arrive",
		"missing_tool_for_",
	})

	assertFileContainsTokens(t, protoSource, []string{
		"rpc DisambiguateSkills",
		"rpc ProcessTurn",
	})

	assertFileContainsTokens(t, brainReadme, []string{
		"POST /api/v1/brain/classify",
		"POST /api/v1/brain/disambiguate",
		"POST /api/v1/brain/decompose",
		"POST /api/v1/brain/process",
		"11 disambiguation groups",
		"planner/verifier",
	})

	body, err := os.ReadFile(brainReadme)
	if err != nil {
		t.Fatalf("read brain readme: %v", err)
	}
	if strings.Contains(strings.ToLower(string(body)), "scaffold directory") {
		t.Fatalf("brain README still contains scaffold marker")
	}

	if _, err := os.Stat(filepath.Join(root, "internal", "brain", "disambiguation")); err == nil {
		t.Fatalf("duplicate go disambiguation router still present; brain routing must have a single runtime implementation")
	}
}
