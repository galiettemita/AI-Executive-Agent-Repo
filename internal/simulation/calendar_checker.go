package simulation

import (
	"fmt"
	"strings"
	"time"
)

// CalendarChecker runs constraint checks for calendar-domain plans.
type CalendarChecker struct{}

func NewCalendarChecker() *CalendarChecker { return &CalendarChecker{} }

// Check runs all calendar constraints against the proposed event and existing snapshot.
func (c *CalendarChecker) Check(in SimulationInput, snapshot CalendarSnapshot) []ConstraintViolation {
	var violations []ConstraintViolation

	if in.ProposedStartTime == nil || in.ProposedEndTime == nil {
		return []ConstraintViolation{{
			Code:        "CALENDAR_NO_TIME_EXTRACTED",
			Description: "Could not extract a proposed event time from the request.",
			Severity:    "WARN",
		}}
	}

	start := *in.ProposedStartTime
	end := *in.ProposedEndTime

	if in.ProposedTimezone != "" {
		if _, err := time.LoadLocation(in.ProposedTimezone); err != nil {
			violations = append(violations, ConstraintViolation{
				Code:        "CALENDAR_INVALID_TIMEZONE",
				Description: fmt.Sprintf("Timezone %q is not valid.", in.ProposedTimezone),
				Severity:    "BLOCK",
			})
		}
	}

	if !end.After(start) {
		violations = append(violations, ConstraintViolation{
			Code:        "CALENDAR_INVALID_DURATION",
			Description: fmt.Sprintf("Event end time (%s) is not after start time (%s).", end.Format(time.RFC3339), start.Format(time.RFC3339)),
			Severity:    "BLOCK",
		})
		return violations
	}

	if start.Before(time.Now().UTC().Add(-5 * time.Minute)) {
		violations = append(violations, ConstraintViolation{
			Code:        "CALENDAR_PAST_TIME",
			Description: fmt.Sprintf("The proposed start time (%s) is in the past.", start.Format("Mon Jan 2, 2006 at 3:04 PM MST")),
			Severity:    "WARN",
		})
	}

	for _, ev := range snapshot.Events {
		if ev.AllDay {
			continue
		}
		if start.Before(ev.EndTime) && end.After(ev.StartTime) {
			violations = append(violations, ConstraintViolation{
				Code: "CALENDAR_CONFLICT",
				Description: fmt.Sprintf("This event overlaps with '%s' (%s – %s).",
					ev.Title, ev.StartTime.Format("3:04 PM"), ev.EndTime.Format("3:04 PM MST")),
				Severity: "BLOCK",
			})
		}
	}

	for _, ev := range snapshot.Events {
		if ev.AllDay {
			continue
		}
		gapMinutes := start.Sub(ev.EndTime).Minutes()
		if gapMinutes >= 0 && gapMinutes < float64(MinTransitMinutes) {
			evLoc := strings.TrimSpace(strings.ToLower(ev.Location))
			if evLoc != "" && evLoc != "virtual" && evLoc != "zoom" && evLoc != "teams" {
				violations = append(violations, ConstraintViolation{
					Code: "CALENDAR_IMPOSSIBLE_TRANSIT",
					Description: fmt.Sprintf("'%s' ends at %s, only %.0f min before this event. Min transit: %d min.",
						ev.Title, ev.EndTime.Format("3:04 PM MST"), gapMinutes, MinTransitMinutes),
					Severity: "WARN",
				})
			}
		}
	}

	day := start.Truncate(24 * time.Hour)
	eventsOnDay := 0
	for _, ev := range snapshot.Events {
		if ev.StartTime.Truncate(24 * time.Hour).Equal(day) {
			eventsOnDay++
		}
	}
	if eventsOnDay >= MaxEventsPerDay {
		violations = append(violations, ConstraintViolation{
			Code:        "CALENDAR_OVERLOADED_DAY",
			Description: fmt.Sprintf("%s already has %d events.", start.Format("Monday, January 2"), eventsOnDay),
			Severity:    "WARN",
		})
	}

	return violations
}
