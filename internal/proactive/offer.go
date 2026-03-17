package proactive

import "fmt"

// OfferBuilder constructs offer-style messages for each signal type.
// RULE: Every offer MUST end with a clear action question.
// RULE: No offer may imply or suggest that action has already been taken.
type OfferBuilder struct{}

func NewOfferBuilder() *OfferBuilder { return &OfferBuilder{} }

// Build returns an offer message for a signal.
func (ob *OfferBuilder) Build(s Signal) (string, error) {
	switch s.Type {
	case SignalCalendarConflict:
		eventATitle, _ := s.Data["event_a_title"].(string)
		eventBTitle, _ := s.Data["event_b_title"].(string)
		overlapMinutes, _ := s.Data["overlap_minutes"].(float64)
		return fmt.Sprintf(
			"Heads up: *%s* and *%s* overlap by %.0f minutes. "+
				"Would you like me to suggest a reschedule? (Reply YES to get options, or NO to ignore)",
			eventATitle, eventBTitle, overlapMinutes,
		), nil

	case SignalEmailUrgency:
		sender, _ := s.Data["sender"].(string)
		subject, _ := s.Data["subject"].(string)
		return fmt.Sprintf(
			"Urgent email from *%s*: \"%s\". "+
				"Would you like me to draft a reply or flag it for immediate attention? (Reply YES or NO)",
			sender, subject,
		), nil

	case SignalDeadlineApproaching:
		task, _ := s.Data["task"].(string)
		deadline, _ := s.Data["deadline"].(string)
		return fmt.Sprintf(
			"Reminder: *%s* is due %s. "+
				"Shall I clear time on your calendar to work on this? (Reply YES or NO)",
			task, deadline,
		), nil

	case SignalTravelConflict:
		event, _ := s.Data["event_title"].(string)
		travelTime, _ := s.Data["travel_minutes"].(float64)
		return fmt.Sprintf(
			"Note: *%s* requires ~%.0f min travel, but you have back-to-back commitments before it. "+
				"Would you like me to find a solution? (Reply YES or NO)",
			event, travelTime,
		), nil

	default:
		return "", fmt.Errorf("offer_builder: unknown signal type: %s", s.Type)
	}
}
