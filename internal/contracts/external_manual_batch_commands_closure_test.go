package contracts

import (
	"path/filepath"
	"testing"
)

func TestExternalManualBatchCommandsClosure(t *testing.T) {
	t.Parallel()

	root := repositoryRoot(t)

	makefilePath := filepath.Join(root, "Makefile")
	assertFileContainsTokens(t, makefilePath, []string{
		"manual-closeout-batch-commands:",
		"generate_manual_closeout_batch_commands.sh",
	})

	scriptPath := filepath.Join(root, "scripts", "deploy", "generate_manual_closeout_batch_commands.sh")
	assertFileNonEmpty(t, scriptPath)
	assertFileContainsTokens(t, scriptPath, []string{
		"go_live_signoff_status.json",
		"manual_closeout_batch_commands.sh",
		"make manual-closeout-confirm ITEM_ID=",
		"make external-phase-sync",
		"usage: $0 <actor-name>",
	})

	docPath := filepath.Join(root, "docs", "EXTERNAL_CLOSEOUT.md")
	assertFileContainsTokens(t, docPath, []string{
		"make manual-closeout-batch-commands",
		"manual_closeout_batch_commands.sh",
		"./artifacts/deploy/manual_closeout_batch_commands.sh ops",
	})
}
