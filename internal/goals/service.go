package goals

import (
	"fmt"
	"sort"
	"sync"
)

type Goal struct {
	ID          string `json:"id"`
	WorkspaceID string `json:"workspace_id"`
	Title       string `json:"title"`
	Status      string `json:"status"`
	Priority    string `json:"priority"`
}

type Milestone struct {
	ID     string `json:"id"`
	GoalID string `json:"goal_id"`
	Title  string `json:"title"`
	Status string `json:"status"`
}

type ProgressLog struct {
	ID      string `json:"id"`
	GoalID  string `json:"goal_id"`
	Summary string `json:"summary"`
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
	mu         sync.RWMutex
	nextID     int
	goals      map[string]Goal
	milestones map[string][]Milestone
	progress   map[string][]ProgressLog
	mcConfig   map[string]MissionControlConfig
	mcWidgets  map[string][]MissionControlWidget
}

func NewService() *Service {
	return &Service{
		nextID:     1,
		goals:      map[string]Goal{},
		milestones: map[string][]Milestone{},
		progress:   map[string][]ProgressLog{},
		mcConfig:   map[string]MissionControlConfig{},
		mcWidgets:  map[string][]MissionControlWidget{},
	}
}

func (s *Service) UpsertGoal(goal Goal) Goal {
	s.mu.Lock()
	defer s.mu.Unlock()
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

func (s *Service) MissionControlSnapshot(workspaceID string) map[string]any {
	goals := s.ListGoals(workspaceID)
	widgets := s.GetMissionControlWidgets(workspaceID)
	return map[string]any{
		"workspace_id": workspaceID,
		"goals_count":  len(goals),
		"widgets":      widgets,
	}
}
