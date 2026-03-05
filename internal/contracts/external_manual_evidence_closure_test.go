package contracts

import (
	"path/filepath"
	"testing"
)

func TestExternalManualEvidenceClosure(t *testing.T) {
	t.Parallel()

	root := repositoryRoot(t)

	makefilePath := filepath.Join(root, "Makefile")
	assertFileContainsTokens(t, makefilePath, []string{
		"manual-closeout-confirm:",
		"update_manual_closeout_evidence.sh",
		"external-phase-sync:",
		"sync_external_phase_artifacts.sh",
	})

	checkerPath := filepath.Join(root, "scripts", "deploy", "external_closeout_check.sh")
	assertFileNonEmpty(t, checkerPath)
	assertFileContainsTokens(t, checkerPath, []string{
		"MANUAL_EVIDENCE_PATH",
		"manual_confirmation_detail",
		"manual_confirmation_or_empty",
		"manual_evidence_path",
		"manual_evidence_confirmed",
	})

	evidenceUpdaterPath := filepath.Join(root, "scripts", "deploy", "update_manual_closeout_evidence.sh")
	assertFileNonEmpty(t, evidenceUpdaterPath)
	assertFileContainsTokens(t, evidenceUpdaterPath, []string{
		"manual_closeout_evidence.json",
		"external-closeout-required-item-ids.txt",
		"unsupported item_id",
		"confirmed_by",
		"confirmed_at_utc",
		"item_id",
	})

	catalogPath := filepath.Join(root, "config", "external-closeout-required-item-ids.txt")
	assertFileNonEmpty(t, catalogPath)
	assertFileContainsTokens(t, catalogPath, []string{
		"partner_applications_submitted",
		"plaid_secret_prod",
		"plaid_webhook_secret",
		"stripe_billing_keys",
		"unstructured_api_key",
		"pagerduty_routing_key",
		"analytics_event_bus",
		"remote_catalog_signing_keys",
	})

	docPath := filepath.Join(root, "docs", "EXTERNAL_CLOSEOUT.md")
	assertFileContainsTokens(t, docPath, []string{
		"make manual-closeout-confirm",
		"artifacts/deploy/manual_closeout_evidence.json",
		"ITEM_ID=partner_applications_submitted",
		"config/external-closeout-required-item-ids.txt",
	})
}
