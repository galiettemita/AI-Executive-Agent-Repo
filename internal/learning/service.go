package learning

import (
	"fmt"
	"sort"
	"sync"
)

type Config struct {
	WorkspaceID      string `json:"workspace_id"`
	MaxActiveLessons int    `json:"max_active_lessons"`
	AutoApplyLessons bool   `json:"auto_apply_lessons"`
}

type Feedback struct {
	ID           string `json:"id"`
	WorkspaceID  string `json:"workspace_id"`
	FeedbackType string `json:"feedback_type"`
	Content      string `json:"content"`
}

type Lesson struct {
	ID          string `json:"id"`
	WorkspaceID string `json:"workspace_id"`
	Title       string `json:"title"`
	Status      string `json:"status"`
}

type Service struct {
	mu        sync.RWMutex
	nextID    int
	configs   map[string]Config
	feedbacks []Feedback
	lessons   map[string]Lesson
}

func NewService() *Service {
	return &Service{
		nextID:    1,
		configs:   map[string]Config{},
		feedbacks: []Feedback{},
		lessons:   map[string]Lesson{},
	}
}

func (s *Service) GetConfig(workspaceID string) (Config, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	cfg, ok := s.configs[workspaceID]
	return cfg, ok
}

func (s *Service) UpsertConfig(workspaceID string, cfg Config) Config {
	s.mu.Lock()
	defer s.mu.Unlock()
	if workspaceID == "" {
		workspaceID = "default"
	}
	cfg.WorkspaceID = workspaceID
	if cfg.MaxActiveLessons == 0 {
		cfg.MaxActiveLessons = 20
	}
	s.configs[workspaceID] = cfg
	return cfg
}

func (s *Service) AddFeedback(feedback Feedback) Feedback {
	s.mu.Lock()
	defer s.mu.Unlock()
	feedback.ID = fmt.Sprintf("feedback_%06d", s.nextID)
	s.nextID++
	if feedback.WorkspaceID == "" {
		feedback.WorkspaceID = "default"
	}
	s.feedbacks = append(s.feedbacks, feedback)

	lesson := Lesson{
		ID:          fmt.Sprintf("lesson_%06d", s.nextID),
		WorkspaceID: feedback.WorkspaceID,
		Title:       "Lesson from feedback",
		Status:      "proposed",
	}
	s.nextID++
	s.lessons[lesson.ID] = lesson
	return feedback
}

func (s *Service) ListLessons(workspaceID string) []Lesson {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Lesson, 0, len(s.lessons))
	for _, lesson := range s.lessons {
		if workspaceID != "" && lesson.WorkspaceID != workspaceID {
			continue
		}
		out = append(out, lesson)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].ID < out[j].ID
	})
	return out
}

func (s *Service) ConfirmLesson(id string) (Lesson, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	lesson, ok := s.lessons[id]
	if !ok {
		return Lesson{}, false
	}
	lesson.Status = "confirmed"
	s.lessons[id] = lesson
	return lesson, true
}

func (s *Service) RetireLesson(id string) (Lesson, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	lesson, ok := s.lessons[id]
	if !ok {
		return Lesson{}, false
	}
	lesson.Status = "retired"
	s.lessons[id] = lesson
	return lesson, true
}
