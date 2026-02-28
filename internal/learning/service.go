package learning

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

type Config struct {
	WorkspaceID      string `json:"workspace_id"`
	MaxActiveLessons int    `json:"max_active_lessons"`
	AutoApplyLessons bool   `json:"auto_apply_lessons"`
}

type Feedback struct {
	ID           string    `json:"id"`
	WorkspaceID  string    `json:"workspace_id"`
	FeedbackType string    `json:"feedback_type"`
	Content      string    `json:"content"`
	CreatedAt    time.Time `json:"created_at"`
}

type Lesson struct {
	ID          string    `json:"id"`
	WorkspaceID string    `json:"workspace_id"`
	Title       string    `json:"title"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
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
	if cfg.MaxActiveLessons <= 0 {
		cfg.MaxActiveLessons = 20
	}
	s.configs[workspaceID] = cfg
	return cfg
}

func (s *Service) AddFeedback(feedback Feedback) Feedback {
	stored, _ := s.SubmitFeedback(feedback)
	return stored
}

func (s *Service) SubmitFeedback(feedback Feedback) (Feedback, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if feedback.WorkspaceID == "" {
		feedback.WorkspaceID = "default"
	}
	if strings.TrimSpace(feedback.Content) == "" {
		return Feedback{}, fmt.Errorf("feedback content is required")
	}

	cfg := s.configs[feedback.WorkspaceID]
	if cfg.WorkspaceID == "" {
		cfg = Config{WorkspaceID: feedback.WorkspaceID, MaxActiveLessons: 20}
		s.configs[feedback.WorkspaceID] = cfg
	}
	if s.activeLessonCountLocked(feedback.WorkspaceID) >= cfg.MaxActiveLessons {
		return Feedback{}, fmt.Errorf("LESSON_CAP_REACHED")
	}

	feedback.ID = fmt.Sprintf("feedback_%06d", s.nextID)
	s.nextID++
	if feedback.CreatedAt.IsZero() {
		feedback.CreatedAt = time.Now().UTC()
	}
	s.feedbacks = append(s.feedbacks, feedback)

	lesson := Lesson{
		ID:          fmt.Sprintf("lesson_%06d", s.nextID),
		WorkspaceID: feedback.WorkspaceID,
		Title:       lessonTitleFromFeedback(feedback.Content),
		Status:      "proposed",
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}
	s.nextID++
	s.lessons[lesson.ID] = lesson
	return feedback, nil
}

func lessonTitleFromFeedback(content string) string {
	trimmed := strings.TrimSpace(content)
	if len(trimmed) > 40 {
		trimmed = trimmed[:40]
	}
	if trimmed == "" {
		return "Lesson from feedback"
	}
	return "Lesson: " + trimmed
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

func (s *Service) activeLessonCountLocked(workspaceID string) int {
	count := 0
	for _, lesson := range s.lessons {
		if lesson.WorkspaceID != workspaceID {
			continue
		}
		if lesson.Status == "proposed" || lesson.Status == "confirmed" {
			count++
		}
	}
	return count
}

func (s *Service) ConfirmLesson(id string) (Lesson, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	lesson, ok := s.lessons[id]
	if !ok {
		return Lesson{}, false
	}
	lesson.Status = "confirmed"
	lesson.UpdatedAt = time.Now().UTC()
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
	lesson.UpdatedAt = time.Now().UTC()
	s.lessons[id] = lesson
	return lesson, true
}

func (s *Service) BulkRetire(workspaceID string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	count := 0
	for id, lesson := range s.lessons {
		if workspaceID != "" && lesson.WorkspaceID != workspaceID {
			continue
		}
		if lesson.Status == "retired" {
			continue
		}
		lesson.Status = "retired"
		lesson.UpdatedAt = time.Now().UTC()
		s.lessons[id] = lesson
		count++
	}
	return count
}
