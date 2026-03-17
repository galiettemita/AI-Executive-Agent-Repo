package goals

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

type Goal struct {
	ID          string     `json:"id"`
	WorkspaceID string     `json:"workspace_id"`
	Title       string     `json:"title"`
	Status      string     `json:"status"`
	Priority    string     `json:"priority"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
}

type Milestone struct {
	ID        string    `json:"id"`
	GoalID    string    `json:"goal_id"`
	Title     string    `json:"title"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	DependsOn []string  `json:"depends_on,omitempty"`
	Horizon   string    `json:"horizon,omitempty"` // daily | weekly | monthly | quarterly
	Order     int       `json:"order"`
}

type ProgressLog struct {
	ID        string    `json:"id"`
	GoalID    string    `json:"goal_id"`
	Summary   string    `json:"summary"`
	CreatedAt time.Time `json:"created_at"`
}

type MissionControlConfig struct {
	WorkspaceID           string `json:"workspace_id"`
	RefreshCadenceMinutes int    `json:"refresh_cadence_minutes"`
}

type MissionControlWidget struct {
	WidgetKey string `json:"widget_key"`
	Enabled   bool   `json:"enabled"`
	Position  int    `json:"position"`
}

type Service struct {
	mu                 sync.RWMutex
	nextID             int
	goals              map[string]Goal
	milestones         map[string][]Milestone
	progress           map[string][]ProgressLog
	mcConfig           map[string]MissionControlConfig
	mcWidgets          map[string][]MissionControlWidget
	dailyGoalCreateCnt map[string]int
}

func NewService() *Service {
	return &Service{
		nextID:             1,
		goals:              map[string]Goal{},
		milestones:         map[string][]Milestone{},
		progress:           map[string][]ProgressLog{},
		mcConfig:           map[string]MissionControlConfig{},
		mcWidgets:          map[string][]MissionControlWidget{},
		dailyGoalCreateCnt: map[string]int{},
	}
}

func goalCreateCountKey(workspaceID string, now time.Time) string {
	if workspaceID == "" {
		workspaceID = "default"
	}
	return workspaceID + "::" + now.UTC().Format("2006-01-02")
}

func (s *Service) CreateGoal(goal Goal, now time.Time) (Goal, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if now.IsZero() {
		now = time.Now().UTC()
	}
	if goal.WorkspaceID == "" {
		goal.WorkspaceID = "default"
	}
	key := goalCreateCountKey(goal.WorkspaceID, now)
	if s.dailyGoalCreateCnt[key] >= 20 {
		return Goal{}, fmt.Errorf("goal rate limit reached")
	}
	s.dailyGoalCreateCnt[key]++
	return s.upsertGoalLocked(goal, now), nil
}

func (s *Service) UpsertGoal(goal Goal) Goal {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.upsertGoalLocked(goal, time.Now().UTC())
}

func (s *Service) upsertGoalLocked(goal Goal, now time.Time) Goal {
	if goal.ID == "" {
		goal.ID = fmt.Sprintf("goal_%06d", s.nextID)
		s.nextID++
	}
	if goal.WorkspaceID == "" {
		goal.WorkspaceID = "default"
	}
	if goal.Status == "" {
		goal.Status = "active"
	}
	if goal.Priority == "" {
		goal.Priority = "medium"
	}
	existing, hasExisting := s.goals[goal.ID]
	if hasExisting {
		goal.CreatedAt = existing.CreatedAt
	} else if goal.CreatedAt.IsZero() {
		goal.CreatedAt = now
	}
	goal.UpdatedAt = now
	if goal.Status == "completed" && goal.CompletedAt == nil {
		completedAt := now
		goal.CompletedAt = &completedAt
	}
	s.goals[goal.ID] = goal
	return goal
}

func (s *Service) ListGoals(workspaceID string) []Goal {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Goal, 0, len(s.goals))
	for _, goal := range s.goals {
		if workspaceID != "" && goal.WorkspaceID != workspaceID {
			continue
		}
		out = append(out, goal)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].ID < out[j].ID
	})
	return out
}

func (s *Service) GetGoal(id string) (Goal, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	goal, ok := s.goals[id]
	return goal, ok
}

func (s *Service) DeleteGoal(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.goals[id]; !ok {
		return false
	}
	delete(s.goals, id)
	delete(s.milestones, id)
	delete(s.progress, id)
	return true
}

func (s *Service) AddMilestone(goalID string, milestone Milestone) Milestone {
	s.mu.Lock()
	defer s.mu.Unlock()
	milestone.ID = fmt.Sprintf("milestone_%06d", s.nextID)
	s.nextID++
	milestone.GoalID = goalID
	if milestone.Status == "" {
		milestone.Status = "pending"
	}
	if milestone.CreatedAt.IsZero() {
		milestone.CreatedAt = time.Now().UTC()
	}
	s.milestones[goalID] = append(s.milestones[goalID], milestone)
	return milestone
}

func (s *Service) ListMilestones(goalID string) []Milestone {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Milestone, len(s.milestones[goalID]))
	copy(out, s.milestones[goalID])
	return out
}

func (s *Service) AddProgress(goalID string, progress ProgressLog) ProgressLog {
	s.mu.Lock()
	defer s.mu.Unlock()
	progress.ID = fmt.Sprintf("goal_progress_%06d", s.nextID)
	s.nextID++
	progress.GoalID = goalID
	if progress.CreatedAt.IsZero() {
		progress.CreatedAt = time.Now().UTC()
	}
	s.progress[goalID] = append(s.progress[goalID], progress)
	return progress
}

func (s *Service) ListProgress(goalID string) []ProgressLog {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]ProgressLog, len(s.progress[goalID]))
	copy(out, s.progress[goalID])
	return out
}

func (s *Service) ReviewGoals(workspaceID string, now time.Time, stalledAfter time.Duration) []Goal {
	s.mu.Lock()
	defer s.mu.Unlock()
	if now.IsZero() {
		now = time.Now().UTC()
	}
	stalled := []Goal{}
	for goalID, goal := range s.goals {
		if goal.WorkspaceID != workspaceID {
			continue
		}
		if goal.Status != "active" {
			continue
		}
		lastActivity := goal.UpdatedAt
		for _, progress := range s.progress[goalID] {
			if progress.CreatedAt.After(lastActivity) {
				lastActivity = progress.CreatedAt
			}
		}
		if now.Sub(lastActivity) >= stalledAfter {
			goal.Status = "stalled"
			goal.UpdatedAt = now
			s.goals[goalID] = goal
			stalled = append(stalled, goal)
		}
	}
	sort.Slice(stalled, func(i, j int) bool { return stalled[i].ID < stalled[j].ID })
	return stalled
}

func (s *Service) GetMissionControlConfig(workspaceID string) (MissionControlConfig, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	cfg, ok := s.mcConfig[workspaceID]
	return cfg, ok
}

func (s *Service) UpsertMissionControlConfig(workspaceID string, cfg MissionControlConfig) MissionControlConfig {
	s.mu.Lock()
	defer s.mu.Unlock()
	if workspaceID == "" {
		workspaceID = "default"
	}
	cfg.WorkspaceID = workspaceID
	if cfg.RefreshCadenceMinutes == 0 {
		cfg.RefreshCadenceMinutes = 30
	}
	s.mcConfig[workspaceID] = cfg
	return cfg
}

func (s *Service) GetMissionControlWidgets(workspaceID string) []MissionControlWidget {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]MissionControlWidget, len(s.mcWidgets[workspaceID]))
	copy(out, s.mcWidgets[workspaceID])
	return out
}

func (s *Service) SetMissionControlWidgets(workspaceID string, widgets []MissionControlWidget) []MissionControlWidget {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]MissionControlWidget, len(widgets))
	copy(out, widgets)
	sort.Slice(out, func(i, j int) bool {
		return out[i].Position < out[j].Position
	})
	s.mcWidgets[workspaceID] = out
	return out
}

// getNextMilestoneLocked returns the next executable milestone for goalID.
// A milestone is eligible when Status == "todo" and every ID in its DependsOn
// slice has Status == "completed". Among eligible milestones the one with the
// lowest Order value is returned.
//
// Returns nil, nil when no eligible milestone exists.
//
// PRECONDITION: caller must hold s.mu (at minimum RLock) before invoking.
// This method acquires no locks.
func (s *Service) getNextMilestoneLocked(goalID string) (*Milestone, error) {
	ms := s.milestones[goalID]

	completedSet := make(map[string]bool, len(ms))
	for _, m := range ms {
		if m.Status == "completed" {
			completedSet[m.ID] = true
		}
	}

	eligible := make([]Milestone, 0, len(ms))
	for _, m := range ms {
		if m.Status != "todo" {
			continue
		}
		allDepsMet := true
		for _, dep := range m.DependsOn {
			if !completedSet[dep] {
				allDepsMet = false
				break
			}
		}
		if allDepsMet {
			eligible = append(eligible, m)
		}
	}

	if len(eligible) == 0 {
		return nil, nil
	}

	sort.Slice(eligible, func(i, j int) bool {
		return eligible[i].Order < eligible[j].Order
	})

	return &eligible[0], nil
}

// GetNextMilestone returns the next executable milestone for the given goalID.
// It acquires a read lock and delegates to getNextMilestoneLocked.
// Returns nil, nil if no eligible milestone exists.
func (s *Service) GetNextMilestone(goalID string) (*Milestone, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.getNextMilestoneLocked(goalID)
}

// BuildContextSummary returns a human-readable summary of all goals and their
// next executable milestone for the given workspaceID.
func (s *Service) BuildContextSummary(workspaceID string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var sb strings.Builder
	for _, g := range s.goals {
		if g.WorkspaceID == workspaceID {
			fmt.Fprintf(&sb, "Goal[%s]: %s (%s)\n", g.Priority, g.Title, g.Status)
			next, _ := s.getNextMilestoneLocked(g.ID)
			if next != nil {
				fmt.Fprintf(&sb, "  Next: %s (horizon:%s)\n", next.Title, next.Horizon)
			}
		}
	}
	return sb.String()
}

func (s *Service) MissionControlSnapshot(workspaceID string) map[string]any {
	goals := s.ListGoals(workspaceID)
	widgets := s.GetMissionControlWidgets(workspaceID)
	return map[string]any{
		"workspace_id": workspaceID,
		"goals_count":  len(goals),
		"widgets":      widgets,
	}
}
