package control

import "testing"

func TestRunSemanticVerifiers(t *testing.T) {
	t.Parallel()

	failures := RunSemanticVerifiers(SemanticVerifierInput{
		InContacts:             false,
		AmbiguousRecipientHits: 2,
		Amount:                 5000,
		HistoricalMedian:       1000,
		MaxSingleTransaction:   2000,
		HasCalendarConflict:    true,
		ConflictAcknowledged:   false,
		WriteActions:           8,
		HasSensitiveData:       true,
		ExternalRecipient:      true,
		IsDeleteAction:         true,
		DeleteConfirmed:        false,
		ClaimCount:             3,
		EvidenceCount:          1,
	})
	if len(failures) == 0 {
		t.Fatal("expected semantic verifier failures")
	}
}

func TestSemanticFailureReasonString(t *testing.T) {
	t.Parallel()

	got := SemanticFailureReasonString(SemanticVerifierFailure{VerifierID: "SV-001", Reason: "recipient_not_in_contacts"})
	if got != "SV-001:recipient_not_in_contacts" {
		t.Fatalf("unexpected semantic failure string: %s", got)
	}
}
