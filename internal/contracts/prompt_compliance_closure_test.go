package contracts

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPromptSeedClosure(t *testing.T) {
	t.Parallel()

	root := repositoryRoot(t)

	assertPromptSet(t, filepath.Join(root, "prompts", "seed_prompts_v9.txt"), []string{
		"brain.system.v9",
		"brain.planner.v9",
		"brain.critic.v9",
		"brain.reflector.v9",
		"brain.capability_extractor.v1",
		"brain.provisioning_approval_message.v1",
		"brain.provisioning_status_message.v1",
		"brain.provisioning_security_justification.v1",
		"brain.provisioning_rank_explainer.v1",
		"brain.discovery.profile_extractor.v1",
		"brain.discovery.behavior_extractor.v1",
		"brain.discovery.codebase_extractor.v1",
		"brain.discovery.systemmap_extractor.v1",
	})

	assertPromptSet(t, filepath.Join(root, "prompts", "seed_prompts_v91.txt"), []string{
		"brain.daily_capture.v1",
		"brain.goal_review.v1",
		"brain.trust_eval.v1",
		"brain.lesson_extractor.v1",
		"brain.capability_explorer.v1",
		"brain.morning_briefing.v1",
		"brain.discovery.followup_generator.v1",
		"brain.code_context.assembler.v1",
	})

	assertPromptSet(t, filepath.Join(root, "prompts", "seed_prompts_v92.txt"), []string{
		"brain.context_compiler.v1",
		"brain.session_resolver.v1",
		"brain.temporal_resolver.v1",
		"brain.error_communicator.v1",
		"brain.rag_query_rewriter.v1",
		"brain.admin_report_synthesizer.v1",
	})
}

func TestComplianceMatrixClosure(t *testing.T) {
	t.Parallel()

	root := repositoryRoot(t)
	assertCSVRowCountAtLeast(t, filepath.Join(root, "spec", "traceability", "compliance_matrix_v9.csv"), 2)
	assertCSVRowCountAtLeast(t, filepath.Join(root, "spec", "traceability", "compliance_matrix_v91.csv"), 12)
	assertCSVRowCountAtLeast(t, filepath.Join(root, "spec", "traceability", "compliance_matrix_v92.csv"), 19)
}

func assertPromptSet(t *testing.T, path string, required []string) {
	t.Helper()

	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read prompt seed %s: %v", path, err)
	}

	lines := strings.Split(string(body), "\n")
	seen := make(map[string]struct{}, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		seen[trimmed] = struct{}{}
	}

	for _, promptID := range required {
		if _, ok := seen[promptID]; !ok {
			t.Fatalf("missing prompt %q in %s", promptID, path)
		}
	}
}

func assertCSVRowCountAtLeast(t *testing.T, path string, minRows int) {
	t.Helper()

	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("open csv %s: %v", path, err)
	}
	defer file.Close()

	rows, err := csv.NewReader(file).ReadAll()
	if err != nil {
		t.Fatalf("parse csv %s: %v", path, err)
	}
	if len(rows) < minRows {
		t.Fatalf("csv %s has too few rows: got=%d want_at_least=%d", path, len(rows), minRows)
	}
}
