package simulation

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// Simulator orchestrates constraint checks across calendar and finance domains.
type Simulator struct {
	calendarProvider CalendarSnapshotProvider
	financeProvider  FinancialSnapshotProvider
	calendarChecker  *CalendarChecker
	financeChecker   *FinanceChecker
}

func NewSimulator(calendarProvider CalendarSnapshotProvider, financeProvider FinancialSnapshotProvider) *Simulator {
	return &Simulator{
		calendarProvider: calendarProvider,
		financeProvider:  financeProvider,
		calendarChecker:  NewCalendarChecker(),
		financeChecker:   NewFinanceChecker(),
	}
}

// Simulate runs the full constraint-satisfaction pipeline for a plan.
func (s *Simulator) Simulate(ctx context.Context, in SimulationInput) (SimulationResult, error) {
	result := SimulationResult{PlanID: in.PlanID, SimulatedAt: time.Now().UTC()}
	domain := ClassifyDomain(in.Intent, in.ToolKeys)
	result.Domain = domain

	if domain == DomainNone {
		result.Passed = true
		return result, nil
	}

	// Extract structured fields from payload
	applyPayloadFields(&in)

	var allViolations []ConstraintViolation

	if domain == DomainCalendar || domain == DomainMulti {
		windowStart := time.Now().UTC()
		windowEnd := windowStart.AddDate(0, 0, 30)
		if in.ProposedStartTime != nil {
			windowStart = in.ProposedStartTime.Add(-24 * time.Hour)
			windowEnd = in.ProposedStartTime.Add(24 * time.Hour)
		}
		if s.calendarProvider != nil {
			calSnap, err := s.calendarProvider.FetchSnapshot(ctx, in.WorkspaceID, windowStart, windowEnd)
			if err != nil {
				allViolations = append(allViolations, ConstraintViolation{
					Code: "CALENDAR_SNAPSHOT_UNAVAILABLE", Description: fmt.Sprintf("Could not fetch calendar: %v", err), Severity: "WARN",
				})
			} else {
				allViolations = append(allViolations, s.calendarChecker.Check(in, calSnap)...)
			}
		}
	}

	if domain == DomainFinance || domain == DomainMulti {
		if s.financeProvider != nil {
			finSnap, err := s.financeProvider.FetchSnapshot(ctx, in.WorkspaceID)
			if err != nil {
				allViolations = append(allViolations, ConstraintViolation{
					Code: "FINANCE_SNAPSHOT_UNAVAILABLE", Description: fmt.Sprintf("Could not fetch financial data: %v", err), Severity: "WARN",
				})
			} else {
				allViolations = append(allViolations, s.financeChecker.Check(in, finSnap)...)
			}
		}
	}

	result.Violations = allViolations
	result.Passed = true
	for _, v := range allViolations {
		if v.Severity == "BLOCK" {
			result.Passed = false
		}
		if v.Severity == "WARN" {
			result.Warnings = append(result.Warnings, v.Description)
		}
	}
	return result, nil
}

func applyPayloadFields(in *SimulationInput) {
	if in.Payload == "" {
		// Fallback: try intent parsing for calendar hints
		if in.ProposedStartTime == nil {
			now := time.Now().UTC()
			lower := strings.ToLower(in.Intent)
			if strings.Contains(lower, "tomorrow") {
				tomorrow := now.AddDate(0, 0, 1)
				start := time.Date(tomorrow.Year(), tomorrow.Month(), tomorrow.Day(), 9, 0, 0, 0, time.UTC)
				end := start.Add(time.Hour)
				in.ProposedStartTime = &start
				in.ProposedEndTime = &end
			} else if strings.Contains(lower, "today") {
				start := time.Date(now.Year(), now.Month(), now.Day(), 14, 0, 0, 0, time.UTC)
				end := start.Add(time.Hour)
				in.ProposedStartTime = &start
				in.ProposedEndTime = &end
			}
		}
		return
	}

	var fields struct {
		StartTime   *time.Time `json:"start_time"`
		EndTime     *time.Time `json:"end_time"`
		Timezone    string     `json:"timezone"`
		Duration    int        `json:"duration_minutes"`
		AmountCents *int64     `json:"amount_cents"`
		AmountUSD   *float64   `json:"amount_usd"`
		MerchantID  string     `json:"merchant_id"`
	}
	_ = json.Unmarshal([]byte(in.Payload), &fields)

	if fields.StartTime != nil && in.ProposedStartTime == nil {
		in.ProposedStartTime = fields.StartTime
	}
	if fields.EndTime != nil && in.ProposedEndTime == nil {
		in.ProposedEndTime = fields.EndTime
	} else if in.ProposedEndTime == nil && in.ProposedStartTime != nil && fields.Duration > 0 {
		end := in.ProposedStartTime.Add(time.Duration(fields.Duration) * time.Minute)
		in.ProposedEndTime = &end
	}
	if fields.Timezone != "" && in.ProposedTimezone == "" {
		in.ProposedTimezone = fields.Timezone
	}
	if fields.AmountCents != nil && in.ProposedAmountCents == nil {
		in.ProposedAmountCents = fields.AmountCents
	} else if fields.AmountUSD != nil && in.ProposedAmountCents == nil {
		cents := int64(*fields.AmountUSD * 100)
		in.ProposedAmountCents = &cents
	}
	if fields.MerchantID != "" && in.MerchantID == "" {
		in.MerchantID = fields.MerchantID
	}
}
