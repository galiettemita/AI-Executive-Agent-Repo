package contracts

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"sort"
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
	assertCSVRowCountExact(t, filepath.Join(root, "spec", "traceability", "compliance_matrix_v9.csv"), 21)
	assertCSVRowCountExact(t, filepath.Join(root, "spec", "traceability", "compliance_matrix_v91.csv"), 20)
	assertCSVRowCountExact(t, filepath.Join(root, "spec", "traceability", "compliance_matrix_v92.csv"), 31)

	assertMatrixCoverage(
		t,
		filepath.Join(root, "spec", "traceability", "compliance_matrix_v9.csv"),
		"requirement_id",
		"status",
		[]string{
			"V9-001", "V9-002", "V9-003", "V9-004", "V9-005",
			"V9-006", "V9-007", "V9-008", "V9-009", "V9-010",
			"V9-011", "V9-012", "V9-013", "V9-014", "V9-015",
			"V9-016", "V9-017", "V9-018", "V9-019", "V9-020",
		},
	)
	assertMatrixCoverage(
		t,
		filepath.Join(root, "spec", "traceability", "compliance_matrix_v91.csv"),
		"requirement_id",
		"status",
		[]string{
			"V91-GATE-001", "V91-GATE-002", "V91-GATE-003", "V91-GATE-004", "V91-GATE-005",
			"V91-GATE-006", "V91-GATE-007", "V91-GATE-008", "V91-GATE-009", "V91-GATE-010",
			"V91-GATE-011",
			"NNR-V91-001", "NNR-V91-002", "NNR-V91-003", "NNR-V91-004",
			"NNR-V91-005", "NNR-V91-006", "NNR-V91-007", "NNR-V91-008",
		},
	)
	assertMatrixCoverage(
		t,
		filepath.Join(root, "spec", "traceability", "compliance_matrix_v92.csv"),
		"requirement_id",
		"status",
		[]string{
			"V92-GATE-001", "V92-GATE-002", "V92-GATE-003", "V92-GATE-004", "V92-GATE-005", "V92-GATE-006",
			"V92-GATE-007", "V92-GATE-008", "V92-GATE-009", "V92-GATE-010", "V92-GATE-011", "V92-GATE-012",
			"V92-GATE-013", "V92-GATE-014", "V92-GATE-015", "V92-GATE-016", "V92-GATE-017", "V92-GATE-018",
			"NNR-V92-001", "NNR-V92-002", "NNR-V92-003", "NNR-V92-004",
			"NNR-V92-005", "NNR-V92-006", "NNR-V92-007", "NNR-V92-008",
			"NNR-V92-009", "NNR-V92-010", "NNR-V92-011", "NNR-V92-012",
		},
	)
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

func assertCSVRowCountExact(t *testing.T, path string, expectedRows int) {
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
	if len(rows) != expectedRows {
		t.Fatalf("csv %s row count mismatch: got=%d want=%d", path, len(rows), expectedRows)
	}
}

func assertMatrixCoverage(t *testing.T, path, idColumn, statusColumn string, requiredIDs []string) {
	t.Helper()
	rows := readCSVRowsWithHeader(t, path)
	if len(rows) == 0 {
		t.Fatalf("csv %s has no data rows", path)
	}

	ids := map[string]struct{}{}
	for _, row := range rows {
		id := strings.TrimSpace(row[idColumn])
		status := strings.TrimSpace(strings.ToLower(row[statusColumn]))
		if id == "" {
			t.Fatalf("csv %s has empty id in row: %v", path, row)
		}
		if _, exists := ids[id]; exists {
			t.Fatalf("csv %s has duplicate id %s", path, id)
		}
		if status != "implemented" {
			t.Fatalf("csv %s has non-implemented status for %s: %s", path, id, status)
		}
		ids[id] = struct{}{}
	}

	for _, requiredID := range requiredIDs {
		if _, ok := ids[requiredID]; !ok {
			t.Fatalf("csv %s missing required id %s", path, requiredID)
		}
	}

	requiredSet := make(map[string]struct{}, len(requiredIDs))
	for _, requiredID := range requiredIDs {
		requiredSet[requiredID] = struct{}{}
	}
	extra := make([]string, 0)
	for id := range ids {
		if _, ok := requiredSet[id]; !ok {
			extra = append(extra, id)
		}
	}
	if len(extra) != 0 {
		sort.Strings(extra)
		t.Fatalf("csv %s has unexpected requirement ids: %v", path, extra)
	}
}

func readCSVRowsWithHeader(t *testing.T, path string) []map[string]string {
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
	if len(rows) < 2 {
		return nil
	}

	header := rows[0]
	out := make([]map[string]string, 0, len(rows)-1)
	for _, row := range rows[1:] {
		record := map[string]string{}
		for i, cell := range row {
			if i >= len(header) {
				continue
			}
			record[header[i]] = cell
		}
		out = append(out, record)
	}
	return out
}
