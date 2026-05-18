package learning

import "context"

// LessonRepository persists lessons and feedback.
type LessonRepository interface {
	StoreFeedback(ctx context.Context, fb *Feedback) error
	StoreLesson(ctx context.Context, lesson *Lesson) error
	GetLesson(ctx context.Context, id string) (*Lesson, error)
	ListLessons(ctx context.Context, workspaceID string) ([]Lesson, error)
	UpdateLessonStatus(ctx context.Context, id string, status string) error
	BulkRetire(ctx context.Context, workspaceID string) (int, error)
	ActiveLessonCount(ctx context.Context, workspaceID string) (int, error)
}

// ConflictRepository persists lesson conflicts.
type ConflictRepository interface {
	StoreConflict(ctx context.Context, conflict *LessonConflict) error
	GetConflict(ctx context.Context, id string) (*LessonConflict, error)
	ListUnresolved(ctx context.Context) ([]LessonConflict, error)
	ResolveConflict(ctx context.Context, id string, resolution string) error
}
