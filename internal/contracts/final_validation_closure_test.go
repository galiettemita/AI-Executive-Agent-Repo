package contracts

import (
	"path/filepath"
	"regexp"
	"testing"
)

func TestFinalValidationReportClosure(t *testing.T) {
	t.Parallel()

	root := repositoryRoot(t)
	reportPath := filepath.Join(root, "docs", "FINAL_VALIDATION_v9.2.0-final.md")
	assertFileNonEmpty(t, reportPath)
	assertFileContainsTokens(t, reportPath, []string{
		"# BREVIO Final Validation Report (v9.2.0-final)",
		"Timestamp (UTC):",
		"## Validation Commands",
		"`make ci`",
		"`make security-validate`",
		"`make infra-validate`",
		"`make db-verify`",
		"## Results",
		"- `make ci`: PASS",
		"- `make security-validate`: PASS",
		"- `make infra-validate`: PASS",
		"- `make db-verify`: PASS",
		"## Blueprint Source Tracking",
		"Brevio_V9_Consolidated_Master_Blueprint.docx",
		"Brevio_V91_Addendum_Soft_Intelligence_Layer.docx",
		"Brevio_V92_Addendum_Production_Hardening.docx",
	})

	content := readFileString(t, reportPath)
	pattern := regexp.MustCompile(`Timestamp \(UTC\): \d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2} UTC`)
	if !pattern.MatchString(content) {
		t.Fatalf("final validation report timestamp does not match expected format: %s", reportPath)
	}
}
