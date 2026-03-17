package simulation

import "strings"

// ClassifyDomain inspects intent and tool keys to determine which constraint domain applies.
func ClassifyDomain(intent string, toolKeys []string) Domain {
	lower := strings.ToLower(intent)

	calendarKeywords := []string{
		"schedule", "meeting", "calendar", "appointment", "book", "reschedule",
		"block time", "event", "remind", "reminder", "call at", "zoom", "teams meeting",
	}
	financeKeywords := []string{
		"pay", "payment", "transfer", "send money", "purchase", "buy", "expense",
		"invoice", "refund", "charge", "wire", "deposit", "withdraw", "spend",
		"budget", "cost", "price", "bill",
	}

	isCalendar := false
	isFinance := false

	for _, kw := range calendarKeywords {
		if strings.Contains(lower, kw) {
			isCalendar = true
			break
		}
	}
	for _, kw := range financeKeywords {
		if strings.Contains(lower, kw) {
			isFinance = true
			break
		}
	}

	for _, tk := range toolKeys {
		tkLower := strings.ToLower(tk)
		if strings.Contains(tkLower, "calendar") || strings.Contains(tkLower, "event") {
			isCalendar = true
		}
		if strings.Contains(tkLower, "payment") || strings.Contains(tkLower, "wallet") ||
			strings.Contains(tkLower, "transfer") || strings.Contains(tkLower, "finance") {
			isFinance = true
		}
	}

	switch {
	case isCalendar && isFinance:
		return DomainMulti
	case isCalendar:
		return DomainCalendar
	case isFinance:
		return DomainFinance
	default:
		return DomainNone
	}
}
