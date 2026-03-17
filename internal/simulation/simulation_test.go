package simulation_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brevio/brevio/internal/simulation"
)

// --- Domain Classifier Tests ---

func TestClassifyDomain_Calendar(t *testing.T) {
	cases := []struct {
		intent   string
		toolKeys []string
	}{
		{"Schedule a meeting with Alice tomorrow at 2pm", []string{"calendar.write"}},
		{"Book a dentist appointment next Tuesday", nil},
		{"Reschedule my 3pm call to 4pm", []string{"calendar.read", "calendar.write"}},
		{"Remind me about the board meeting", nil},
	}
	for _, tc := range cases {
		assert.Equal(t, simulation.DomainCalendar, simulation.ClassifyDomain(tc.intent, tc.toolKeys), tc.intent)
	}
}

func TestClassifyDomain_Finance(t *testing.T) {
	cases := []struct {
		intent   string
		toolKeys []string
	}{
		{"Pay the invoice for $500", []string{"payment.send"}},
		{"Transfer $200 to the marketing account", nil},
		{"Send money to Bob", []string{"wallet.transfer"}},
	}
	for _, tc := range cases {
		assert.Equal(t, simulation.DomainFinance, simulation.ClassifyDomain(tc.intent, tc.toolKeys), tc.intent)
	}
}

func TestClassifyDomain_None(t *testing.T) {
	cases := []string{
		"What's the weather today?",
		"Summarize my emails",
		"Draft a reply to Alice",
	}
	for _, intent := range cases {
		assert.Equal(t, simulation.DomainNone, simulation.ClassifyDomain(intent, nil), intent)
	}
}

func TestClassifyDomain_Multi(t *testing.T) {
	intent := "Schedule a meeting and pay the venue deposit of $500"
	domain := simulation.ClassifyDomain(intent, []string{"calendar.write", "payment.send"})
	assert.Equal(t, simulation.DomainMulti, domain)
}

// --- Calendar Checker Tests ---

func TestCalendarChecker_DirectConflict(t *testing.T) {
	checker := simulation.NewCalendarChecker()
	tomorrow := time.Now().UTC().Add(24 * time.Hour)
	start := time.Date(tomorrow.Year(), tomorrow.Month(), tomorrow.Day(), 14, 0, 0, 0, time.UTC)
	end := start.Add(time.Hour)

	snap := simulation.CalendarSnapshot{
		Events: []simulation.CalendarEvent{
			{ID: "ev1", Title: "Existing Meeting", StartTime: start.Add(-30 * time.Minute), EndTime: start.Add(30 * time.Minute)},
		},
	}
	in := simulation.SimulationInput{ProposedStartTime: &start, ProposedEndTime: &end, ProposedTimezone: "UTC"}
	violations := checker.Check(in, snap)
	blocked := false
	for _, v := range violations {
		if v.Code == "CALENDAR_CONFLICT" && v.Severity == "BLOCK" {
			blocked = true
		}
	}
	assert.True(t, blocked, "direct conflict should BLOCK")
}

func TestCalendarChecker_NoConflict(t *testing.T) {
	checker := simulation.NewCalendarChecker()
	tomorrow := time.Now().UTC().Add(24 * time.Hour)
	start := time.Date(tomorrow.Year(), tomorrow.Month(), tomorrow.Day(), 14, 0, 0, 0, time.UTC)
	end := start.Add(time.Hour)

	snap := simulation.CalendarSnapshot{
		Events: []simulation.CalendarEvent{
			{ID: "ev1", Title: "Morning standup", StartTime: time.Date(tomorrow.Year(), tomorrow.Month(), tomorrow.Day(), 9, 0, 0, 0, time.UTC), EndTime: time.Date(tomorrow.Year(), tomorrow.Month(), tomorrow.Day(), 9, 30, 0, 0, time.UTC)},
		},
	}
	in := simulation.SimulationInput{ProposedStartTime: &start, ProposedEndTime: &end, ProposedTimezone: "UTC"}
	violations := checker.Check(in, snap)
	for _, v := range violations {
		assert.NotEqual(t, "BLOCK", v.Severity, "no BLOCK expected: %s", v.Code)
	}
}

func TestCalendarChecker_EndBeforeStart(t *testing.T) {
	checker := simulation.NewCalendarChecker()
	tomorrow := time.Now().UTC().Add(24 * time.Hour)
	start := time.Date(tomorrow.Year(), tomorrow.Month(), tomorrow.Day(), 15, 0, 0, 0, time.UTC)
	end := start.Add(-time.Hour)

	in := simulation.SimulationInput{ProposedStartTime: &start, ProposedEndTime: &end, ProposedTimezone: "UTC"}
	violations := checker.Check(in, simulation.CalendarSnapshot{})
	require.NotEmpty(t, violations)
	assert.Equal(t, "CALENDAR_INVALID_DURATION", violations[0].Code)
	assert.Equal(t, "BLOCK", violations[0].Severity)
}

func TestCalendarChecker_NoTimeExtracted(t *testing.T) {
	checker := simulation.NewCalendarChecker()
	in := simulation.SimulationInput{}
	violations := checker.Check(in, simulation.CalendarSnapshot{})
	require.Len(t, violations, 1)
	assert.Equal(t, "CALENDAR_NO_TIME_EXTRACTED", violations[0].Code)
	assert.Equal(t, "WARN", violations[0].Severity)
}

// --- Finance Checker Tests ---

func TestFinanceChecker_InsufficientBalance(t *testing.T) {
	checker := simulation.NewFinanceChecker()
	amount := int64(50000)
	in := simulation.SimulationInput{ProposedAmountCents: &amount}
	snap := simulation.FinancialSnapshot{WalletStatus: "active", WalletBalanceCents: 10000}
	violations := checker.Check(in, snap)
	blocked := false
	for _, v := range violations {
		if v.Code == "FINANCE_INSUFFICIENT_BALANCE" && v.Severity == "BLOCK" {
			blocked = true
		}
	}
	assert.True(t, blocked)
}

func TestFinanceChecker_SufficientBalance(t *testing.T) {
	checker := simulation.NewFinanceChecker()
	amount := int64(5000)
	in := simulation.SimulationInput{ProposedAmountCents: &amount}
	snap := simulation.FinancialSnapshot{WalletStatus: "active", WalletBalanceCents: 100000}
	violations := checker.Check(in, snap)
	for _, v := range violations {
		assert.NotEqual(t, "BLOCK", v.Severity, "sufficient balance should not BLOCK: %s", v.Code)
	}
}

func TestFinanceChecker_FrozenWallet(t *testing.T) {
	checker := simulation.NewFinanceChecker()
	amount := int64(100)
	in := simulation.SimulationInput{ProposedAmountCents: &amount}
	snap := simulation.FinancialSnapshot{WalletStatus: "frozen", WalletBalanceCents: 999999}
	violations := checker.Check(in, snap)
	require.NotEmpty(t, violations)
	assert.Equal(t, "FINANCE_WALLET_FROZEN", violations[0].Code)
	assert.Equal(t, "BLOCK", violations[0].Severity)
}

func TestFinanceChecker_BudgetCapExceeded(t *testing.T) {
	checker := simulation.NewFinanceChecker()
	amount := int64(20000)
	in := simulation.SimulationInput{ProposedAmountCents: &amount}
	snap := simulation.FinancialSnapshot{
		WalletStatus: "active", WalletBalanceCents: 100000,
		MonthlyBudgetCents: 50000, MonthlySpentCents: 45000,
	}
	violations := checker.Check(in, snap)
	blocked := false
	for _, v := range violations {
		if v.Code == "FINANCE_BUDGET_CAP_EXCEEDED" {
			blocked = true
		}
	}
	assert.True(t, blocked)
}

func TestFinanceChecker_NoAmount_NoViolations(t *testing.T) {
	checker := simulation.NewFinanceChecker()
	in := simulation.SimulationInput{}
	snap := simulation.FinancialSnapshot{WalletStatus: "active"}
	violations := checker.Check(in, snap)
	assert.Empty(t, violations)
}

// --- Simulator Integration Tests ---

func TestSimulator_DomainNone_AlwaysPasses(t *testing.T) {
	sim := simulation.NewSimulator(&simulation.NoOpCalendarProvider{}, nil)
	result, err := sim.Simulate(context.Background(), simulation.SimulationInput{
		WorkspaceID: "ws-1", PlanID: "plan-1",
		Intent: "Summarize my emails from today", ToolKeys: []string{"email.read"},
	})
	require.NoError(t, err)
	assert.True(t, result.Passed)
	assert.Equal(t, simulation.DomainNone, result.Domain)
}

func TestSimulator_CalendarConflict_BlocksPlan(t *testing.T) {
	tomorrow := time.Now().UTC().Add(24 * time.Hour)
	conflictStart := time.Date(tomorrow.Year(), tomorrow.Month(), tomorrow.Day(), 14, 0, 0, 0, time.UTC)
	conflictEnd := conflictStart.Add(time.Hour)

	calProvider := &stubCalendarProvider{events: []simulation.CalendarEvent{
		{ID: "ev1", Title: "Existing Meeting", StartTime: conflictStart, EndTime: conflictEnd},
	}}
	sim := simulation.NewSimulator(calProvider, &noOpFinanceProvider{})

	proposed := conflictStart
	proposedEnd := conflictEnd
	result, err := sim.Simulate(context.Background(), simulation.SimulationInput{
		WorkspaceID: "ws-1", PlanID: "plan-1",
		Intent: "Schedule a meeting", ToolKeys: []string{"calendar.write"},
		ProposedStartTime: &proposed, ProposedEndTime: &proposedEnd, ProposedTimezone: "UTC",
	})
	require.NoError(t, err)
	assert.False(t, result.Passed)
	assert.Equal(t, simulation.DomainCalendar, result.Domain)
}

// --- Test Helpers ---

type stubCalendarProvider struct {
	events []simulation.CalendarEvent
}

func (p *stubCalendarProvider) FetchSnapshot(_ context.Context, workspaceID string, _, _ time.Time) (simulation.CalendarSnapshot, error) {
	return simulation.CalendarSnapshot{WorkspaceID: workspaceID, Events: p.events}, nil
}

type noOpFinanceProvider struct{}

func (p *noOpFinanceProvider) FetchSnapshot(_ context.Context, workspaceID string) (simulation.FinancialSnapshot, error) {
	return simulation.FinancialSnapshot{WorkspaceID: workspaceID, WalletStatus: "active"}, nil
}
