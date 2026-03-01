package executor

import "testing"

func TestAuditAndAutoCommitChainHashesDeterministic(t *testing.T) {
	t.Parallel()

	key := []byte("audit-chain-key")
	auditA := ComputeAuditChainHash(
		key,
		"",
		"entry-1",
		"BREVIO.hands.tool.committed.v1",
		"ws-1",
		"user-1",
		"2026-02-26T00:00:00Z",
		"committed tool action",
	)
	auditB := ComputeAuditChainHash(
		key,
		"",
		"entry-1",
		"BREVIO.hands.tool.committed.v1",
		"ws-1",
		"user-1",
		"2026-02-26T00:00:00Z",
		"committed tool action",
	)
	if auditA != auditB {
		t.Fatalf("expected deterministic audit chain hash")
	}

	proofA := ComputeAutoCommitProofHash(
		[]byte("proof-chain-key"),
		"audit-prev",
		"proof-1",
		"tool-exec-1",
		"A3",
		"consent-1",
		"2026-02-26T00:00:00Z",
	)
	proofB := ComputeAutoCommitProofHash(
		[]byte("proof-chain-key"),
		"audit-prev",
		"proof-1",
		"tool-exec-1",
		"A3",
		"consent-1",
		"2026-02-26T00:00:00Z",
	)
	if proofA != proofB {
		t.Fatalf("expected deterministic auto-commit proof chain hash")
	}
}

func TestVerifyChainHelpers(t *testing.T) {
	t.Parallel()

	key := []byte("chain-key")
	auditEntry := AuditChainEntry{
		PreviousChainHash: "",
		EntryID:           "entry_1",
		Event:             "BREVIO.audit.example.v1",
		WorkspaceID:       "ws_1",
		UserID:            "user_1",
		TimestampRFC3339:  "2026-03-01T12:00:00Z",
		ActionSummary:     "example action",
	}
	auditEntry.ChainHash = ComputeAuditChainHash(
		key,
		auditEntry.PreviousChainHash,
		auditEntry.EntryID,
		auditEntry.Event,
		auditEntry.WorkspaceID,
		auditEntry.UserID,
		auditEntry.TimestampRFC3339,
		auditEntry.ActionSummary,
	)
	if !VerifyAuditChain(key, []AuditChainEntry{auditEntry}) {
		t.Fatal("expected valid audit chain")
	}

	proofEntry := AutoCommitProofEntry{
		PreviousProofHash: "",
		ProofID:           "proof_1",
		ToolExecutionID:   "tool_exec_1",
		EffectiveAutonomy: "A3",
		ConsentID:         "consent_1",
		TimestampRFC3339:  "2026-03-01T12:00:00Z",
	}
	proofEntry.ProofHash = ComputeAutoCommitProofHash(
		key,
		proofEntry.PreviousProofHash,
		proofEntry.ProofID,
		proofEntry.ToolExecutionID,
		proofEntry.EffectiveAutonomy,
		proofEntry.ConsentID,
		proofEntry.TimestampRFC3339,
	)
	if !VerifyAutoCommitProofChain(key, []AutoCommitProofEntry{proofEntry}) {
		t.Fatal("expected valid auto-commit proof chain")
	}
}
