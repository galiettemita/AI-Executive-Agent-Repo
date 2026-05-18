package admin

import (
	"fmt"
	"math"
	"sort"
	"sync"
	"time"
)

// ---------------------------------------------------------------------------
// CostAttribution & LLMCostLedger
// ---------------------------------------------------------------------------

// CostAttribution represents a single LLM invocation cost record.
type CostAttribution struct {
	WorkspaceID   string `json:"workspace_id"`
	UserID        string `json:"user_id"`
	ToolKey       string `json:"tool_key"`
	TokensUsed    int64  `json:"tokens_used"`
	CostMicroCents int64  `json:"cost_micro_cents"`
	Model         string `json:"model"`
	Timestamp     time.Time `json:"timestamp"`
}

// CostBreakdown summarises costs for a user within a time window.
type CostBreakdown struct {
	TotalMicroCents int64            `json:"total_micro_cents"`
	ByModel         map[string]int64 `json:"by_model"`
	ByTool          map[string]int64 `json:"by_tool"`
	InvocationCount int              `json:"invocation_count"`
}

// WorkspaceCostSummary provides workspace-level cost projections.
type WorkspaceCostSummary struct {
	TotalMicroCents          int64   `json:"total_micro_cents"`
	ProjectedMonthlyMicroCents int64 `json:"projected_monthly_micro_cents"`
	BudgetMicroCents         int64   `json:"budget_micro_cents"`
	BurnPct                  float64 `json:"burn_pct"`
}

// OperatorMarginReport compares revenue against cost.
type OperatorMarginReport struct {
	RevenueMicroCents int64   `json:"revenue_micro_cents"`
	CostMicroCents    int64   `json:"cost_micro_cents"`
	MarginPct         float64 `json:"margin_pct"`
}

// LLMCostLedger tracks per-invocation LLM costs at micro-cent precision.
type LLMCostLedger struct {
	mu      sync.RWMutex
	entries []CostAttribution
	// budgets per workspace in micro-cents
	budgets map[string]int64
	// revenue per workspace in micro-cents (from subscriptions)
	revenue map[string]int64
	now     func() time.Time
}

// NewLLMCostLedger creates a new cost ledger.
func NewLLMCostLedger() *LLMCostLedger {
	return &LLMCostLedger{
		entries: []CostAttribution{},
		budgets: map[string]int64{},
		revenue: map[string]int64{},
		now:     func() time.Time { return time.Now().UTC() },
	}
}

// SetBudget sets the monthly budget for a workspace in micro-cents.
func (l *LLMCostLedger) SetBudget(workspaceID string, budgetMicroCents int64) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.budgets[workspaceID] = budgetMicroCents
}

// SetRevenue sets the revenue for a workspace in micro-cents.
func (l *LLMCostLedger) SetRevenue(workspaceID string, revenueMicroCents int64) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.revenue[workspaceID] = revenueMicroCents
}

// RecordCost records a single cost attribution entry.
func (l *LLMCostLedger) RecordCost(attr CostAttribution) error {
	if attr.WorkspaceID == "" {
		return fmt.Errorf("workspace_id is required")
	}
	if attr.UserID == "" {
		return fmt.Errorf("user_id is required")
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if attr.Timestamp.IsZero() {
		attr.Timestamp = l.now()
	}
	l.entries = append(l.entries, attr)
	return nil
}

// GetUserCostBreakdown returns a cost breakdown for a specific user within a time range.
func (l *LLMCostLedger) GetUserCostBreakdown(workspaceID, userID string, from, to time.Time) (*CostBreakdown, error) {
	if workspaceID == "" {
		return nil, fmt.Errorf("workspace_id is required")
	}
	if userID == "" {
		return nil, fmt.Errorf("user_id is required")
	}
	l.mu.RLock()
	defer l.mu.RUnlock()

	bd := &CostBreakdown{
		ByModel: map[string]int64{},
		ByTool:  map[string]int64{},
	}

	for _, e := range l.entries {
		if e.WorkspaceID != workspaceID || e.UserID != userID {
			continue
		}
		if e.Timestamp.Before(from) || e.Timestamp.After(to) {
			continue
		}
		bd.TotalMicroCents += e.CostMicroCents
		bd.ByModel[e.Model] += e.CostMicroCents
		bd.ByTool[e.ToolKey] += e.CostMicroCents
		bd.InvocationCount++
	}
	return bd, nil
}

// GetWorkspaceCostSummary returns workspace-level cost summary with projections.
func (l *LLMCostLedger) GetWorkspaceCostSummary(workspaceID string) (*WorkspaceCostSummary, error) {
	if workspaceID == "" {
		return nil, fmt.Errorf("workspace_id is required")
	}
	l.mu.RLock()
	defer l.mu.RUnlock()

	var totalCost int64
	var earliest, latest time.Time
	for _, e := range l.entries {
		if e.WorkspaceID != workspaceID {
			continue
		}
		totalCost += e.CostMicroCents
		if earliest.IsZero() || e.Timestamp.Before(earliest) {
			earliest = e.Timestamp
		}
		if latest.IsZero() || e.Timestamp.After(latest) {
			latest = e.Timestamp
		}
	}

	summary := &WorkspaceCostSummary{
		TotalMicroCents: totalCost,
		BudgetMicroCents: l.budgets[workspaceID],
	}

	// Project monthly cost based on daily burn rate
	if !earliest.IsZero() && !latest.IsZero() {
		daySpan := latest.Sub(earliest).Hours() / 24
		if daySpan > 0 {
			dailyRate := float64(totalCost) / daySpan
			summary.ProjectedMonthlyMicroCents = int64(dailyRate * 30)
		} else {
			summary.ProjectedMonthlyMicroCents = totalCost * 30
		}
	}

	if summary.BudgetMicroCents > 0 {
		summary.BurnPct = float64(totalCost) / float64(summary.BudgetMicroCents) * 100
	}
	return summary, nil
}

// GetMarginReport computes operator margin for a workspace.
func (l *LLMCostLedger) GetMarginReport(workspaceID string) *OperatorMarginReport {
	l.mu.RLock()
	defer l.mu.RUnlock()

	var totalCost int64
	for _, e := range l.entries {
		if e.WorkspaceID != workspaceID {
			continue
		}
		totalCost += e.CostMicroCents
	}

	rev := l.revenue[workspaceID]
	report := &OperatorMarginReport{
		RevenueMicroCents: rev,
		CostMicroCents:    totalCost,
	}
	if rev > 0 {
		report.MarginPct = float64(rev-totalCost) / float64(rev) * 100
	}
	return report
}

// ---------------------------------------------------------------------------
// MRRTracker — monthly recurring revenue
// ---------------------------------------------------------------------------

// MRRSnapshot records MRR at a point in time.
type MRRSnapshot struct {
	Timestamp time.Time `json:"timestamp"`
	MRRCents  int64     `json:"mrr_cents"`
}

type subscriptionRecord struct {
	WorkspaceID string
	Plan        string
	AmountCents int64
	EventType   string
	Timestamp   time.Time
}

// MRRTracker tracks monthly recurring revenue from subscription events.
type MRRTracker struct {
	mu            sync.RWMutex
	subscriptions []subscriptionRecord
	// active MRR per workspace
	activeMRR map[string]int64
	snapshots []MRRSnapshot
	now       func() time.Time
}

// NewMRRTracker creates a new MRR tracker.
func NewMRRTracker() *MRRTracker {
	return &MRRTracker{
		subscriptions: []subscriptionRecord{},
		activeMRR:     map[string]int64{},
		snapshots:     []MRRSnapshot{},
		now:           func() time.Time { return time.Now().UTC() },
	}
}

// RecordSubscriptionEvent records a subscription lifecycle event.
func (m *MRRTracker) RecordSubscriptionEvent(workspaceID, plan string, amountCents int64, eventType string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	rec := subscriptionRecord{
		WorkspaceID: workspaceID,
		Plan:        plan,
		AmountCents: amountCents,
		EventType:   eventType,
		Timestamp:   m.now(),
	}
	m.subscriptions = append(m.subscriptions, rec)

	switch eventType {
	case "new", "upgrade":
		m.activeMRR[workspaceID] = amountCents
	case "downgrade":
		m.activeMRR[workspaceID] = amountCents
	case "churn":
		delete(m.activeMRR, workspaceID)
	}

	// Snapshot current MRR
	total := m.totalMRRLocked()
	m.snapshots = append(m.snapshots, MRRSnapshot{
		Timestamp: rec.Timestamp,
		MRRCents:  total,
	})
}

// GetMRR returns current total MRR in cents.
func (m *MRRTracker) GetMRR() int64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.totalMRRLocked()
}

func (m *MRRTracker) totalMRRLocked() int64 {
	var total int64
	for _, v := range m.activeMRR {
		total += v
	}
	return total
}

// GetMRRSnapshots returns the most recent N MRR snapshots.
func (m *MRRTracker) GetMRRSnapshots(limit int) []MRRSnapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if limit <= 0 || limit > len(m.snapshots) {
		limit = len(m.snapshots)
	}
	start := len(m.snapshots) - limit
	out := make([]MRRSnapshot, limit)
	copy(out, m.snapshots[start:])
	return out
}

// ---------------------------------------------------------------------------
// CohortTracker — user retention cohorts
// ---------------------------------------------------------------------------

// CohortRetention holds retention metrics for a cohort.
type CohortRetention struct {
	CohortWeek   time.Time `json:"cohort_week"`
	TotalUsers   int       `json:"total_users"`
	RetainedUsers int      `json:"retained_users"`
	RetentionPct float64   `json:"retention_pct"`
}

type userActivity struct {
	WorkspaceID  string
	UserID       string
	ActivityDate time.Time
}

// CohortTracker tracks user retention cohorts.
type CohortTracker struct {
	mu         sync.RWMutex
	activities []userActivity
}

// NewCohortTracker creates a new cohort tracker.
func NewCohortTracker() *CohortTracker {
	return &CohortTracker{
		activities: []userActivity{},
	}
}

// RecordUserActivity records a user activity for cohort tracking.
func (c *CohortTracker) RecordUserActivity(workspaceID, userID string, activityDate time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.activities = append(c.activities, userActivity{
		WorkspaceID:  workspaceID,
		UserID:       userID,
		ActivityDate: activityDate,
	})
}

// ComputeRetention computes retention for a cohort week over a number of days.
func (c *CohortTracker) ComputeRetention(cohortWeek time.Time, days int) *CohortRetention {
	c.mu.RLock()
	defer c.mu.RUnlock()

	cohortEnd := cohortWeek.AddDate(0, 0, 7)
	retentionDeadline := cohortWeek.AddDate(0, 0, days)

	// Find users active during the cohort week
	cohortUsers := map[string]struct{}{}
	for _, a := range c.activities {
		if !a.ActivityDate.Before(cohortWeek) && a.ActivityDate.Before(cohortEnd) {
			cohortUsers[a.WorkspaceID+":"+a.UserID] = struct{}{}
		}
	}

	if len(cohortUsers) == 0 {
		return &CohortRetention{CohortWeek: cohortWeek}
	}

	// Find which cohort users were active after the retention period
	retained := map[string]struct{}{}
	for _, a := range c.activities {
		key := a.WorkspaceID + ":" + a.UserID
		if _, inCohort := cohortUsers[key]; !inCohort {
			continue
		}
		if !a.ActivityDate.Before(retentionDeadline) {
			retained[key] = struct{}{}
		}
	}

	total := len(cohortUsers)
	retainedCount := len(retained)
	pct := 0.0
	if total > 0 {
		pct = float64(retainedCount) / float64(total) * 100
	}

	return &CohortRetention{
		CohortWeek:   cohortWeek,
		TotalUsers:   total,
		RetainedUsers: retainedCount,
		RetentionPct: pct,
	}
}

// ---------------------------------------------------------------------------
// SkillACLService — per-user skill access control
// ---------------------------------------------------------------------------

// SkillACL defines allowed and denied skills for a user.
type SkillACL struct {
	WorkspaceID   string   `json:"workspace_id"`
	UserID        string   `json:"user_id"`
	AllowedSkills []string `json:"allowed_skills"`
	DeniedSkills  []string `json:"denied_skills"`
}

// SkillACLService manages per-user skill access control.
type SkillACLService struct {
	mu   sync.RWMutex
	acls map[string]*SkillACL // key: "workspaceID:userID"
}

// NewSkillACLService creates a new skill ACL service.
func NewSkillACLService() *SkillACLService {
	return &SkillACLService{
		acls: map[string]*SkillACL{},
	}
}

func skillACLKey(workspaceID, userID string) string {
	return workspaceID + ":" + userID
}

// SetSkillACL sets skill access control for a user.
func (s *SkillACLService) SetSkillACL(workspaceID, userID string, allowedSkills, deniedSkills []string) error {
	if workspaceID == "" {
		return fmt.Errorf("workspace_id is required")
	}
	if userID == "" {
		return fmt.Errorf("user_id is required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.acls[skillACLKey(workspaceID, userID)] = &SkillACL{
		WorkspaceID:   workspaceID,
		UserID:        userID,
		AllowedSkills: allowedSkills,
		DeniedSkills:  deniedSkills,
	}
	return nil
}

// CheckSkillAccess checks whether a user can access a specific skill.
func (s *SkillACLService) CheckSkillAccess(workspaceID, userID, skillID string) (bool, string) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	acl, ok := s.acls[skillACLKey(workspaceID, userID)]
	if !ok {
		// No ACL means default allow
		return true, "no ACL configured, default allow"
	}

	// Check denied list first
	for _, denied := range acl.DeniedSkills {
		if denied == skillID {
			return false, fmt.Sprintf("skill %q is explicitly denied", skillID)
		}
	}

	// If an allow list exists, the skill must be in it
	if len(acl.AllowedSkills) > 0 {
		for _, allowed := range acl.AllowedSkills {
			if allowed == skillID {
				return true, "skill is in allowed list"
			}
		}
		return false, fmt.Sprintf("skill %q is not in allowed list", skillID)
	}

	return true, "skill is not denied"
}

// GetSkillACL returns the ACL for a user.
func (s *SkillACLService) GetSkillACL(workspaceID, userID string) *SkillACL {
	s.mu.RLock()
	defer s.mu.RUnlock()
	acl, ok := s.acls[skillACLKey(workspaceID, userID)]
	if !ok {
		return &SkillACL{
			WorkspaceID:   workspaceID,
			UserID:        userID,
			AllowedSkills: []string{},
			DeniedSkills:  []string{},
		}
	}
	cp := *acl
	return &cp
}

// ---------------------------------------------------------------------------
// BehavioralRiskScorer
// ---------------------------------------------------------------------------

// RiskScore represents a computed risk assessment for a user.
type RiskScore struct {
	Score      float64   `json:"score"`
	Level      string    `json:"level"`      // low, medium, high, critical
	Factors    []string  `json:"factors"`
	ComputedAt time.Time `json:"computed_at"`
}

type actionRecord struct {
	WorkspaceID string
	UserID      string
	Action      string
	Metadata    map[string]any
	Timestamp   time.Time
}

// BehavioralRiskScorer computes risk scores based on user actions.
type BehavioralRiskScorer struct {
	mu      sync.RWMutex
	actions []actionRecord
	now     func() time.Time
}

// NewBehavioralRiskScorer creates a new risk scorer.
func NewBehavioralRiskScorer() *BehavioralRiskScorer {
	return &BehavioralRiskScorer{
		actions: []actionRecord{},
		now:     func() time.Time { return time.Now().UTC() },
	}
}

// RecordAction records a user action for risk analysis.
func (b *BehavioralRiskScorer) RecordAction(workspaceID, userID, action string, metadata map[string]any) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.actions = append(b.actions, actionRecord{
		WorkspaceID: workspaceID,
		UserID:      userID,
		Action:      action,
		Metadata:    metadata,
		Timestamp:   b.now(),
	})
}

// ComputeRiskScore evaluates behavioral risk for a user.
func (b *BehavioralRiskScorer) ComputeRiskScore(workspaceID, userID string) *RiskScore {
	b.mu.RLock()
	defer b.mu.RUnlock()

	var factors []string
	score := 0.0

	var userActions []actionRecord
	for _, a := range b.actions {
		if a.WorkspaceID == workspaceID && a.UserID == userID {
			userActions = append(userActions, a)
		}
	}

	// Factor 1: unusual hours (actions outside 06:00-22:00)
	unusualHourCount := 0
	for _, a := range userActions {
		hour := a.Timestamp.Hour()
		if hour < 6 || hour > 22 {
			unusualHourCount++
		}
	}
	if unusualHourCount > 0 {
		hourScore := math.Min(float64(unusualHourCount)*10, 30)
		score += hourScore
		factors = append(factors, fmt.Sprintf("unusual_hours:%d", unusualHourCount))
	}

	// Factor 2: high-volume actions (more than 100 in recent window)
	if len(userActions) > 100 {
		volumeScore := math.Min(float64(len(userActions)-100)*0.5, 30)
		score += volumeScore
		factors = append(factors, fmt.Sprintf("high_volume:%d", len(userActions)))
	}

	// Factor 3: permission escalation attempts
	escalationCount := 0
	for _, a := range userActions {
		if a.Action == "permission_escalation" || a.Action == "role_change" || a.Action == "admin_access_attempt" {
			escalationCount++
		}
	}
	if escalationCount > 0 {
		escScore := math.Min(float64(escalationCount)*20, 40)
		score += escScore
		factors = append(factors, fmt.Sprintf("escalation_attempts:%d", escalationCount))
	}

	// Clamp score to 0-100
	score = math.Min(score, 100)

	level := "low"
	switch {
	case score >= 80:
		level = "critical"
	case score >= 50:
		level = "high"
	case score >= 25:
		level = "medium"
	}

	return &RiskScore{
		Score:      score,
		Level:      level,
		Factors:    factors,
		ComputedAt: b.now(),
	}
}

// ---------------------------------------------------------------------------
// OAuthExpiryTracker
// ---------------------------------------------------------------------------

// ExpiringToken represents an OAuth token nearing expiry.
type ExpiringToken struct {
	WorkspaceID   string    `json:"workspace_id"`
	Provider      string    `json:"provider"`
	TokenID       string    `json:"token_id"`
	ExpiresAt     time.Time `json:"expires_at"`
	DaysRemaining int       `json:"days_remaining"`
}

// OAuthExpiryTracker monitors OAuth token expiry.
type OAuthExpiryTracker struct {
	mu     sync.RWMutex
	tokens []ExpiringToken
	now    func() time.Time
}

// NewOAuthExpiryTracker creates a new OAuth expiry tracker.
func NewOAuthExpiryTracker() *OAuthExpiryTracker {
	return &OAuthExpiryTracker{
		tokens: []ExpiringToken{},
		now:    func() time.Time { return time.Now().UTC() },
	}
}

// TrackToken registers a token for expiry monitoring.
func (o *OAuthExpiryTracker) TrackToken(workspaceID, provider, tokenID string, expiresAt time.Time) {
	o.mu.Lock()
	defer o.mu.Unlock()

	// Upsert by tokenID
	for i, t := range o.tokens {
		if t.TokenID == tokenID {
			o.tokens[i].ExpiresAt = expiresAt
			o.tokens[i].WorkspaceID = workspaceID
			o.tokens[i].Provider = provider
			return
		}
	}
	o.tokens = append(o.tokens, ExpiringToken{
		WorkspaceID: workspaceID,
		Provider:    provider,
		TokenID:     tokenID,
		ExpiresAt:   expiresAt,
	})
}

// GetExpiringSoon returns tokens expiring within the given duration.
func (o *OAuthExpiryTracker) GetExpiringSoon(within time.Duration) []ExpiringToken {
	o.mu.RLock()
	defer o.mu.RUnlock()

	now := o.now()
	deadline := now.Add(within)
	var result []ExpiringToken

	for _, t := range o.tokens {
		if t.ExpiresAt.After(now) && !t.ExpiresAt.After(deadline) {
			tok := t
			tok.DaysRemaining = int(t.ExpiresAt.Sub(now).Hours() / 24)
			result = append(result, tok)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].ExpiresAt.Before(result[j].ExpiresAt)
	})
	return result
}

// ---------------------------------------------------------------------------
// ToolMTTRTracker — mean time to recovery
// ---------------------------------------------------------------------------

type failureRecord struct {
	ToolKey   string
	FailedAt  time.Time
	Recovered bool
	RecoveredAt time.Time
}

// ToolMTTRTracker tracks mean time to recovery for tools.
type ToolMTTRTracker struct {
	mu       sync.RWMutex
	failures []failureRecord
}

// NewToolMTTRTracker creates a new MTTR tracker.
func NewToolMTTRTracker() *ToolMTTRTracker {
	return &ToolMTTRTracker{
		failures: []failureRecord{},
	}
}

// RecordFailure records a tool failure event.
func (t *ToolMTTRTracker) RecordFailure(toolKey string, failedAt time.Time) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.failures = append(t.failures, failureRecord{
		ToolKey:  toolKey,
		FailedAt: failedAt,
	})
}

// RecordRecovery marks the most recent unrecovered failure for the tool as recovered.
func (t *ToolMTTRTracker) RecordRecovery(toolKey string, recoveredAt time.Time) {
	t.mu.Lock()
	defer t.mu.Unlock()

	for i := len(t.failures) - 1; i >= 0; i-- {
		if t.failures[i].ToolKey == toolKey && !t.failures[i].Recovered {
			t.failures[i].Recovered = true
			t.failures[i].RecoveredAt = recoveredAt
			return
		}
	}
}

// GetMTTR returns the mean time to recovery for a specific tool.
func (t *ToolMTTRTracker) GetMTTR(toolKey string) time.Duration {
	t.mu.RLock()
	defer t.mu.RUnlock()

	var totalDur time.Duration
	var count int
	for _, f := range t.failures {
		if f.ToolKey == toolKey && f.Recovered {
			totalDur += f.RecoveredAt.Sub(f.FailedAt)
			count++
		}
	}
	if count == 0 {
		return 0
	}
	return totalDur / time.Duration(count)
}

// GetAllMTTR returns MTTR for all tools that have recovery data.
func (t *ToolMTTRTracker) GetAllMTTR() map[string]time.Duration {
	t.mu.RLock()
	defer t.mu.RUnlock()

	totals := map[string]time.Duration{}
	counts := map[string]int{}
	for _, f := range t.failures {
		if f.Recovered {
			totals[f.ToolKey] += f.RecoveredAt.Sub(f.FailedAt)
			counts[f.ToolKey]++
		}
	}

	result := map[string]time.Duration{}
	for key, total := range totals {
		result[key] = total / time.Duration(counts[key])
	}
	return result
}

// ---------------------------------------------------------------------------
// FeatureAdoptionTracker
// ---------------------------------------------------------------------------

// FeatureAdoptionTracker tracks feature adoption across workspaces.
type FeatureAdoptionTracker struct {
	mu sync.RWMutex
	// featureWorkspaces: featureID -> set of workspaceIDs
	featureWorkspaces map[string]map[string]struct{}
	// totalWorkspaces is used to calculate adoption rate
	totalWorkspaces int
}

// NewFeatureAdoptionTracker creates a new adoption tracker.
func NewFeatureAdoptionTracker(totalWorkspaces int) *FeatureAdoptionTracker {
	if totalWorkspaces <= 0 {
		totalWorkspaces = 1
	}
	return &FeatureAdoptionTracker{
		featureWorkspaces: map[string]map[string]struct{}{},
		totalWorkspaces:   totalWorkspaces,
	}
}

// RecordAdoption records that a workspace has adopted a feature.
func (f *FeatureAdoptionTracker) RecordAdoption(workspaceID, featureID string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.featureWorkspaces[featureID] == nil {
		f.featureWorkspaces[featureID] = map[string]struct{}{}
	}
	f.featureWorkspaces[featureID][workspaceID] = struct{}{}
}

// GetAdoptionRate returns the percentage of workspaces that adopted a feature.
func (f *FeatureAdoptionTracker) GetAdoptionRate(featureID string) float64 {
	f.mu.RLock()
	defer f.mu.RUnlock()
	adopters := len(f.featureWorkspaces[featureID])
	return float64(adopters) / float64(f.totalWorkspaces) * 100
}

// GetAdoptionHeatmap returns the number of adopters per feature.
func (f *FeatureAdoptionTracker) GetAdoptionHeatmap() map[string]int {
	f.mu.RLock()
	defer f.mu.RUnlock()
	heatmap := map[string]int{}
	for feature, workspaces := range f.featureWorkspaces {
		heatmap[feature] = len(workspaces)
	}
	return heatmap
}

// ---------------------------------------------------------------------------
// AgentActionReplay
// ---------------------------------------------------------------------------

// ActionRecord represents a recorded agent action for replay.
type ActionRecord struct {
	ID          string         `json:"id"`
	WorkspaceID string         `json:"workspace_id"`
	UserID      string         `json:"user_id"`
	Action      string         `json:"action"`
	ToolKey     string         `json:"tool_key"`
	Input       map[string]any `json:"input"`
	Output      map[string]any `json:"output"`
	Timestamp   time.Time      `json:"timestamp"`
	Duration    time.Duration  `json:"duration"`
}

// AgentActionReplay provides action recording and replay capabilities.
type AgentActionReplay struct {
	mu      sync.RWMutex
	actions []ActionRecord
	index   map[string]int // action ID -> index
}

// NewAgentActionReplay creates a new action replay service.
func NewAgentActionReplay() *AgentActionReplay {
	return &AgentActionReplay{
		actions: []ActionRecord{},
		index:   map[string]int{},
	}
}

// RecordAction records an agent action.
func (r *AgentActionReplay) RecordAction(workspaceID string, action ActionRecord) {
	r.mu.Lock()
	defer r.mu.Unlock()
	action.WorkspaceID = workspaceID
	if action.ID == "" {
		action.ID = fmt.Sprintf("action_%06d", len(r.actions)+1)
	}
	r.index[action.ID] = len(r.actions)
	r.actions = append(r.actions, action)
}

// ReplayActions returns all actions for a workspace within a time range.
func (r *AgentActionReplay) ReplayActions(workspaceID string, from, to time.Time) []ActionRecord {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []ActionRecord
	for _, a := range r.actions {
		if a.WorkspaceID != workspaceID {
			continue
		}
		if a.Timestamp.Before(from) || a.Timestamp.After(to) {
			continue
		}
		result = append(result, a)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Timestamp.Before(result[j].Timestamp)
	})
	return result
}

// GetActionByID retrieves a specific action by ID.
func (r *AgentActionReplay) GetActionByID(actionID string) (*ActionRecord, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	idx, ok := r.index[actionID]
	if !ok {
		return nil, fmt.Errorf("action %q not found", actionID)
	}
	action := r.actions[idx]
	return &action, nil
}
