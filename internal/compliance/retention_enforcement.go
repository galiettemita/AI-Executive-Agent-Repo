package compliance

import "time"

type RetentionEvaluation struct {
	Expired      bool
	PolicyID     string
	ExpiryAction string
	EventName    string
	ExpiresAt    time.Time
}

func EvaluateRetentionExpiry(policyID string, createdAt, now time.Time) RetentionEvaluation {
	policies := DefaultRetentionPolicies()
	policy, ok := policies[policyID]
	if !ok {
		policy = policies["RP-001"]
		policyID = "RP-001"
	}
	if policy.RetentionPeriod == 0 {
		return RetentionEvaluation{
			Expired:      false,
			PolicyID:     policyID,
			ExpiryAction: policy.ExpiryAction,
			EventName:    "BREVIO.retention.expired.v1",
			ExpiresAt:    time.Time{},
		}
	}
	expiresAt := createdAt.UTC().Add(policy.RetentionPeriod)
	expired := !now.UTC().Before(expiresAt)
	return RetentionEvaluation{
		Expired:      expired,
		PolicyID:     policyID,
		ExpiryAction: policy.ExpiryAction,
		EventName:    "BREVIO.retention.expired.v1",
		ExpiresAt:    expiresAt,
	}
}
