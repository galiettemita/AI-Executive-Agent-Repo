package temporal_reasoning

import (
	"testing"
	"time"
)

var refTime = time.Date(2026, 3, 4, 10, 0, 0, 0, time.UTC) // Wednesday

func TestParseNextMondayAt3PM(t *testing.T) {
	t.Parallel()
	parser := NewTemporalNLPParser()
	result, err := parser.Parse("next Monday at 3pm", refTime, time.UTC)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ResolvedTime.Weekday() != time.Monday {
		t.Fatalf("expected Monday, got %s", result.ResolvedTime.Weekday())
	}
	if result.ResolvedTime.Hour() != 15 {
		t.Fatalf("expected 15:00, got %d:00", result.ResolvedTime.Hour())
	}
	if result.IsRecurring {
		t.Fatal("expected non-recurring")
	}
	if result.Confidence < 0.90 {
		t.Fatalf("expected high confidence, got %f", result.Confidence)
	}
}

func TestParseNextFriday(t *testing.T) {
	t.Parallel()
	parser := NewTemporalNLPParser()
	result, err := parser.Parse("next friday", refTime, time.UTC)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ResolvedTime.Weekday() != time.Friday {
		t.Fatalf("expected Friday, got %s", result.ResolvedTime.Weekday())
	}
	if result.ResolvedTime.Before(refTime) {
		t.Fatal("expected future date")
	}
}

func TestParseEveryWeekdayAt9AM(t *testing.T) {
	t.Parallel()
	parser := NewTemporalNLPParser()
	result, err := parser.Parse("every weekday at 9am", refTime, time.UTC)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsRecurring {
		t.Fatal("expected recurring")
	}
	if result.RecurrenceRule == "" {
		t.Fatal("expected recurrence rule")
	}
	if result.Confidence < 0.85 {
		t.Fatalf("expected high confidence, got %f", result.Confidence)
	}
}

func TestParseIn2HoursAnd30Minutes(t *testing.T) {
	t.Parallel()
	parser := NewTemporalNLPParser()
	result, err := parser.Parse("in 2 hours and 30 minutes", refTime, time.UTC)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := refTime.Add(2*time.Hour + 30*time.Minute)
	if !result.ResolvedTime.Equal(expected) {
		t.Fatalf("expected %s, got %s", expected, result.ResolvedTime)
	}
	if result.Confidence < 0.90 {
		t.Fatalf("expected high confidence, got %f", result.Confidence)
	}
}

func TestParseIn45Minutes(t *testing.T) {
	t.Parallel()
	parser := NewTemporalNLPParser()
	result, err := parser.Parse("in 45 minutes", refTime, time.UTC)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := refTime.Add(45 * time.Minute)
	if !result.ResolvedTime.Equal(expected) {
		t.Fatalf("expected %s, got %s", expected, result.ResolvedTime)
	}
}

func TestParseIn3Hours(t *testing.T) {
	t.Parallel()
	parser := NewTemporalNLPParser()
	result, err := parser.Parse("in 3 hours", refTime, time.UTC)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := refTime.Add(3 * time.Hour)
	if !result.ResolvedTime.Equal(expected) {
		t.Fatalf("expected %s, got %s", expected, result.ResolvedTime)
	}
}

func TestParseBeforeTheMeeting(t *testing.T) {
	t.Parallel()
	parser := NewTemporalNLPParser()
	result, err := parser.Parse("before the meeting", refTime, time.UTC)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.AmbiguityLevel != "high" {
		t.Fatalf("expected high ambiguity, got %s", result.AmbiguityLevel)
	}
	if result.Confidence > 0.50 {
		t.Fatalf("expected low confidence for context-dependent, got %f", result.Confidence)
	}
}

func TestParseEndOfDay(t *testing.T) {
	t.Parallel()
	parser := NewTemporalNLPParser()
	result, err := parser.Parse("end of day", refTime, time.UTC)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ResolvedTime.Hour() != 17 {
		t.Fatalf("expected 17:00, got %d:00", result.ResolvedTime.Hour())
	}
	if result.ResolvedTime.Day() != refTime.Day() {
		t.Fatalf("expected same day")
	}
}

func TestParseEndOfDayWhenPast(t *testing.T) {
	t.Parallel()
	parser := NewTemporalNLPParser()
	lateRef := time.Date(2026, 3, 4, 18, 0, 0, 0, time.UTC) // after 17:00
	result, err := parser.Parse("end of day", lateRef, time.UTC)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ResolvedTime.Day() != 5 {
		t.Fatalf("expected next day when past EOD, got day %d", result.ResolvedTime.Day())
	}
}

func TestParseEndOfWeek(t *testing.T) {
	t.Parallel()
	parser := NewTemporalNLPParser()
	result, err := parser.Parse("end of week", refTime, time.UTC) // Wednesday
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ResolvedTime.Weekday() != time.Friday {
		t.Fatalf("expected Friday, got %s", result.ResolvedTime.Weekday())
	}
}

func TestParseEndOfMonth(t *testing.T) {
	t.Parallel()
	parser := NewTemporalNLPParser()
	result, err := parser.Parse("end of month", refTime, time.UTC)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ResolvedTime.Day() != 31 {
		t.Fatalf("expected March 31, got day %d", result.ResolvedTime.Day())
	}
}

func TestParseThisAfternoon(t *testing.T) {
	t.Parallel()
	parser := NewTemporalNLPParser()
	result, err := parser.Parse("this afternoon", refTime, time.UTC) // 10am ref
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ResolvedTime.Hour() != 14 {
		t.Fatalf("expected 14:00, got %d:00", result.ResolvedTime.Hour())
	}
	if result.AmbiguityLevel != "medium" {
		t.Fatalf("expected medium ambiguity, got %s", result.AmbiguityLevel)
	}
}

func TestParseTonight(t *testing.T) {
	t.Parallel()
	parser := NewTemporalNLPParser()
	result, err := parser.Parse("tonight", refTime, time.UTC)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ResolvedTime.Hour() != 20 {
		t.Fatalf("expected 20:00, got %d:00", result.ResolvedTime.Hour())
	}
}

func TestParseThisEvening(t *testing.T) {
	t.Parallel()
	parser := NewTemporalNLPParser()
	result, err := parser.Parse("this evening", refTime, time.UTC)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ResolvedTime.Hour() != 18 {
		t.Fatalf("expected 18:00, got %d:00", result.ResolvedTime.Hour())
	}
}

func TestParseTomorrow(t *testing.T) {
	t.Parallel()
	parser := NewTemporalNLPParser()
	result, err := parser.Parse("tomorrow", refTime, time.UTC)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ResolvedTime.Day() != 5 {
		t.Fatalf("expected day 5, got %d", result.ResolvedTime.Day())
	}
	if result.Confidence < 0.90 {
		t.Fatalf("expected high confidence, got %f", result.Confidence)
	}
}

func TestParseTomorrowAt10AM(t *testing.T) {
	t.Parallel()
	parser := NewTemporalNLPParser()
	result, err := parser.Parse("tomorrow at 10am", refTime, time.UTC)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ResolvedTime.Day() != 5 || result.ResolvedTime.Hour() != 10 {
		t.Fatalf("expected March 5 at 10:00, got %s", result.ResolvedTime)
	}
}

func TestParseEmptyExpression(t *testing.T) {
	t.Parallel()
	parser := NewTemporalNLPParser()
	_, err := parser.Parse("", refTime, time.UTC)
	if err == nil {
		t.Fatal("expected error for empty expression")
	}
}

func TestParseUnrecognizedExpression(t *testing.T) {
	t.Parallel()
	parser := NewTemporalNLPParser()
	result, err := parser.Parse("sometime in the future maybe", refTime, time.UTC)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Confidence > 0.30 {
		t.Fatalf("expected low confidence for unrecognized, got %f", result.Confidence)
	}
	if result.AmbiguityLevel != "high" {
		t.Fatalf("expected high ambiguity, got %s", result.AmbiguityLevel)
	}
}

func TestParseWithTimezone(t *testing.T) {
	t.Parallel()
	parser := NewTemporalNLPParser()
	result, err := parser.ParseWithTimezone("tomorrow at 9am", refTime, "America/New_York")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ResolvedTime.Location().String() != "America/New_York" {
		t.Fatalf("expected America/New_York, got %s", result.ResolvedTime.Location())
	}
}

func TestParseWithInvalidTimezone(t *testing.T) {
	t.Parallel()
	parser := NewTemporalNLPParser()
	_, err := parser.ParseWithTimezone("tomorrow", refTime, "Invalid/Zone")
	if err == nil {
		t.Fatal("expected error for invalid timezone")
	}
}

func TestResolveDayOfWeek(t *testing.T) {
	t.Parallel()
	// refTime is Wednesday March 4, 2026
	monday := ResolveDayOfWeek("monday", refTime)
	if monday.Weekday() != time.Monday {
		t.Fatalf("expected Monday, got %s", monday.Weekday())
	}
	if monday.Day() != 9 { // next Monday
		t.Fatalf("expected March 9, got March %d", monday.Day())
	}
}

func TestResolveDayOfWeekSameDay(t *testing.T) {
	t.Parallel()
	// Wednesday -> next Wednesday should be 7 days later
	wed := ResolveDayOfWeek("wednesday", refTime)
	if wed.Weekday() != time.Wednesday {
		t.Fatalf("expected Wednesday, got %s", wed.Weekday())
	}
	if wed.Day() != 11 {
		t.Fatalf("expected March 11, got March %d", wed.Day())
	}
}

func TestResolveDayOfWeekInvalidName(t *testing.T) {
	t.Parallel()
	result := ResolveDayOfWeek("notaday", refTime)
	if !result.Equal(refTime) {
		t.Fatal("expected refTime for invalid day name")
	}
}

func TestResolveTimeOfDay(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name         string
		expectedHour int
		expectedOK   bool
	}{
		{"this morning", 9, true},
		{"this afternoon", 14, true},
		{"this evening", 18, true},
		{"tonight", 20, true},
		{"noon", 12, true},
		{"midnight", 0, true},
		{"unknown", 0, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			hour, _, ok := ResolveTimeOfDay(tc.name)
			if ok != tc.expectedOK {
				t.Fatalf("expected ok=%v, got %v", tc.expectedOK, ok)
			}
			if ok && hour != tc.expectedHour {
				t.Fatalf("expected hour %d, got %d", tc.expectedHour, hour)
			}
		})
	}
}

func TestParseEveryDayAt8AM(t *testing.T) {
	t.Parallel()
	parser := NewTemporalNLPParser()
	result, err := parser.Parse("every day at 8am", refTime, time.UTC)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsRecurring {
		t.Fatal("expected recurring")
	}
	if result.RecurrenceRule != "RRULE:FREQ=DAILY" {
		t.Fatalf("unexpected rule: %s", result.RecurrenceRule)
	}
}

func TestParseNilTimezone(t *testing.T) {
	t.Parallel()
	parser := NewTemporalNLPParser()
	result, err := parser.Parse("tomorrow", refTime, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ResolvedTime.Location() != time.UTC {
		t.Fatalf("expected UTC for nil location, got %s", result.ResolvedTime.Location())
	}
}

func TestCustomEndOfDay(t *testing.T) {
	t.Parallel()
	parser := NewTemporalNLPParser()
	parser.EndOfDay = 18
	result, err := parser.Parse("end of day", refTime, time.UTC)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ResolvedTime.Hour() != 18 {
		t.Fatalf("expected 18:00 with custom EOD, got %d:00", result.ResolvedTime.Hour())
	}
}
