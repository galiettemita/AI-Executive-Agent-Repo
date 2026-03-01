package control

import (
	"regexp"
	"slices"
	"strings"
)

type MerchantRule struct {
	MerchantPattern string
	Action          string
	MaxAmount       float64
	Reason          string
	IsRegex         bool
}

func merchantRuleMatches(rule MerchantRule, merchant string) bool {
	pattern := strings.TrimSpace(rule.MerchantPattern)
	if pattern == "" {
		return false
	}
	if rule.IsRegex {
		re, err := regexp.Compile(pattern)
		if err != nil {
			return false
		}
		return re.MatchString(merchant)
	}
	return strings.EqualFold(strings.TrimSpace(merchant), pattern)
}

// EvaluateMerchantRules applies deny -> limit -> allow ordering.
func EvaluateMerchantRules(rules []MerchantRule, merchant string, amount float64) (decision string, reason string) {
	orderedActions := []string{"deny", "limit", "allow"}
	for _, action := range orderedActions {
		for _, rule := range rules {
			if !strings.EqualFold(strings.TrimSpace(rule.Action), action) {
				continue
			}
			if !merchantRuleMatches(rule, merchant) {
				continue
			}
			switch strings.ToLower(action) {
			case "deny":
				return "deny", firstReason(rule.Reason, "MERCHANT_RULE_DENY")
			case "limit":
				if rule.MaxAmount > 0 && amount > rule.MaxAmount {
					return "require_confirm", firstReason(rule.Reason, "MERCHANT_RULE_LIMIT_EXCEEDED")
				}
			case "allow":
				return "allow", firstReason(rule.Reason, "MERCHANT_RULE_ALLOW")
			}
		}
	}
	return "default", "MERCHANT_RULE_NO_MATCH"
}

type FinancialAnomalyInput struct {
	Amount                     float64
	Merchant                   string
	Category                   string
	MerchantRollingAverage30d  float64
	HistoricalTransactionsPerD float64
	TransactionsToMerchant24h  int
	IsNewMerchant              bool
	InWorkHours                bool
	TransactionsLast5Min       int
}

type FinancialAnomaly struct {
	Rule     string
	Severity string
}

func DetectFinancialAnomalies(input FinancialAnomalyInput) []FinancialAnomaly {
	anomalies := []FinancialAnomaly{}

	if input.MerchantRollingAverage30d > 0 && input.Amount > (3*input.MerchantRollingAverage30d) {
		anomalies = append(anomalies, FinancialAnomaly{Rule: "amount_outlier", Severity: "elevated"})
	}
	if input.TransactionsToMerchant24h > 5 && input.HistoricalTransactionsPerD < 2 {
		anomalies = append(anomalies, FinancialAnomaly{Rule: "frequency_spike", Severity: "low"})
	}
	if input.IsNewMerchant && input.Amount > 200 {
		anomalies = append(anomalies, FinancialAnomaly{Rule: "new_merchant_large", Severity: "elevated"})
	}
	if !input.InWorkHours && input.Amount > 100 {
		anomalies = append(anomalies, FinancialAnomaly{Rule: "off_hours", Severity: "low"})
	}
	if input.TransactionsLast5Min > 3 {
		anomalies = append(anomalies, FinancialAnomaly{Rule: "rapid_succession", Severity: "elevated"})
	}

	return anomalies
}

func RequiresNextFinancialConfirmation(anomalies []FinancialAnomaly) bool {
	for _, anomaly := range anomalies {
		if strings.EqualFold(anomaly.Severity, "elevated") {
			return true
		}
	}
	return false
}

func ContainsAnomalyRule(anomalies []FinancialAnomaly, rule string) bool {
	return slices.ContainsFunc(anomalies, func(anomaly FinancialAnomaly) bool {
		return strings.EqualFold(anomaly.Rule, rule)
	})
}

func firstReason(candidate, fallback string) string {
	if strings.TrimSpace(candidate) != "" {
		return candidate
	}
	return fallback
}
