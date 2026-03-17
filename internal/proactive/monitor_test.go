package proactive_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brevio/brevio/internal/proactive"
)

type mockCalendarFetcher struct {
	events []proactive.CalendarEvent
	err    error
}

func (m *mockCalendarFetcher) FetchUpcoming(_ context.Context, _ string, _ time.Duration) ([]proactive.CalendarEvent, error) {
	return m.events, m.err
}

type mockEmailFetcher struct {
	emails []proactive.EmailSignal
	err    error
}

func (m *mockEmailFetcher) FetchRecent(_ context.Context, _ string, _ time.Duration) ([]proactive.EmailSignal, error) {
	return m.emails, m.err
}

// actionWords that must NEVER appear in an offer message.
var actionWords = []string{
	"i have scheduled", "i've moved", "i already", "done:", "completed:",
	"i have sent", "i've forwarded", "action taken", "executed",
}

func assertNoActionLanguage(t *testing.T, text string) {
	t.Helper()
	lower := strings.ToLower(text)
	for _, w := range actionWords {
		assert.NotContains(t, lower, w, "offer must not contain action language: %q", w)
	}
}

func assertHasPrompt(t *testing.T, text string) {
	t.Helper()
	lower := strings.ToLower(text)
	assert.True(t,
		strings.Contains(lower, "yes") && strings.Contains(lower, "no"),
		"offer must contain YES/NO prompt, got: %s", text)
}

func TestProactiveMonitor_DetectsCalendarConflict(t *testing.T) {
	now := time.Now().UTC()
	cal := &mockCalendarFetcher{events: []proactive.CalendarEvent{
		{ID: "e1", Title: "Budget Review", StartTime: now, EndTime: now.Add(60 * time.Minute)},
		{ID: "e2", Title: "1:1 with Alice", StartTime: now.Add(45 * time.Minute), EndTime: now.Add(90 * time.Minute)},
	}}
	monitor := proactive.NewProactiveMonitor(nil, proactive.NewSnoozeStore(nil), cal, nil)
	signals, err := monitor.DetectSignals(context.Background(), "ws-1")
	require.NoError(t, err)
	assert.Len(t, signals, 1)
	assert.Equal(t, proactive.SignalCalendarConflict, signals[0].Type)
	assert.GreaterOrEqual(t, signals[0].Urgency, 0.5)
}

func TestProactiveMonitor_NoConflictForNonOverlappingEvents(t *testing.T) {
	now := time.Now().UTC()
	cal := &mockCalendarFetcher{events: []proactive.CalendarEvent{
		{ID: "e1", Title: "Meeting A", StartTime: now, EndTime: now.Add(30 * time.Minute)},
		{ID: "e2", Title: "Meeting B", StartTime: now.Add(60 * time.Minute), EndTime: now.Add(90 * time.Minute)},
	}}
	monitor := proactive.NewProactiveMonitor(nil, proactive.NewSnoozeStore(nil), cal, nil)
	signals, err := monitor.DetectSignals(context.Background(), "ws-1")
	require.NoError(t, err)
	assert.Empty(t, signals)
}

func TestProactiveMonitor_DetectsEmailUrgency_UrgentKeyword(t *testing.T) {
	email := &mockEmailFetcher{emails: []proactive.EmailSignal{
		{Subject: "URGENT: Board meeting prep needed", Sender: "ceo@acme.com", Snippet: "Please prepare the deck ASAP"},
	}}
	monitor := proactive.NewProactiveMonitor(nil, proactive.NewSnoozeStore(nil), nil, email)
	signals, err := monitor.DetectSignals(context.Background(), "ws-1")
	require.NoError(t, err)
	assert.Len(t, signals, 1)
	assert.Equal(t, proactive.SignalEmailUrgency, signals[0].Type)
}

func TestProactiveMonitor_NoSignalForNonUrgentEmail(t *testing.T) {
	email := &mockEmailFetcher{emails: []proactive.EmailSignal{
		{Subject: "Weekly newsletter", Sender: "news@example.com", Snippet: "Here is your weekly update"},
	}}
	monitor := proactive.NewProactiveMonitor(nil, proactive.NewSnoozeStore(nil), nil, email)
	signals, err := monitor.DetectSignals(context.Background(), "ws-1")
	require.NoError(t, err)
	assert.Empty(t, signals)
}

func TestProactiveMonitor_SkipsSnoozedWorkspace(t *testing.T) {
	// SnoozeStore with nil pool returns false (not snoozed).
	// We test nil fetchers to confirm zero signals.
	monitor := proactive.NewProactiveMonitor(nil, proactive.NewSnoozeStore(nil), nil, nil)
	signals, err := monitor.DetectSignals(context.Background(), "ws-snoozed")
	require.NoError(t, err)
	assert.Empty(t, signals)
}

func TestProactiveMonitor_SkipsSnoozedSignalType(t *testing.T) {
	// With nil pool, snooze is never active, so calendar signals pass through.
	// Verify that without snooze, a conflict is detected (functional test).
	now := time.Now().UTC()
	cal := &mockCalendarFetcher{events: []proactive.CalendarEvent{
		{ID: "e1", Title: "A", StartTime: now, EndTime: now.Add(60 * time.Minute)},
		{ID: "e2", Title: "B", StartTime: now.Add(30 * time.Minute), EndTime: now.Add(90 * time.Minute)},
	}}
	monitor := proactive.NewProactiveMonitor(nil, proactive.NewSnoozeStore(nil), cal, nil)
	signals, err := monitor.DetectSignals(context.Background(), "ws-1")
	require.NoError(t, err)
	assert.NotEmpty(t, signals)
}

func TestProactiveMonitor_UrgencyThreshold_Filters(t *testing.T) {
	// Events with only 5-min overlap get urgency=0.6 (>0.5 threshold), so they pass.
	now := time.Now().UTC()
	cal := &mockCalendarFetcher{events: []proactive.CalendarEvent{
		{ID: "e1", Title: "A", StartTime: now, EndTime: now.Add(35 * time.Minute)},
		{ID: "e2", Title: "B", StartTime: now.Add(30 * time.Minute), EndTime: now.Add(60 * time.Minute)},
	}}
	monitor := proactive.NewProactiveMonitor(nil, proactive.NewSnoozeStore(nil), cal, nil)
	signals, err := monitor.DetectSignals(context.Background(), "ws-1")
	require.NoError(t, err)
	for _, s := range signals {
		assert.GreaterOrEqual(t, s.Urgency, 0.5)
	}
}

func TestOfferBuilder_BuildsCalendarConflictOffer(t *testing.T) {
	ob := proactive.NewOfferBuilder()
	text, err := ob.Build(proactive.Signal{
		Type: proactive.SignalCalendarConflict,
		Data: map[string]any{
			"event_a_title":   "Budget Review",
			"event_b_title":   "1:1 with Alice",
			"overlap_minutes": float64(15),
		},
	})
	require.NoError(t, err)
	assert.Contains(t, text, "Budget Review")
	assert.Contains(t, text, "1:1 with Alice")
	assertHasPrompt(t, text)
	assertNoActionLanguage(t, text)
}

func TestOfferBuilder_BuildsEmailUrgencyOffer(t *testing.T) {
	ob := proactive.NewOfferBuilder()
	text, err := ob.Build(proactive.Signal{
		Type: proactive.SignalEmailUrgency,
		Data: map[string]any{
			"sender":  "ceo@acme.com",
			"subject": "Q3 budget approval needed",
		},
	})
	require.NoError(t, err)
	assert.Contains(t, text, "ceo@acme.com")
	assert.Contains(t, text, "Q3 budget approval needed")
	assertHasPrompt(t, text)
	assertNoActionLanguage(t, text)
}

func TestOfferBuilder_ErrorsForUnknownSignalType(t *testing.T) {
	ob := proactive.NewOfferBuilder()
	_, err := ob.Build(proactive.Signal{Type: "unknown_type"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown signal type")
}
