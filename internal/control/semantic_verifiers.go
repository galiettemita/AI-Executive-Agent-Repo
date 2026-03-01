package control

import "fmt"

type SemanticVerifierFailure struct {
	VerifierID string
	Reason     string
}

func verifySV001WrongRecipient(inContacts bool, ambiguousMatches int) *SemanticVerifierFailure {
	if !inContacts {
		return &SemanticVerifierFailure{VerifierID: "SV-001", Reason: "recipient_not_in_contacts"}
	}
	if ambiguousMatches > 1 {
		return &SemanticVerifierFailure{VerifierID: "SV-001", Reason: "ambiguous_recipient_match"}
	}
	return nil
}

func verifySV002FinancialAmount(amount, historicalMedian, maxSingleTransaction float64) *SemanticVerifierFailure {
	if historicalMedian > 0 && amount > 2*historicalMedian {
		return &SemanticVerifierFailure{VerifierID: "SV-002", Reason: "amount_over_historical_pattern"}
	}
	if maxSingleTransaction > 0 && amount > maxSingleTransaction {
		return &SemanticVerifierFailure{VerifierID: "SV-002", Reason: "amount_over_workspace_max_single_transaction"}
	}
	return nil
}

func verifySV003CalendarConflict(hasConflict, explicitlyAcknowledged bool) *SemanticVerifierFailure {
	if hasConflict && !explicitlyAcknowledged {
		return &SemanticVerifierFailure{VerifierID: "SV-003", Reason: "calendar_conflict_unacknowledged"}
	}
	return nil
}

func verifySV004BulkAction(writeActions int) *SemanticVerifierFailure {
	if writeActions > 5 {
		return &SemanticVerifierFailure{VerifierID: "SV-004", Reason: "bulk_action_exceeds_limit"}
	}
	return nil
}

func verifySV005SensitiveDataOutbound(hasSensitiveData, externalRecipient bool) *SemanticVerifierFailure {
	if hasSensitiveData && externalRecipient {
		return &SemanticVerifierFailure{VerifierID: "SV-005", Reason: "sensitive_data_outbound"}
	}
	return nil
}

func verifySV006DeleteConfirmation(isDeleteAction, explicitlyConfirmed bool) *SemanticVerifierFailure {
	if isDeleteAction && !explicitlyConfirmed {
		return &SemanticVerifierFailure{VerifierID: "SV-006", Reason: "delete_requires_confirmation"}
	}
	return nil
}

func verifySV007UngroundedClaim(claimCount, evidenceCount int) *SemanticVerifierFailure {
	if claimCount > evidenceCount {
		return &SemanticVerifierFailure{VerifierID: "SV-007", Reason: "claims_exceed_evidence"}
	}
	return nil
}

type SemanticVerifierInput struct {
	InContacts             bool
	AmbiguousRecipientHits int
	Amount                 float64
	HistoricalMedian       float64
	MaxSingleTransaction   float64
	HasCalendarConflict    bool
	ConflictAcknowledged   bool
	WriteActions           int
	HasSensitiveData       bool
	ExternalRecipient      bool
	IsDeleteAction         bool
	DeleteConfirmed        bool
	ClaimCount             int
	EvidenceCount          int
}

func RunSemanticVerifiers(input SemanticVerifierInput) []SemanticVerifierFailure {
	failures := []SemanticVerifierFailure{}
	checks := []*SemanticVerifierFailure{
		verifySV001WrongRecipient(input.InContacts, input.AmbiguousRecipientHits),
		verifySV002FinancialAmount(input.Amount, input.HistoricalMedian, input.MaxSingleTransaction),
		verifySV003CalendarConflict(input.HasCalendarConflict, input.ConflictAcknowledged),
		verifySV004BulkAction(input.WriteActions),
		verifySV005SensitiveDataOutbound(input.HasSensitiveData, input.ExternalRecipient),
		verifySV006DeleteConfirmation(input.IsDeleteAction, input.DeleteConfirmed),
		verifySV007UngroundedClaim(input.ClaimCount, input.EvidenceCount),
	}
	for _, check := range checks {
		if check != nil {
			failures = append(failures, *check)
		}
	}
	return failures
}

func SemanticFailureReasonString(failure SemanticVerifierFailure) string {
	return fmt.Sprintf("%s:%s", failure.VerifierID, failure.Reason)
}
