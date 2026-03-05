package contracts

import (
	"path/filepath"
	"testing"
)

func TestFinalGoLiveApprovalPacketClosure(t *testing.T) {
	t.Parallel()

	root := repositoryRoot(t)

	makefilePath := filepath.Join(root, "Makefile")
	assertFileContainsTokens(t, makefilePath, []string{
		"go-live-approval-packet:",
		"generate_final_go_live_approval_packet.sh",
		"go-live-approval-confirm:",
		"confirm_final_go_live_approval.sh",
	})

	scriptPath := filepath.Join(root, "scripts", "deploy", "generate_final_go_live_approval_packet.sh")
	assertFileNonEmpty(t, scriptPath)
	assertFileContainsTokens(t, scriptPath, []string{
		"phase_closure_manifest.json",
		"phase_handoff_bundle.json",
		"phase_status.txt",
		"final_go_live_approval_packet.json",
		"final_go_live_approval_packet.md",
		"technical_ready_for_approval",
		"human_approvals_complete",
		"ready_for_go_live_execution",
	})

	confirmScriptPath := filepath.Join(root, "scripts", "deploy", "confirm_final_go_live_approval.sh")
	assertFileNonEmpty(t, confirmScriptPath)
	assertFileContainsTokens(t, confirmScriptPath, []string{
		"approvals_required",
		"APPROVED",
		"generate_final_go_live_approval_packet.sh",
	})

	docPath := filepath.Join(root, "docs", "EXTERNAL_CLOSEOUT.md")
	assertFileContainsTokens(t, docPath, []string{
		"make go-live-approval-packet",
		"make go-live-approval-confirm",
		"final_go_live_approval_packet.json",
		"final_go_live_approval_packet.md",
	})

	ciWorkflowPath := filepath.Join(root, ".github", "workflows", "ci.yml")
	assertFileContainsTokens(t, ciWorkflowPath, []string{
		"Generate final go-live approval packet",
		"generate_final_go_live_approval_packet.sh",
		"final_go_live_approval_packet.json",
		"final_go_live_approval_packet.md",
	})

	prodWorkflowPath := filepath.Join(root, ".github", "workflows", "deploy-production.yml")
	assertFileContainsTokens(t, prodWorkflowPath, []string{
		"Generate final go-live approval packet",
		"generate_final_go_live_approval_packet.sh",
		"final_go_live_approval_packet.json",
		"final_go_live_approval_packet.md",
	})
}
