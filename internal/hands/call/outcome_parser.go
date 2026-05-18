package call

import (
	"strings"
)

// CallOutcome represents the parsed result of a completed call.
type CallOutcome struct {
	Success        bool           `json:"success"`
	Summary        string         `json:"summary"`
	Details        map[string]any `json:"details"`
	FollowUpRequired bool         `json:"followUpRequired"`
	FollowUpAction string         `json:"followUpAction,omitempty"`
}

// ParseOutcome analyzes a call transcript and extracts structured outcomes.
func ParseOutcome(transcript string, callType string) *CallOutcome {
	lower := strings.ToLower(transcript)

	outcome := &CallOutcome{
		Details: make(map[string]any),
	}

	// Detect general success/failure signals.
	successSignals := []string{"confirmed", "booked", "reserved", "scheduled", "all set", "you're good", "appointment is set"}
	failureSignals := []string{"sorry", "not available", "fully booked", "can't", "cannot", "no availability", "unfortunately"}

	successCount := 0
	failureCount := 0
	for _, s := range successSignals {
		if strings.Contains(lower, s) {
			successCount++
		}
	}
	for _, s := range failureSignals {
		if strings.Contains(lower, s) {
			failureCount++
		}
	}
	outcome.Success = successCount > failureCount

	switch callType {
	case "reservation":
		outcome.parseReservation(lower)
	case "appointment":
		outcome.parseAppointment(lower)
	case "quote":
		outcome.parseQuote(lower)
	default:
		outcome.Summary = summarizeTranscript(transcript)
	}

	if outcome.Summary == "" {
		outcome.Summary = summarizeTranscript(transcript)
	}

	return outcome
}

func (o *CallOutcome) parseReservation(lower string) {
	// Extract party size.
	if idx := strings.Index(lower, "party of "); idx != -1 {
		rest := lower[idx+len("party of "):]
		if len(rest) > 0 {
			size := extractLeadingDigits(rest)
			if size != "" {
				o.Details["party_size"] = size
			}
		}
	}

	// Extract confirmation signals.
	if strings.Contains(lower, "confirmed") || strings.Contains(lower, "reserved") || strings.Contains(lower, "booked") {
		if o.Success {
			o.Summary = "Reservation confirmed"
		}
	}

	// Detect date/time mentions.
	extractDateTime(lower, o)

	if !o.Success {
		o.FollowUpRequired = true
		o.FollowUpAction = "Retry reservation with alternative dates/times"
		o.Summary = "Reservation could not be confirmed"
	}
}

func (o *CallOutcome) parseAppointment(lower string) {
	if strings.Contains(lower, "scheduled") || strings.Contains(lower, "appointment is set") || strings.Contains(lower, "booked") {
		if o.Success {
			o.Summary = "Appointment scheduled"
		}
	}

	extractDateTime(lower, o)

	if !o.Success {
		o.FollowUpRequired = true
		o.FollowUpAction = "Retry appointment scheduling with alternative availability"
		o.Summary = "Appointment could not be scheduled"
	}
}

func (o *CallOutcome) parseQuote(lower string) {
	// Look for price indicators.
	priceIndicators := []string{"$", "dollars", "price", "cost", "quote", "estimate"}
	for _, p := range priceIndicators {
		if idx := strings.Index(lower, p); idx != -1 {
			// Extract surrounding context as the price detail.
			start := idx - 30
			if start < 0 {
				start = 0
			}
			end := idx + 40
			if end > len(lower) {
				end = len(lower)
			}
			o.Details["price_context"] = strings.TrimSpace(lower[start:end])
			break
		}
	}

	// Look for timeline.
	timelineIndicators := []string{"days", "weeks", "business days", "timeline", "turnaround"}
	for _, t := range timelineIndicators {
		if strings.Contains(lower, t) {
			o.Details["has_timeline"] = true
			break
		}
	}

	if o.Success {
		o.Summary = "Quote received"
	} else {
		o.Summary = "Could not obtain quote"
		o.FollowUpRequired = true
		o.FollowUpAction = "Follow up for quote details"
	}
}

func extractLeadingDigits(s string) string {
	var digits []byte
	for i := 0; i < len(s) && i < 5; i++ {
		if s[i] >= '0' && s[i] <= '9' {
			digits = append(digits, s[i])
		} else {
			break
		}
	}
	return string(digits)
}

func extractDateTime(lower string, o *CallOutcome) {
	days := []string{"monday", "tuesday", "wednesday", "thursday", "friday", "saturday", "sunday"}
	for _, day := range days {
		if strings.Contains(lower, day) {
			o.Details["day_mentioned"] = day
			break
		}
	}

	timeIndicators := []string{"am", "pm", "o'clock", "noon"}
	for _, t := range timeIndicators {
		if strings.Contains(lower, t) {
			o.Details["has_time"] = true
			break
		}
	}
}

func summarizeTranscript(transcript string) string {
	sentences := strings.Split(transcript, ".")
	if len(sentences) <= 2 {
		return strings.TrimSpace(transcript)
	}
	// Return last two meaningful sentences as summary.
	var summary []string
	for i := len(sentences) - 1; i >= 0 && len(summary) < 2; i-- {
		s := strings.TrimSpace(sentences[i])
		if len(s) > 10 {
			summary = append([]string{s}, summary...)
		}
	}
	return strings.Join(summary, ". ") + "."
}
