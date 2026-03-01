package compliance

import (
	"testing"
	"time"
)

func TestEvaluateRetentionExpiry(t *testing.T) {
	t.Parallel()

	createdAt := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	evaluation := EvaluateRetentionExpiry("RP-005", createdAt, time.Date(2024, 2, 15, 0, 0, 0, 0, time.UTC))
	if !evaluation.Expired || evaluation.PolicyID != "RP-005" || evaluation.EventName != "BREVIO.retention.expired.v1" {
		t.Fatalf("unexpected retention evaluation: %+v", evaluation)
	}

	indefinite := EvaluateRetentionExpiry("RP-006", createdAt, time.Now().UTC().Add(20*365*24*time.Hour))
	if indefinite.Expired {
		t.Fatalf("did not expect expiry for RP-006: %+v", indefinite)
	}
}
