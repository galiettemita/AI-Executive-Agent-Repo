package temporal_reasoning

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Config struct {
	WorkspaceID               string `json:"workspace_id"`
	DefaultTimezone           string `json:"default_timezone"`
	MaxHorizonDays            int    `json:"max_horizon_days"`
	ConflictPriorityThreshold int    `json:"conflict_priority_threshold"`
	TravelSpeedKPH            int    `json:"travel_speed_kph"`
}

type Constraint struct {
	ID          string `json:"id"`
	WorkspaceID string `json:"workspace_id"`
	Subject     string `json:"subject"`
	StartsAt    string `json:"starts_at"`
	EndsAt      string `json:"ends_at"`
	Priority    int    `json:"priority"`
	Status      string `json:"status"`
}

type Resolution struct {
	WorkspaceID   string  `json:"workspace_id"`
	Expression    string  `json:"expression"`
	ReferenceDate string  `json:"reference_date"`
	ResolvedDate  string  `json:"resolved_date"`
	Timezone      string  `json:"timezone"`
	Confidence    float64 `json:"confidence"`
}

type Conflict struct {
	ConstraintID string `json:"constraint_id"`
	Title        string `json:"title"`
	StartTS      string `json:"start_ts"`
	EndTS        string `json:"end_ts"`
	Reason       string `json:"reason"`
	Priority     int    `json:"priority"`
}

type ConflictReport struct {
	HasConflict    bool       `json:"has_conflict"`
	ResolutionHint string     `json:"resolution_hint"`
	Conflicts      []Conflict `json:"conflicts"`
}

type Service struct {
	mu             sync.RWMutex
	configs        map[string]Config
	constraints    map[string]map[string]Constraint
	travelEstimate map[string]int
}

func NewService() *Service {
	return &Service{
		configs:        map[string]Config{},
		constraints:    map[string]map[string]Constraint{},
		travelEstimate: map[string]int{},
	}
}

func normalizeWorkspaceID(workspaceID string) string {
	if strings.TrimSpace(workspaceID) == "" {
		return "default"
	}
	return workspaceID
}

func (s *Service) DefaultConfig(workspaceID string) Config {
	return Config{
		WorkspaceID:               normalizeWorkspaceID(workspaceID),
		DefaultTimezone:           "UTC",
		MaxHorizonDays:            365,
		ConflictPriorityThreshold: 80,
		TravelSpeedKPH:            40,
	}
}

func resolveLocation(name string) (*time.Location, error) {
	if strings.TrimSpace(name) == "" {
		return time.UTC, nil
	}
	return time.LoadLocation(name)
}

func (s *Service) UpsertConfig(workspaceID string, cfg Config) Config {
	s.mu.Lock()
	defer s.mu.Unlock()

	workspaceID = normalizeWorkspaceID(workspaceID)
	defaults := s.DefaultConfig(workspaceID)
	cfg.WorkspaceID = workspaceID
	if cfg.DefaultTimezone == "" {
		cfg.DefaultTimezone = defaults.DefaultTimezone
	}
	if _, err := resolveLocation(cfg.DefaultTimezone); err != nil {
		cfg.DefaultTimezone = defaults.DefaultTimezone
	}
	if cfg.MaxHorizonDays <= 0 {
		cfg.MaxHorizonDays = defaults.MaxHorizonDays
	}
	if cfg.ConflictPriorityThreshold <= 0 {
		cfg.ConflictPriorityThreshold = defaults.ConflictPriorityThreshold
	}
	if cfg.TravelSpeedKPH <= 0 {
		cfg.TravelSpeedKPH = defaults.TravelSpeedKPH
	}

	s.configs[workspaceID] = cfg
	return cfg
}

func (s *Service) GetConfig(workspaceID string) (Config, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	cfg, ok := s.configs[normalizeWorkspaceID(workspaceID)]
	return cfg, ok
}

func parseTimestamp(value string) (time.Time, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return time.Time{}, fmt.Errorf("timestamp required")
	}
	if parsed, err := time.Parse(time.RFC3339, trimmed); err == nil {
		return parsed.UTC(), nil
	}
	if parsed, err := time.Parse(time.RFC3339Nano, trimmed); err == nil {
		return parsed.UTC(), nil
	}
	return time.Time{}, fmt.Errorf("invalid timestamp %q", value)
}

func validateConstraintTimes(constraint Constraint) error {
	if strings.TrimSpace(constraint.Subject) == "" {
		return fmt.Errorf("constraint subject required")
	}
	start, err := parseTimestamp(constraint.StartsAt)
	if err != nil {
		return fmt.Errorf("invalid starts_at: %w", err)
	}
	end, err := parseTimestamp(constraint.EndsAt)
	if err != nil {
		return fmt.Errorf("invalid ends_at: %w", err)
	}
	if !end.After(start) {
		return fmt.Errorf("ends_at must be after starts_at")
	}
	return nil
}

func (s *Service) UpsertConstraint(workspaceID string, constraint Constraint) (Constraint, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	workspaceID = normalizeWorkspaceID(workspaceID)
	if err := validateConstraintTimes(constraint); err != nil {
		return Constraint{}, err
	}
	if constraint.ID == "" {
		constraint.ID = fmt.Sprintf("constraint_%06d", len(s.constraints[workspaceID])+1)
	}
	if constraint.Status == "" {
		constraint.Status = "active"
	}
	if constraint.Priority <= 0 {
		constraint.Priority = 50
	}
	constraint.WorkspaceID = workspaceID

	if _, ok := s.constraints[workspaceID]; !ok {
		s.constraints[workspaceID] = map[string]Constraint{}
	}
	s.constraints[workspaceID][constraint.ID] = constraint
	return constraint, nil
}

func (s *Service) ListConstraints(workspaceID string) []Constraint {
	s.mu.RLock()
	defer s.mu.RUnlock()

	source := s.constraints[normalizeWorkspaceID(workspaceID)]
	out := make([]Constraint, 0, len(source))
	for _, constraint := range source {
		out = append(out, constraint)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].ID < out[j].ID
	})
	return out
}

func (s *Service) DeleteConstraint(workspaceID, id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	workspaceID = normalizeWorkspaceID(workspaceID)
	if _, ok := s.constraints[workspaceID]; !ok {
		return false
	}
	if _, ok := s.constraints[workspaceID][id]; !ok {
		return false
	}
	delete(s.constraints[workspaceID], id)
	return true
}

func (s *Service) timezoneForWorkspace(workspaceID, explicit string) (string, *time.Location, error) {
	if strings.TrimSpace(explicit) != "" {
		loc, err := resolveLocation(explicit)
		return explicit, loc, err
	}
	if cfg, ok := s.GetConfig(workspaceID); ok {
		loc, err := resolveLocation(cfg.DefaultTimezone)
		return cfg.DefaultTimezone, loc, err
	}
	loc, err := resolveLocation("UTC")
	return "UTC", loc, err
}

func parseReferenceDate(referenceDate string, loc *time.Location) (time.Time, error) {
	const layout = "2006-01-02"
	if strings.TrimSpace(referenceDate) == "" {
		now := time.Now().In(loc)
		return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc), nil
	}
	parsed, err := time.ParseInLocation(layout, referenceDate, loc)
	if err != nil {
		return time.Time{}, err
	}
	return parsed, nil
}

func (s *Service) ResolveExpression(workspaceID, expression, referenceDate, timezone string) (Resolution, error) {
	workspaceID = normalizeWorkspaceID(workspaceID)
	if strings.TrimSpace(expression) == "" {
		return Resolution{}, fmt.Errorf("expression required")
	}

	timezoneName, loc, err := s.timezoneForWorkspace(workspaceID, timezone)
	if err != nil {
		return Resolution{}, err
	}
	reference, err := parseReferenceDate(referenceDate, loc)
	if err != nil {
		return Resolution{}, fmt.Errorf("invalid reference_date: %w", err)
	}

	resolvedDate := reference
	confidence := 0.45
	lower := strings.TrimSpace(strings.ToLower(expression))

	switch {
	case strings.Contains(lower, "tomorrow"):
		resolvedDate = reference.AddDate(0, 0, 1)
		confidence = 0.95
	case strings.Contains(lower, "today"), strings.Contains(lower, "tonight"), strings.Contains(lower, "this evening"):
		resolvedDate = reference
		confidence = 0.82
	case strings.Contains(lower, "next week"):
		resolvedDate = reference.AddDate(0, 0, 7)
		confidence = 0.9
	case strings.HasPrefix(lower, "next "):
		if resolved, ok := resolveNextWeekday(reference, strings.TrimPrefix(lower, "next ")); ok {
			resolvedDate = resolved
			confidence = 0.88
		}
	case strings.HasPrefix(lower, "in ") && strings.HasSuffix(lower, " weeks"):
		countRaw := strings.TrimSuffix(strings.TrimPrefix(lower, "in "), " weeks")
		count, parseErr := strconv.Atoi(strings.TrimSpace(countRaw))
		if parseErr == nil && count >= 0 {
			resolvedDate = reference.AddDate(0, 0, count*7)
			confidence = 0.87
		}
	case strings.HasPrefix(lower, "in ") && strings.HasSuffix(lower, " days"):
		countRaw := strings.TrimSuffix(strings.TrimPrefix(lower, "in "), " days")
		count, parseErr := strconv.Atoi(strings.TrimSpace(countRaw))
		if parseErr == nil && count >= 0 {
			resolvedDate = reference.AddDate(0, 0, count)
			confidence = 0.85
		}
	}

	if cfg, ok := s.GetConfig(workspaceID); ok {
		if horizonExceeded(reference, resolvedDate, cfg.MaxHorizonDays) {
			confidence = 0.1
		}
	}

	return Resolution{
		WorkspaceID:   workspaceID,
		Expression:    expression,
		ReferenceDate: reference.Format("2006-01-02"),
		ResolvedDate:  resolvedDate.Format("2006-01-02"),
		Timezone:      timezoneName,
		Confidence:    confidence,
	}, nil
}

func overlaps(startA, endA, startB, endB time.Time) bool {
	return startA.Before(endB) && endA.After(startB)
}

func (s *Service) DetectConflicts(workspaceID, proposedStart, proposedEnd string) ([]Conflict, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	workspaceID = normalizeWorkspaceID(workspaceID)
	start, err := parseTimestamp(proposedStart)
	if err != nil {
		return nil, fmt.Errorf("invalid proposed_start: %w", err)
	}
	end, err := parseTimestamp(proposedEnd)
	if err != nil {
		return nil, fmt.Errorf("invalid proposed_end: %w", err)
	}
	if !end.After(start) {
		return nil, fmt.Errorf("proposed_end must be after proposed_start")
	}

	cfg, ok := s.configs[workspaceID]
	threshold := 80
	if ok && cfg.ConflictPriorityThreshold > 0 {
		threshold = cfg.ConflictPriorityThreshold
	}

	out := make([]Conflict, 0)
	for _, constraint := range s.constraints[workspaceID] {
		if constraint.Status != "active" {
			continue
		}
		if constraint.Priority < threshold {
			continue
		}
		constraintStart, startErr := parseTimestamp(constraint.StartsAt)
		constraintEnd, endErr := parseTimestamp(constraint.EndsAt)
		if startErr != nil || endErr != nil {
			continue
		}
		if overlaps(start, end, constraintStart, constraintEnd) {
			out = append(out, Conflict{
				ConstraintID: constraint.ID,
				Title:        constraint.Subject,
				StartTS:      constraint.StartsAt,
				EndTS:        constraint.EndsAt,
				Reason:       "TEMPORAL_CONSTRAINT_VIOLATION",
				Priority:     constraint.Priority,
			})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].ConstraintID < out[j].ConstraintID
	})
	return out, nil
}

func (s *Service) BuildConflictReport(workspaceID, proposedStart, proposedEnd string) (ConflictReport, error) {
	conflicts, err := s.DetectConflicts(workspaceID, proposedStart, proposedEnd)
	if err != nil {
		return ConflictReport{}, err
	}
	report := ConflictReport{
		HasConflict:    len(conflicts) > 0,
		ResolutionHint: "No temporal conflicts detected",
		Conflicts:      conflicts,
	}
	if report.HasConflict {
		report.ResolutionHint = "Shift schedule window or request manual override for high-priority constraints"
	}
	return report, nil
}

func (s *Service) EstimateTravelMinutes(workspaceID, origin, destination string, distanceKM float64) int {
	if distanceKM <= 0 {
		return 0
	}

	speed := 40
	if cfg, ok := s.GetConfig(workspaceID); ok && cfg.TravelSpeedKPH > 0 {
		speed = cfg.TravelSpeedKPH
	}
	minutes := int(math.Ceil((distanceKM / float64(speed)) * 60))
	if minutes < 1 {
		minutes = 1
	}

	cacheKey := fmt.Sprintf("%s|%s|%s|%.3f", normalizeWorkspaceID(workspaceID), strings.ToLower(origin), strings.ToLower(destination), distanceKM)
	s.mu.Lock()
	s.travelEstimate[cacheKey] = minutes
	s.mu.Unlock()
	return minutes
}

func (s *Service) LookupTravelMinutes(workspaceID, origin, destination string, distanceKM float64) (int, bool) {
	cacheKey := fmt.Sprintf("%s|%s|%s|%.3f", normalizeWorkspaceID(workspaceID), strings.ToLower(origin), strings.ToLower(destination), distanceKM)
	s.mu.RLock()
	defer s.mu.RUnlock()
	minutes, ok := s.travelEstimate[cacheKey]
	return minutes, ok
}

func resolveNextWeekday(reference time.Time, weekdayRaw string) (time.Time, bool) {
	targetWeekday := strings.ToLower(strings.TrimSpace(weekdayRaw))
	weekdayIndex := map[string]time.Weekday{
		"sunday":    time.Sunday,
		"monday":    time.Monday,
		"tuesday":   time.Tuesday,
		"wednesday": time.Wednesday,
		"thursday":  time.Thursday,
		"friday":    time.Friday,
		"saturday":  time.Saturday,
	}
	target, ok := weekdayIndex[targetWeekday]
	if !ok {
		return time.Time{}, false
	}
	offset := int(target-reference.Weekday()) % 7
	if offset <= 0 {
		offset += 7
	}
	return reference.AddDate(0, 0, offset), true
}

func horizonExceeded(referenceDate, resolvedDate time.Time, maxHorizonDays int) bool {
	if maxHorizonDays <= 0 {
		return false
	}
	delta := resolvedDate.Sub(referenceDate).Hours() / 24
	return delta > float64(maxHorizonDays)
}
