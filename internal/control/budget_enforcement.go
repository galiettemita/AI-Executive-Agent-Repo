package control

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// BudgetEnforcer provides budget checking with durable evidence persistence.
// Every check/consume/deny is recorded as a BudgetEvent for audit reconstruction.
type BudgetEnforcer struct {
	mu            sync.Mutex
	budgetCaps    map[string]budgetState // keyed by workspace_id
	repo          ReceiptRepository
}

type budgetState struct {
	MonthlyCapUnits int
	MonthlyUsed     int
	MonthlyCapUSD   float64
	MonthlyUsedUSD  float64
	Plan            string
}

// NewBudgetEnforcer creates a budget enforcer with optional persistence.
func NewBudgetEnforcer(repo ReceiptRepository) *BudgetEnforcer {
	return &BudgetEnforcer{
		budgetCaps: make(map[string]budgetState),
		repo:       repo,
	}
}

// SetBudget configures budget caps for a workspace.
func (b *BudgetEnforcer) SetBudget(workspaceID, plan string, capUnits int, capUSD float64) {
	b.mu.Lock()
	defer b.mu.Unlock()
	state := b.budgetCaps[workspaceID]
	state.MonthlyCapUnits = capUnits
	state.MonthlyCapUSD = capUSD
	state.Plan = plan
	b.budgetCaps[workspaceID] = state
}

// BudgetCheckResult captures the result of a budget enforcement check.
type BudgetCheckResult struct {
	Allowed       bool
	Exhausted     bool
	Warning       bool
	ReasonCode    string
	RemainingUnits int
	RemainingUSD  float64
}

// Check evaluates the budget without consuming units. Returns whether the
// operation should be allowed and persists evidence.
func (b *BudgetEnforcer) Check(ctx context.Context, workspaceID string, units int, costUSD float64) BudgetCheckResult {
	b.mu.Lock()
	state, exists := b.budgetCaps[workspaceID]
	b.mu.Unlock()

	if !exists {
		// No budget configured — allow (enterprise/unlimited).
		return BudgetCheckResult{
			Allowed:    true,
			ReasonCode: "NO_BUDGET_CAP",
		}
	}

	remaining := state.MonthlyCapUnits - state.MonthlyUsed
	remainingUSD := state.MonthlyCapUSD - state.MonthlyUsedUSD

	result := BudgetCheckResult{
		Allowed:        true,
		RemainingUnits: remaining,
		RemainingUSD:   remainingUSD,
		ReasonCode:     "BUDGET_CHECK_PASSED",
	}

	// Check unit exhaustion.
	if state.MonthlyCapUnits > 0 && state.MonthlyUsed+units > state.MonthlyCapUnits {
		result.Allowed = false
		result.Exhausted = true
		result.ReasonCode = "BUDGET_UNITS_EXHAUSTED"
	}

	// Check USD exhaustion.
	if state.MonthlyCapUSD > 0 && state.MonthlyUsedUSD+costUSD > state.MonthlyCapUSD {
		result.Allowed = false
		result.Exhausted = true
		result.ReasonCode = "BUDGET_USD_EXHAUSTED"
	}

	// Warning threshold (80%).
	if state.MonthlyCapUnits > 0 {
		usageRatio := float64(state.MonthlyUsed+units) / float64(state.MonthlyCapUnits)
		if usageRatio >= 0.80 && result.Allowed {
			result.Warning = true
			result.ReasonCode = "BUDGET_WARNING_80_PERCENT"
		}
	}
	if state.MonthlyCapUSD > 0 {
		usageRatio := (state.MonthlyUsedUSD + costUSD) / state.MonthlyCapUSD
		if usageRatio >= 0.80 && result.Allowed {
			result.Warning = true
			result.ReasonCode = "BUDGET_WARNING_80_PERCENT"
		}
	}

	// Persist evidence.
	b.persistEvent(ctx, workspaceID, "", result, state, units, costUSD)

	return result
}

// Consume deducts units from the budget and persists the event. Returns error
// if budget is exhausted (deny).
func (b *BudgetEnforcer) Consume(ctx context.Context, workspaceID, receiptID string, units int, costUSD float64) error {
	b.mu.Lock()
	state, exists := b.budgetCaps[workspaceID]
	if !exists {
		b.mu.Unlock()
		return nil // No cap — unlimited.
	}

	if state.MonthlyCapUnits > 0 && state.MonthlyUsed+units > state.MonthlyCapUnits {
		b.mu.Unlock()
		result := BudgetCheckResult{
			Allowed:        false,
			Exhausted:      true,
			ReasonCode:     "BUDGET_UNITS_EXHAUSTED",
			RemainingUnits: state.MonthlyCapUnits - state.MonthlyUsed,
		}
		b.persistEvent(ctx, workspaceID, receiptID, result, state, units, costUSD)
		return fmt.Errorf("%w: workspace %s exhausted %d/%d units",
			ErrBudgetExceeded, workspaceID, state.MonthlyUsed, state.MonthlyCapUnits)
	}

	if state.MonthlyCapUSD > 0 && state.MonthlyUsedUSD+costUSD > state.MonthlyCapUSD {
		b.mu.Unlock()
		result := BudgetCheckResult{
			Allowed:      false,
			Exhausted:    true,
			ReasonCode:   "BUDGET_USD_EXHAUSTED",
			RemainingUSD: state.MonthlyCapUSD - state.MonthlyUsedUSD,
		}
		b.persistEvent(ctx, workspaceID, receiptID, result, state, units, costUSD)
		return fmt.Errorf("%w: workspace %s exhausted $%.2f/$%.2f",
			ErrBudgetExceeded, workspaceID, state.MonthlyUsedUSD, state.MonthlyCapUSD)
	}

	state.MonthlyUsed += units
	state.MonthlyUsedUSD += costUSD
	b.budgetCaps[workspaceID] = state
	b.mu.Unlock()

	result := BudgetCheckResult{
		Allowed:        true,
		ReasonCode:     "BUDGET_CONSUMED",
		RemainingUnits: state.MonthlyCapUnits - state.MonthlyUsed,
		RemainingUSD:   state.MonthlyCapUSD - state.MonthlyUsedUSD,
	}

	// Warning at 80%.
	if state.MonthlyCapUnits > 0 {
		if float64(state.MonthlyUsed)/float64(state.MonthlyCapUnits) >= 0.80 {
			result.Warning = true
		}
	}

	b.persistEvent(ctx, workspaceID, receiptID, result, state, units, costUSD)
	return nil
}

// IsExhausted returns true if the workspace budget is fully consumed.
func (b *BudgetEnforcer) IsExhausted(workspaceID string) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	state, exists := b.budgetCaps[workspaceID]
	if !exists {
		return false
	}
	if state.MonthlyCapUnits > 0 && state.MonthlyUsed >= state.MonthlyCapUnits {
		return true
	}
	if state.MonthlyCapUSD > 0 && state.MonthlyUsedUSD >= state.MonthlyCapUSD {
		return true
	}
	return false
}

func (b *BudgetEnforcer) persistEvent(ctx context.Context, workspaceID, receiptID string, result BudgetCheckResult, state budgetState, units int, costUSD float64) {
	if b.repo == nil {
		return
	}

	action := "check"
	if result.Exhausted {
		action = "deny"
	} else if result.Warning {
		action = "warn"
	} else if result.ReasonCode == "BUDGET_CONSUMED" {
		action = "consume"
	}

	_ = b.repo.StoreBudgetEvent(ctx, &BudgetEvent{
		WorkspaceID: workspaceID,
		ReceiptID:   receiptID,
		Action:      action,
		UnitsUsed:   state.MonthlyUsed,
		UnitsCap:    state.MonthlyCapUnits,
		CostUSD:     state.MonthlyUsedUSD,
		CapUSD:      state.MonthlyCapUSD,
		Evidence: map[string]any{
			"plan":             state.Plan,
			"requested_units":  units,
			"requested_usd":    costUSD,
			"remaining_units":  result.RemainingUnits,
			"remaining_usd":    result.RemainingUSD,
			"period":           time.Now().UTC().Format("2006-01"),
			"threshold_80_pct": result.Warning,
		},
		CreatedAt: time.Now().UTC(),
	})
}
