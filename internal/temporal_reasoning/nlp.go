package temporal_reasoning

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// ParsedTemporal is the result of parsing a natural-language temporal
// expression into a concrete or recurring time reference.
type ParsedTemporal struct {
	Expression     string    `json:"expression"`
	ResolvedTime   time.Time `json:"resolved_time"`
	IsRecurring    bool      `json:"is_recurring"`
	RecurrenceRule string    `json:"recurrence_rule,omitempty"`
	Confidence     float64   `json:"confidence"`
	AmbiguityLevel string   `json:"ambiguity_level"`
}

// TemporalNLPParser provides enhanced temporal expression parsing with
// timezone support, recurrence detection, and relative-duration resolution.
type TemporalNLPParser struct {
	// EndOfDay configures the hour boundary for "end of day" (default 17).
	EndOfDay int
	// EndOfWeekDay configures which weekday is "end of week" (default Friday).
	EndOfWeekDay time.Weekday
}

// NewTemporalNLPParser creates a parser with sensible defaults.
func NewTemporalNLPParser() *TemporalNLPParser {
	return &TemporalNLPParser{
		EndOfDay:     17,
		EndOfWeekDay: time.Friday,
	}
}

// Parse interprets a natural-language temporal expression relative to refTime
// in the given timezone.
func (p *TemporalNLPParser) Parse(text string, refTime time.Time, tz *time.Location) (*ParsedTemporal, error) {
	if strings.TrimSpace(text) == "" {
		return nil, fmt.Errorf("empty expression")
	}
	if tz == nil {
		tz = time.UTC
	}
	refTime = refTime.In(tz)
	lower := strings.TrimSpace(strings.ToLower(text))

	// Try each parser in priority order.
	parsers := []func(string, time.Time) (*ParsedTemporal, bool){
		p.parseRecurring,
		p.parseRelativeDuration,
		p.parseNextDayOfWeekWithTime,
		p.parseNextDayOfWeek,
		p.parseTimeOfDayRange,
		p.parseEndOfPeriod,
		p.parseTomorrow,
		p.parseContextDependent,
	}

	for _, parser := range parsers {
		if result, ok := parser(lower, refTime); ok {
			result.Expression = text
			return result, nil
		}
	}

	return &ParsedTemporal{
		Expression:     text,
		ResolvedTime:   refTime,
		Confidence:     0.20,
		AmbiguityLevel: "high",
	}, nil
}

// ParseWithTimezone is a convenience wrapper that accepts a timezone name
// string instead of a *time.Location.
func (p *TemporalNLPParser) ParseWithTimezone(text string, refTime time.Time, tzName string) (*ParsedTemporal, error) {
	if tzName == "" {
		tzName = "UTC"
	}
	loc, err := time.LoadLocation(tzName)
	if err != nil {
		return nil, fmt.Errorf("invalid timezone %q: %w", tzName, err)
	}
	return p.Parse(text, refTime, loc)
}

// --- Recurring patterns ---

var recurringPatterns = []struct {
	pattern *regexp.Regexp
	rule    string
}{
	{regexp.MustCompile(`^every\s+weekday\s+at\s+(\d{1,2})\s*(am|pm)?$`), "RRULE:FREQ=WEEKLY;BYDAY=MO,TU,WE,TH,FR"},
	{regexp.MustCompile(`^every\s+day\s+at\s+(\d{1,2})\s*(am|pm)?$`), "RRULE:FREQ=DAILY"},
	{regexp.MustCompile(`^every\s+(monday|tuesday|wednesday|thursday|friday|saturday|sunday)\s+at\s+(\d{1,2})\s*(am|pm)?$`), ""},
	{regexp.MustCompile(`^every\s+week$`), "RRULE:FREQ=WEEKLY"},
	{regexp.MustCompile(`^every\s+month$`), "RRULE:FREQ=MONTHLY"},
}

func (p *TemporalNLPParser) parseRecurring(text string, refTime time.Time) (*ParsedTemporal, bool) {
	for _, rp := range recurringPatterns {
		matches := rp.pattern.FindStringSubmatch(text)
		if matches == nil {
			continue
		}

		rule := rp.rule
		resolved := refTime

		// Handle "every <weekday> at <time>"
		if strings.HasPrefix(text, "every ") && rule == "" && len(matches) >= 3 {
			dayName := matches[1]
			hour := parseHour(matches[2], "")
			if len(matches) >= 4 {
				hour = parseHour(matches[2], matches[3])
			}
			dayAbbrev := weekdayAbbrev(dayName)
			rule = fmt.Sprintf("RRULE:FREQ=WEEKLY;BYDAY=%s", dayAbbrev)
			resolved = ResolveDayOfWeek(dayName, refTime)
			resolved = time.Date(resolved.Year(), resolved.Month(), resolved.Day(), hour, 0, 0, 0, refTime.Location())
		} else if len(matches) >= 2 {
			hour := 9 // default
			if matches[1] != "" {
				hour = parseHour(matches[1], "")
				if len(matches) >= 3 && matches[2] != "" {
					hour = parseHour(matches[1], matches[2])
				}
			}
			resolved = time.Date(refTime.Year(), refTime.Month(), refTime.Day()+1, hour, 0, 0, 0, refTime.Location())
		}

		return &ParsedTemporal{
			ResolvedTime:   resolved,
			IsRecurring:    true,
			RecurrenceRule: rule,
			Confidence:     0.90,
			AmbiguityLevel: "low",
		}, true
	}
	return nil, false
}

// --- Relative duration: "in X hours and Y minutes", "in 2 hours", "in 30 minutes" ---

var relativeDurationPattern = regexp.MustCompile(
	`^in\s+(?:(\d+)\s+hours?)?\s*(?:and\s+)?(?:(\d+)\s+minutes?)?$`,
)

func (p *TemporalNLPParser) parseRelativeDuration(text string, refTime time.Time) (*ParsedTemporal, bool) {
	matches := relativeDurationPattern.FindStringSubmatch(text)
	if matches == nil {
		return nil, false
	}
	hours := 0
	minutes := 0
	if matches[1] != "" {
		hours, _ = strconv.Atoi(matches[1])
	}
	if matches[2] != "" {
		minutes, _ = strconv.Atoi(matches[2])
	}
	if hours == 0 && minutes == 0 {
		return nil, false
	}
	resolved := refTime.Add(time.Duration(hours)*time.Hour + time.Duration(minutes)*time.Minute)
	return &ParsedTemporal{
		ResolvedTime:   resolved,
		Confidence:     0.95,
		AmbiguityLevel: "low",
	}, true
}

// --- "next Monday at 3pm" ---

var nextDayAtTimePattern = regexp.MustCompile(
	`^next\s+(monday|tuesday|wednesday|thursday|friday|saturday|sunday)\s+at\s+(\d{1,2})\s*(am|pm)?$`,
)

func (p *TemporalNLPParser) parseNextDayOfWeekWithTime(text string, refTime time.Time) (*ParsedTemporal, bool) {
	matches := nextDayAtTimePattern.FindStringSubmatch(text)
	if matches == nil {
		return nil, false
	}
	dayName := matches[1]
	hour := parseHour(matches[2], matches[3])
	resolved := ResolveDayOfWeek(dayName, refTime)
	resolved = time.Date(resolved.Year(), resolved.Month(), resolved.Day(), hour, 0, 0, 0, refTime.Location())
	return &ParsedTemporal{
		ResolvedTime:   resolved,
		Confidence:     0.93,
		AmbiguityLevel: "low",
	}, true
}

// --- "next Monday" ---

var nextDayPattern = regexp.MustCompile(
	`^next\s+(monday|tuesday|wednesday|thursday|friday|saturday|sunday)$`,
)

func (p *TemporalNLPParser) parseNextDayOfWeek(text string, refTime time.Time) (*ParsedTemporal, bool) {
	matches := nextDayPattern.FindStringSubmatch(text)
	if matches == nil {
		return nil, false
	}
	resolved := ResolveDayOfWeek(matches[1], refTime)
	return &ParsedTemporal{
		ResolvedTime:   resolved,
		Confidence:     0.92,
		AmbiguityLevel: "low",
	}, true
}

// --- Time-of-day ranges: "this afternoon", "tonight", "this evening", "this morning" ---

func (p *TemporalNLPParser) parseTimeOfDayRange(text string, refTime time.Time) (*ParsedTemporal, bool) {
	hour, minute, ok := ResolveTimeOfDay(text)
	if !ok {
		return nil, false
	}
	resolved := time.Date(refTime.Year(), refTime.Month(), refTime.Day(), hour, minute, 0, 0, refTime.Location())
	// If the time is already past, interpret as the same time tomorrow.
	if resolved.Before(refTime) {
		resolved = resolved.AddDate(0, 0, 1)
	}
	return &ParsedTemporal{
		ResolvedTime:   resolved,
		Confidence:     0.80,
		AmbiguityLevel: "medium",
	}, true
}

// --- End-of-period: "end of day", "end of week", "end of month" ---

func (p *TemporalNLPParser) parseEndOfPeriod(text string, refTime time.Time) (*ParsedTemporal, bool) {
	switch text {
	case "end of day", "eod":
		resolved := time.Date(refTime.Year(), refTime.Month(), refTime.Day(), p.EndOfDay, 0, 0, 0, refTime.Location())
		if resolved.Before(refTime) {
			resolved = resolved.AddDate(0, 0, 1)
		}
		return &ParsedTemporal{
			ResolvedTime:   resolved,
			Confidence:     0.88,
			AmbiguityLevel: "low",
		}, true

	case "end of week", "eow":
		daysUntil := int(p.EndOfWeekDay-refTime.Weekday()+7) % 7
		if daysUntil == 0 && refTime.Hour() >= p.EndOfDay {
			daysUntil = 7
		}
		resolved := time.Date(refTime.Year(), refTime.Month(), refTime.Day()+daysUntil, p.EndOfDay, 0, 0, 0, refTime.Location())
		return &ParsedTemporal{
			ResolvedTime:   resolved,
			Confidence:     0.85,
			AmbiguityLevel: "low",
		}, true

	case "end of month", "eom":
		// First day of next month minus one day.
		firstOfNext := time.Date(refTime.Year(), refTime.Month()+1, 1, p.EndOfDay, 0, 0, 0, refTime.Location())
		resolved := firstOfNext.AddDate(0, 0, -1)
		return &ParsedTemporal{
			ResolvedTime:   resolved,
			Confidence:     0.88,
			AmbiguityLevel: "low",
		}, true
	}
	return nil, false
}

// --- "tomorrow" / "tomorrow at <time>" ---

var tomorrowPattern = regexp.MustCompile(`^tomorrow(?:\s+at\s+(\d{1,2})\s*(am|pm)?)?$`)

func (p *TemporalNLPParser) parseTomorrow(text string, refTime time.Time) (*ParsedTemporal, bool) {
	matches := tomorrowPattern.FindStringSubmatch(text)
	if matches == nil {
		return nil, false
	}
	tomorrow := refTime.AddDate(0, 0, 1)
	hour := 9 // default
	if matches[1] != "" {
		hour = parseHour(matches[1], matches[2])
	}
	resolved := time.Date(tomorrow.Year(), tomorrow.Month(), tomorrow.Day(), hour, 0, 0, 0, refTime.Location())
	return &ParsedTemporal{
		ResolvedTime:   resolved,
		Confidence:     0.95,
		AmbiguityLevel: "low",
	}, true
}

// --- Context-dependent: "before the meeting" ---

func (p *TemporalNLPParser) parseContextDependent(text string, refTime time.Time) (*ParsedTemporal, bool) {
	contextPhrases := []string{
		"before the meeting",
		"after the meeting",
		"before lunch",
		"after lunch",
	}
	for _, phrase := range contextPhrases {
		if text == phrase {
			return &ParsedTemporal{
				ResolvedTime:   refTime,
				Confidence:     0.30,
				AmbiguityLevel: "high",
			}, true
		}
	}
	return nil, false
}

// ResolveDayOfWeek returns the next occurrence of the named weekday after
// refTime. If refTime is already on that weekday, it returns the following
// week.
func ResolveDayOfWeek(name string, refTime time.Time) time.Time {
	weekdayIndex := map[string]time.Weekday{
		"sunday":    time.Sunday,
		"monday":    time.Monday,
		"tuesday":   time.Tuesday,
		"wednesday": time.Wednesday,
		"thursday":  time.Thursday,
		"friday":    time.Friday,
		"saturday":  time.Saturday,
	}
	target, ok := weekdayIndex[strings.ToLower(strings.TrimSpace(name))]
	if !ok {
		return refTime
	}
	offset := int(target-refTime.Weekday()+7) % 7
	if offset == 0 {
		offset = 7
	}
	return time.Date(refTime.Year(), refTime.Month(), refTime.Day()+offset, 0, 0, 0, 0, refTime.Location())
}

// ResolveTimeOfDay maps common time-of-day names to (hour, minute).
// Returns false if the name is not recognized.
func ResolveTimeOfDay(name string) (hour, minute int, ok bool) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "this morning", "morning":
		return 9, 0, true
	case "this afternoon", "afternoon":
		return 14, 0, true
	case "this evening", "evening":
		return 18, 0, true
	case "tonight":
		return 20, 0, true
	case "noon", "midday":
		return 12, 0, true
	case "midnight":
		return 0, 0, true
	default:
		return 0, 0, false
	}
}

// parseHour converts hour string and optional am/pm suffix to 24-hour int.
func parseHour(hourStr, ampm string) int {
	h, err := strconv.Atoi(hourStr)
	if err != nil {
		return 9
	}
	ampm = strings.ToLower(strings.TrimSpace(ampm))
	switch ampm {
	case "pm":
		if h < 12 {
			h += 12
		}
	case "am":
		if h == 12 {
			h = 0
		}
	}
	if h > 23 {
		h = 23
	}
	return h
}

// weekdayAbbrev returns the iCalendar two-letter abbreviation for a weekday.
func weekdayAbbrev(name string) string {
	abbrevs := map[string]string{
		"monday":    "MO",
		"tuesday":   "TU",
		"wednesday": "WE",
		"thursday":  "TH",
		"friday":    "FR",
		"saturday":  "SA",
		"sunday":    "SU",
	}
	if a, ok := abbrevs[strings.ToLower(name)]; ok {
		return a
	}
	return "MO"
}
