package control

import "testing"

func TestEvaluateMerchantRulesOrdering(t *testing.T) {
	t.Parallel()

	rules := []MerchantRule{
		{MerchantPattern: "Acme", Action: "allow", Reason: "trusted vendor"},
		{MerchantPattern: "Acme", Action: "deny", Reason: "blocked vendor"},
	}
	decision, reason := EvaluateMerchantRules(rules, "Acme", 10)
	if decision != "deny" || reason != "blocked vendor" {
		t.Fatalf("unexpected deny precedence result: decision=%s reason=%s", decision, reason)
	}

	decision, reason = EvaluateMerchantRules([]MerchantRule{
		{MerchantPattern: "Bookstore", Action: "limit", MaxAmount: 50},
	}, "Bookstore", 80)
	if decision != "require_confirm" || reason != "MERCHANT_RULE_LIMIT_EXCEEDED" {
		t.Fatalf("unexpected limit rule result: decision=%s reason=%s", decision, reason)
	}
}

func TestDetectFinancialAnomalies(t *testing.T) {
	t.Parallel()

	anomalies := DetectFinancialAnomalies(FinancialAnomalyInput{
		Amount:                     500,
		MerchantRollingAverage30d:  100,
		HistoricalTransactionsPerD: 1.5,
		TransactionsToMerchant24h:  6,
		IsNewMerchant:              true,
		InWorkHours:                false,
		TransactionsLast5Min:       4,
	})
	if len(anomalies) < 4 {
		t.Fatalf("expected multiple anomalies, got %d: %+v", len(anomalies), anomalies)
	}
	if !ContainsAnomalyRule(anomalies, "amount_outlier") || !ContainsAnomalyRule(anomalies, "rapid_succession") {
		t.Fatalf("missing expected rules: %+v", anomalies)
	}
	if !RequiresNextFinancialConfirmation(anomalies) {
		t.Fatal("expected elevated anomaly to force next confirmation")
	}
}
