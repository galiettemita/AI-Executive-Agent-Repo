package simulation

import "fmt"

// FinanceChecker runs constraint checks for finance-domain plans.
type FinanceChecker struct{}

func NewFinanceChecker() *FinanceChecker { return &FinanceChecker{} }

// Check runs all financial constraints.
func (f *FinanceChecker) Check(in SimulationInput, snap FinancialSnapshot) []ConstraintViolation {
	var violations []ConstraintViolation

	if snap.WalletStatus == "frozen" || snap.WalletStatus == "closed" {
		violations = append(violations, ConstraintViolation{
			Code:        "FINANCE_WALLET_FROZEN",
			Description: fmt.Sprintf("Your wallet is currently %s.", snap.WalletStatus),
			Severity:    "BLOCK",
		})
		return violations
	}

	if in.ProposedAmountCents == nil {
		return violations
	}
	amount := *in.ProposedAmountCents

	if amount <= 0 {
		violations = append(violations, ConstraintViolation{
			Code:        "FINANCE_INVALID_AMOUNT",
			Description: fmt.Sprintf("Amount of $%.2f is invalid.", float64(amount)/100),
			Severity:    "BLOCK",
		})
		return violations
	}

	if snap.WalletBalanceCents < amount {
		shortfall := amount - snap.WalletBalanceCents
		violations = append(violations, ConstraintViolation{
			Code: "FINANCE_INSUFFICIENT_BALANCE",
			Description: fmt.Sprintf("Insufficient balance. Proposed: $%.2f, Available: $%.2f (shortfall: $%.2f).",
				float64(amount)/100, float64(snap.WalletBalanceCents)/100, float64(shortfall)/100),
			Severity: "BLOCK",
		})
	}

	if snap.MonthlyBudgetCents > 0 {
		remaining := snap.MonthlyBudgetCents - snap.MonthlySpentCents
		if amount > remaining {
			violations = append(violations, ConstraintViolation{
				Code: "FINANCE_BUDGET_CAP_EXCEEDED",
				Description: fmt.Sprintf("Payment of $%.2f exceeds remaining budget of $%.2f.",
					float64(amount)/100, float64(remaining)/100),
				Severity: "BLOCK",
			})
		} else if remaining-amount < snap.MonthlyBudgetCents/10 {
			violations = append(violations, ConstraintViolation{
				Code: "FINANCE_BUDGET_LOW",
				Description: fmt.Sprintf("This payment will leave only $%.2f of your monthly budget.",
					float64(remaining-amount)/100),
				Severity: "WARN",
			})
		}
	}

	if amount > 100_000 {
		violations = append(violations, ConstraintViolation{
			Code:        "FINANCE_LARGE_TRANSACTION",
			Description: fmt.Sprintf("Large payment of $%.2f. Please confirm.", float64(amount)/100),
			Severity:    "WARN",
		})
	}

	return violations
}
