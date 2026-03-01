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
