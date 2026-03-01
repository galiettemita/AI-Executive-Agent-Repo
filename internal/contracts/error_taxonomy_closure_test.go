package contracts

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAppendixBErrorTaxonomyClosure(t *testing.T) {
	t.Parallel()

	root := repositoryRoot(t)
	path := filepath.Join(root, "spec", "errors", "appendix_b_error_taxonomy.csv")
	assertFileNonEmpty(t, path)

	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("open taxonomy csv: %v", err)
	}
	defer func() {
		_ = file.Close()
	}()

	reader := csv.NewReader(file)
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatalf("read taxonomy csv: %v", err)
	}
	if len(rows) < 2 {
		t.Fatalf("taxonomy csv must contain header + rows")
	}

	header := strings.Join(rows[0], ",")
	if header != "error_code,category,severity,retryable,user_message" {
		t.Fatalf("unexpected taxonomy header: %q", header)
	}

	codes := map[string]struct{}{}
	for _, row := range rows[1:] {
		if len(row) != 5 {
			t.Fatalf("taxonomy row should have 5 columns, got %d: %v", len(row), row)
		}
		code := strings.TrimSpace(row[0])
		if code == "" {
			t.Fatalf("taxonomy row has empty error_code: %v", row)
		}
		codes[code] = struct{}{}
	}

	required := []string{
		"BUDGET_CALLS_EXHAUSTED",
		"CONTEXT_BUDGET_EXCEEDED",
		"EVENT_SCHEMA_INVALID",
		"FEATURE_DISABLED",
		"GUARDRAIL_BLOCK_ACTIVE",
		"MODEL_TIER_EXCEEDED",
		"PII_ENCRYPTION_REQUIRED",
		"SANDBOX_VIOLATION",
		"SELF_MODIFICATION_DENIED",
		"TOOL_QUARANTINED",
	}
	for _, code := range required {
		if _, ok := codes[code]; !ok {
			t.Fatalf("taxonomy csv missing required code: %s", code)
		}
	}
}
