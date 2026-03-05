package contracts

import (
	"path/filepath"
	"testing"
)

func TestExternalManualProviderStepsClosure(t *testing.T) {
	t.Parallel()

	root := repositoryRoot(t)

	makefilePath := filepath.Join(root, "Makefile")
	assertFileContainsTokens(t, makefilePath, []string{
		"manual-provider-steps:",
		"generate_manual_provider_steps.sh",
	})

	scriptPath := filepath.Join(root, "scripts", "deploy", "generate_manual_provider_steps.sh")
	assertFileNonEmpty(t, scriptPath)
	assertFileContainsTokens(t, scriptPath, []string{
		"go_live_signoff_status.json",
		"manual_provider_steps.md",
		"make manual-closeout-confirm ITEM_ID=",
		"make phase-status",
		"manual_required_items",
	})

	docPath := filepath.Join(root, "docs", "EXTERNAL_CLOSEOUT.md")
	assertFileContainsTokens(t, docPath, []string{
		"make manual-provider-steps",
		"manual_provider_steps.md",
	})
}
