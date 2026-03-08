package admin

import (
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// SubscriptionEvent represents a subscription lifecycle event.
type SubscriptionEvent struct {
	ID            string    `json:"id"`
	WorkspaceID   string    `json:"workspace_id"`
	UserID        string    `json:"user_id"`
	EventType     string    `json:"event_type"` // new, upgrade, downgrade, churn
	Amount        float64   `json:"amount"`
	Currency      string    `json:"currency"`
	StripeEventID string    `json:"stripe_event_id"`
	CreatedAt     time.Time `json:"created_at"`
}

// RevOpsMRRSnapshot records MRR at a point in time.
type RevOpsMRRSnapshot struct {
	ID          string    `json:"id"`
	WorkspaceID string    `json:"workspace_id"`
	Date        time.Time `json:"date"`
	MRR         float64   `json:"mrr"`
	ARR         float64   `json:"arr"`
	UserCount   int       `json:"user_count"`
	CreatedAt   time.Time `json:"created_at"`
}

// UserCohort tracks a user's cohort membership.
type UserCohort struct {
	WorkspaceID   string    `json:"workspace_id"`
	UserID        string    `json:"user_id"`
	SignupWeek    time.Time `json:"signup_week"`
	RetentionWeek int       `json:"retention_week"`
	IsActive      bool      `json:"is_active"`
}

// RevOpsOperatorMarginReport compares revenue against COGS.
type RevOpsOperatorMarginReport struct {
	WorkspaceID   string    `json:"workspace_id"`
	Date          time.Time `json:"date"`
	Revenue       float64   `json:"revenue"`
	COGS          float64   `json:"cogs"`
	Margin        float64   `json:"margin"`
	MarginPercent float64   `json:"margin_percent"`
}

// RevenueOpsService manages revenue operations.
type RevenueOpsService struct {
	mu           sync.Mutex
	events       []SubscriptionEvent
	mrrSnapshots []RevOpsMRRSnapshot
	cohorts      []UserCohort
	activeMRR    map[string]float64 // workspaceID -> MRR
	userCounts   map[string]int     // workspaceID -> user count
	cac          map[string]float64 // workspaceID -> CAC
	cogs         map[string]float64 // workspaceID -> COGS
	now          func() time.Time
}

// NewRevenueOpsService creates a new RevenueOpsService.
func NewRevenueOpsService() *RevenueOpsService {
	return &RevenueOpsService{
		events:       []SubscriptionEvent{},
		mrrSnapshots: []RevOpsMRRSnapshot{},
		cohorts:      []UserCohort{},
		activeMRR:    map[string]float64{},
		userCounts:   map[string]int{},
		cac:          map[string]float64{},
		cogs:         map[string]float64{},
		now:          func() time.Time { return time.Now().UTC() },
	}
}

// RecordSubscriptionEvent records a subscription lifecycle event.
func (s *RevenueOpsService) RecordSubscriptionEvent(evt SubscriptionEvent) (SubscriptionEvent, error) {
	if evt.WorkspaceID == "" {
		return SubscriptionEvent{}, fmt.Errorf("workspace_id is required")
	}
	if evt.EventType == "" {
		return SubscriptionEvent{}, fmt.Errorf("event_type is required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	evt.ID = uuid.Must(uuid.NewV7()).String()
	if evt.Currency == "" {
		evt.Currency = "USD"
	}
	if evt.CreatedAt.IsZero() {
		evt.CreatedAt = s.now()
	}
	s.events = append(s.events, evt)

	switch evt.EventType {
	case "new", "upgrade":
		s.activeMRR[evt.WorkspaceID] = evt.Amount
		s.userCounts[evt.WorkspaceID]++
	case "downgrade":
		s.activeMRR[evt.WorkspaceID] = evt.Amount
	case "churn":
		delete(s.activeMRR, evt.WorkspaceID)
		delete(s.userCounts, evt.WorkspaceID)
	}
	return evt, nil
}

// ComputeMRRSnapshot takes a point-in-time MRR snapshot for a workspace.
func (s *RevenueOpsService) ComputeMRRSnapshot(workspaceID string) (RevOpsMRRSnapshot, error) {
	if workspaceID == "" {
		return RevOpsMRRSnapshot{}, fmt.Errorf("workspace_id is required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	mrr := s.activeMRR[workspaceID]
	snap := RevOpsMRRSnapshot{
		ID:          uuid.Must(uuid.NewV7()).String(),
		WorkspaceID: workspaceID,
		Date:        s.now(),
		MRR:         mrr,
		ARR:         mrr * 12,
		UserCount:   s.userCounts[workspaceID],
		CreatedAt:   s.now(),
	}
	s.mrrSnapshots = append(s.mrrSnapshots, snap)
	return snap, nil
}

// GetMRR returns the current MRR for a workspace.
func (s *RevenueOpsService) GetMRR(workspaceID string) float64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.activeMRR[workspaceID]
}

// ComputeCohortRetention computes cohort membership for a user.
func (s *RevenueOpsService) ComputeCohortRetention(workspaceID, userID string, signupWeek time.Time, retentionWeek int, isActive bool) UserCohort {
	s.mu.Lock()
	defer s.mu.Unlock()

	cohort := UserCohort{
		WorkspaceID:   workspaceID,
		UserID:        userID,
		SignupWeek:    signupWeek,
		RetentionWeek: retentionWeek,
		IsActive:      isActive,
	}
	s.cohorts = append(s.cohorts, cohort)
	return cohort
}

// GetCohorts returns all cohort entries for a workspace.
func (s *RevenueOpsService) GetCohorts(workspaceID string) []UserCohort {
	s.mu.Lock()
	defer s.mu.Unlock()

	var result []UserCohort
	for _, c := range s.cohorts {
		if c.WorkspaceID == workspaceID {
			result = append(result, c)
		}
	}
	return result
}

// ComputeLTV computes a simple LTV estimate: average revenue per user * avg retention weeks.
func (s *RevenueOpsService) ComputeLTV(workspaceID string) float64 {
	s.mu.Lock()
	defer s.mu.Unlock()

	var totalRevenue float64
	var userCount int
	for _, evt := range s.events {
		if evt.WorkspaceID == workspaceID && (evt.EventType == "new" || evt.EventType == "upgrade") {
			totalRevenue += evt.Amount
			userCount++
		}
	}
	if userCount == 0 {
		return 0
	}

	// Count active cohort weeks for retention multiplier
	var totalWeeks, cohortCount int
	for _, c := range s.cohorts {
		if c.WorkspaceID == workspaceID && c.IsActive {
			totalWeeks += c.RetentionWeek
			cohortCount++
		}
	}
	avgRetention := 1.0
	if cohortCount > 0 {
		avgRetention = float64(totalWeeks) / float64(cohortCount)
	}

	avgRevenue := totalRevenue / float64(userCount)
	return avgRevenue * avgRetention
}

// SetCAC sets the customer acquisition cost for a workspace.
func (s *RevenueOpsService) SetCAC(workspaceID string, cac float64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cac[workspaceID] = cac
}

// SetCOGS sets the cost of goods sold for a workspace.
func (s *RevenueOpsService) SetCOGS(workspaceID string, cogs float64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cogs[workspaceID] = cogs
}

// GetMarginReport computes an operator margin report for a workspace.
func (s *RevenueOpsService) GetMarginReport(workspaceID string) RevOpsOperatorMarginReport {
	s.mu.Lock()
	defer s.mu.Unlock()

	var totalRevenue float64
	for _, evt := range s.events {
		if evt.WorkspaceID == workspaceID && (evt.EventType == "new" || evt.EventType == "upgrade" || evt.EventType == "downgrade") {
			totalRevenue += evt.Amount
		}
	}

	cogs := s.cogs[workspaceID]
	margin := totalRevenue - cogs
	marginPct := 0.0
	if totalRevenue > 0 {
		marginPct = (margin / totalRevenue) * 100
	}

	return RevOpsOperatorMarginReport{
		WorkspaceID:   workspaceID,
		Date:          s.now(),
		Revenue:       totalRevenue,
		COGS:          cogs,
		Margin:        margin,
		MarginPercent: marginPct,
	}
}
