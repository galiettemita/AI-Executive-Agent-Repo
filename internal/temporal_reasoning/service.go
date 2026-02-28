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

func (s *Service) DefaultConfig(workspaceID string) Config {
	return Config{
		WorkspaceID:               workspaceID,
		DefaultTimezone:           "UTC",
		MaxHorizonDays:            365,
		ConflictPriorityThreshold: 80,
		TravelSpeedKPH:            40,
	}
}

func (s *Service) UpsertConfig(workspaceID string, cfg Config) Config {
	s.mu.Lock()
	defer s.mu.Unlock()

	if workspaceID == "" {
		workspaceID = "default"
	}
	defaults := s.DefaultConfig(workspaceID)
	cfg.WorkspaceID = workspaceID
	if cfg.DefaultTimezone == "" {
		cfg.DefaultTimezone = defaults.DefaultTimezone
	}
	if cfg.MaxHorizonDays == 0 {
		cfg.MaxHorizonDays = defaults.MaxHorizonDays
	}
	if cfg.ConflictPriorityThreshold == 0 {
		cfg.ConflictPriorityThreshold = defaults.ConflictPriorityThreshold
	}
	if cfg.TravelSpeedKPH == 0 {
		cfg.TravelSpeedKPH = defaults.TravelSpeedKPH
	}

	s.configs[workspaceID] = cfg
	return cfg
}

func (s *Service) GetConfig(workspaceID string) (Config, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	cfg, ok := s.configs[workspaceID]
	return cfg, ok
}

func (s *Service) UpsertConstraint(workspaceID string, constraint Constraint) Constraint {
	s.mu.Lock()
	defer s.mu.Unlock()

	if workspaceID == "" {
		workspaceID = "default"
	}
	if constraint.ID == "" {
		constraint.ID = fmt.Sprintf("constraint_%06d", len(s.constraints[workspaceID])+1)
	}
	if constraint.Status == "" {
		constraint.Status = "active"
	}
	if constraint.Priority == 0 {
		constraint.Priority = 50
	}
	constraint.WorkspaceID = workspaceID

	if _, ok := s.constraints[workspaceID]; !ok {
		s.constraints[workspaceID] = map[string]Constraint{}
	}
	s.constraints[workspaceID][constraint.ID] = constraint
	return constraint
}

func (s *Service) ListConstraints(workspaceID string) []Constraint {
	s.mu.RLock()
	defer s.mu.RUnlock()

	source := s.constraints[workspaceID]
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
	if _, ok := s.constraints[workspaceID]; !ok {
		return false
	}
	if _, ok := s.constraints[workspaceID][id]; !ok {
		return false
	}
	delete(s.constraints[workspaceID], id)
	return true
}

func (s *Service) ResolveExpression(workspaceID, expression, referenceDate, timezone string) Resolution {
	if workspaceID == "" {
		workspaceID = "default"
	}
	if referenceDate == "" {
		referenceDate = "2026-01-01"
	}

	resolvedDate := referenceDate
	confidence := 0.50
	lower := strings.TrimSpace(strings.ToLower(expression))

	switch {
	case strings.Contains(lower, "tomorrow"):
		resolvedDate = plusDays(referenceDate, 1)
		confidence = 0.95
	case strings.Contains(lower, "next week"):
		resolvedDate = plusDays(referenceDate, 7)
		confidence = 0.90
	case strings.HasPrefix(lower, "next "):
		if resolved, ok := resolveNextWeekday(referenceDate, strings.TrimPrefix(lower, "next ")); ok {
			resolvedDate = resolved
			confidence = 0.88
		}
	case strings.HasPrefix(lower, "in ") && strings.HasSuffix(lower, " weeks"):
		countRaw := strings.TrimSuffix(strings.TrimPrefix(lower, "in "), " weeks")
		count, err := strconv.Atoi(strings.TrimSpace(countRaw))
		if err == nil && count >= 0 {
			resolvedDate = plusDays(referenceDate, count*7)
			confidence = 0.87
		}
	case strings.HasPrefix(lower, "in ") && strings.HasSuffix(lower, " days"):
		countRaw := strings.TrimSuffix(strings.TrimPrefix(lower, "in "), " days")
		count, err := strconv.Atoi(strings.TrimSpace(countRaw))
		if err == nil && count >= 0 {
			resolvedDate = plusDays(referenceDate, count)
			confidence = 0.85
		}
	}

	if timezone == "" {
		if cfg, ok := s.GetConfig(workspaceID); ok {
			timezone = cfg.DefaultTimezone
		} else {
			timezone = "UTC"
		}
	}
	if cfg, ok := s.GetConfig(workspaceID); ok {
		if horizonExceeded(referenceDate, resolvedDate, cfg.MaxHorizonDays) {
			confidence = 0.10
		}
	}

	return Resolution{
		WorkspaceID:   workspaceID,
		Expression:    expression,
		ReferenceDate: referenceDate,
		ResolvedDate:  resolvedDate,
		Timezone:      timezone,
		Confidence:    confidence,
	}
}

func (s *Service) DetectConflicts(workspaceID, proposedStart, proposedEnd string) []Conflict {
	s.mu.RLock()
	defer s.mu.RUnlock()

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
		if overlaps(proposedStart, proposedEnd, constraint.StartsAt, constraint.EndsAt) {
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
	return out
}

func (s *Service) BuildConflictReport(workspaceID, proposedStart, proposedEnd string) ConflictReport {
	conflicts := s.DetectConflicts(workspaceID, proposedStart, proposedEnd)
	report := ConflictReport{
		HasConflict:    len(conflicts) > 0,
		ResolutionHint: "No temporal conflicts detected",
		Conflicts:      conflicts,
	}
	if report.HasConflict {
		report.ResolutionHint = "Shift schedule window or request manual override for high-priority constraints"
	}
	return report
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

	cacheKey := fmt.Sprintf("%s|%s|%s|%.3f", workspaceID, strings.ToLower(origin), strings.ToLower(destination), distanceKM)
	s.mu.Lock()
	s.travelEstimate[cacheKey] = minutes
	s.mu.Unlock()
	return minutes
}

func (s *Service) LookupTravelMinutes(workspaceID, origin, destination string, distanceKM float64) (int, bool) {
	cacheKey := fmt.Sprintf("%s|%s|%s|%.3f", workspaceID, strings.ToLower(origin), strings.ToLower(destination), distanceKM)
	s.mu.RLock()
	defer s.mu.RUnlock()
	minutes, ok := s.travelEstimate[cacheKey]
	return minutes, ok
}

func plusDays(referenceDate string, days int) string {
	const layout = "2006-01-02"
	parsed, err := time.Parse(layout, referenceDate)
	if err != nil {
		return referenceDate
	}
	return parsed.Add(time.Duration(days) * 24 * time.Hour).Format(layout)
}

func overlaps(startA, endA, startB, endB string) bool {
	if startA == "" || endA == "" || startB == "" || endB == "" {
		return false
	}
	return startA < endB && endA > startB
}

func resolveNextWeekday(referenceDate, weekdayRaw string) (string, bool) {
	const layout = "2006-01-02"
	parsed, err := time.Parse(layout, referenceDate)
	if err != nil {
		return "", false
	}
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
		return "", false
	}
	offset := int(target-parsed.Weekday()) % 7
	if offset <= 0 {
		offset += 7
	}
	return parsed.Add(time.Duration(offset) * 24 * time.Hour).Format(layout), true
}

func horizonExceeded(referenceDate, resolvedDate string, maxHorizonDays int) bool {
	if maxHorizonDays <= 0 {
		return false
	}
	const layout = "2006-01-02"
	ref, errRef := time.Parse(layout, referenceDate)
	resolved, errResolved := time.Parse(layout, resolvedDate)
	if errRef != nil || errResolved != nil {
		return false
	}
	delta := resolved.Sub(ref).Hours() / 24
	return delta > float64(maxHorizonDays)
}
