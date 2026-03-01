package contracts

import (
	"path/filepath"
	"testing"
)

func TestDRPostureClosure(t *testing.T) {
	t.Parallel()

	root := repositoryRoot(t)
	rdsPath := filepath.Join(root, "terraform", "modules", "rds", "main.tf")
	assertFileNonEmpty(t, rdsPath)
	assertFileContainsTokens(t, rdsPath, []string{
		"multi_az          = true",
		"storage_encrypted = true",
		"pitr_enabled   = true",
		"retention_days = 14",
	})

	runbookPath := filepath.Join(root, "runbooks", "RB-009.md")
	assertFileNonEmpty(t, runbookPath)
	assertFileContainsTokens(t, runbookPath, []string{
		"Scheduled monthly restore drill",
		"latest snapshot/PITR point",
		"Record actual RTO/RPO values",
		"workspace isolation checks pass post-restore",
	})
}
