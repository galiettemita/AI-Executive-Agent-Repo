package contracts

import (
	"path/filepath"
	"testing"
)

func TestExternalCloseoutAutomationClosure(t *testing.T) {
	t.Parallel()

	root := repositoryRoot(t)

	scriptPath := filepath.Join(root, "scripts", "deploy", "external_closeout_check.sh")
	assertFileNonEmpty(t, scriptPath)
	assertFileContainsTokens(t, scriptPath, []string{
		"PLAID_SECRET_PROD",
		"PLAID_WEBHOOK_SECRET",
		"STRIPE_SECRET_KEY",
		"STRIPE_WEBHOOK_SECRET",
		"UNSTRUCTURED_API_KEY",
		"PAGERDUTY_ROUTING_KEY",
		"ANALYTICS_EVENT_BUS",
		"REMOTE_CATALOG_PRIVATE_KEY",
		"REMOTE_CATALOG_PUBLIC_KEY",
		"external_closeout_status.json",
	})

	assertFileContainsTokens(t, filepath.Join(root, "Makefile"), []string{
		"external-closeout-check:",
		"bash scripts/deploy/external_closeout_check.sh",
	})

	docPath := filepath.Join(root, "docs", "EXTERNAL_CLOSEOUT.md")
	assertFileNonEmpty(t, docPath)
	assertFileContainsTokens(t, docPath, []string{
		"make external-closeout-check",
		"PARTNER_APPS_CONFIRMED",
		"ANALYTICS_EVENT_BUS",
		"artifacts/deploy/external_closeout_status.json",
	})
}
