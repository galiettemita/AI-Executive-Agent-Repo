package simulation

import "time"

// Domain classifies what class of constraint the simulator will check.
type Domain string

const (
	DomainCalendar Domain = "calendar"
	DomainFinance  Domain = "finance"
	DomainNone     Domain = "none"
	DomainMulti    Domain = "multi"
)

// ConstraintViolation describes a single failed constraint.
type ConstraintViolation struct {
	Code        string `json:"code"`
	Description string `json:"description"`
	Severity    string `json:"severity"` // "BLOCK" | "WARN"
}

// SimulationInput is the input to SimulatePlanActivity.
type SimulationInput struct {
	WorkspaceID       string     `json:"workspace_id"`
	UserID            string     `json:"user_id"`
	PlanID            string     `json:"plan_id"`
	Intent            string     `json:"intent"`
	Payload           string     `json:"payload"`
	ToolKeys          []string   `json:"tool_keys"`
	RiskLevel         string     `json:"risk_level"`
	ProposedStartTime *time.Time `json:"proposed_start_time,omitempty"`
	ProposedEndTime   *time.Time `json:"proposed_end_time,omitempty"`
	ProposedTimezone  string     `json:"proposed_timezone,omitempty"`
	ProposedAmountCents *int64   `json:"proposed_amount_cents,omitempty"`
	MerchantID        string     `json:"merchant_id,omitempty"`
}

// SimulationResult is the output of SimulatePlanActivity.
type SimulationResult struct {
	PlanID      string                `json:"plan_id"`
	Domain      Domain                `json:"domain"`
	Passed      bool                  `json:"passed"`
	Violations  []ConstraintViolation `json:"violations"`
	Warnings    []string              `json:"warnings"`
	SimulatedAt time.Time             `json:"simulated_at"`
}

// CalendarSnapshot is a lightweight read of existing calendar events.
type CalendarSnapshot struct {
	WorkspaceID string          `json:"workspace_id"`
	Events      []CalendarEvent `json:"events"`
	FetchedAt   time.Time       `json:"fetched_at"`
}

// CalendarEvent is a single existing calendar event.
type CalendarEvent struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	StartTime time.Time `json:"start_time"`
	EndTime   time.Time `json:"end_time"`
	Timezone  string    `json:"timezone"`
	Location  string    `json:"location,omitempty"`
	AllDay    bool      `json:"all_day"`
}

// FinancialSnapshot holds the current financial state for a workspace.
type FinancialSnapshot struct {
	WorkspaceID        string    `json:"workspace_id"`
	WalletBalanceCents int64     `json:"wallet_balance_cents"`
	WalletStatus       string    `json:"wallet_status"`
	MonthlyBudgetCents int64     `json:"monthly_budget_cents"`
	MonthlySpentCents  int64     `json:"monthly_spent_cents"`
	FetchedAt          time.Time `json:"fetched_at"`
}

const MinTransitMinutes = 20
const MaxEventsPerDay = 8
