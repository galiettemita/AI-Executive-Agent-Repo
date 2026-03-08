package learning

import (
	"context"
	"fmt"
	"time"

	"github.com/brevio/brevio/internal/database"
	"github.com/jackc/pgx/v5"
)

// PgLessonRepository implements LessonRepository using PostgreSQL.
type PgLessonRepository struct {
	db database.Querier
}

// NewPgLessonRepository creates a new PgLessonRepository.
func NewPgLessonRepository(db database.Querier) *PgLessonRepository {
	return &PgLessonRepository{db: db}
}

func (r *PgLessonRepository) StoreFeedback(ctx context.Context, fb *Feedback) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO learning_feedbacks (id, workspace_id, feedback_type, content, created_at)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (id) DO NOTHING`,
		fb.ID, fb.WorkspaceID, fb.FeedbackType, fb.Content, fb.CreatedAt)
	if err != nil {
		return fmt.Errorf("store feedback: %w", err)
	}
	return nil
}

func (r *PgLessonRepository) StoreLesson(ctx context.Context, lesson *Lesson) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO learning_lessons (id, workspace_id, title, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (id) DO UPDATE SET
			title = EXCLUDED.title,
			status = EXCLUDED.status,
			updated_at = EXCLUDED.updated_at`,
		lesson.ID, lesson.WorkspaceID, lesson.Title, lesson.Status, lesson.CreatedAt, lesson.UpdatedAt)
	if err != nil {
		return fmt.Errorf("store lesson: %w", err)
	}
	return nil
}

func (r *PgLessonRepository) GetLesson(ctx context.Context, id string) (*Lesson, error) {
	row := r.db.QueryRow(ctx, `
		SELECT id, workspace_id, title, status, created_at, updated_at
		FROM learning_lessons WHERE id = $1`, id)

	lesson := &Lesson{}
	err := row.Scan(&lesson.ID, &lesson.WorkspaceID, &lesson.Title, &lesson.Status,
		&lesson.CreatedAt, &lesson.UpdatedAt)
	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("lesson not found: %s", id)
	}
	if err != nil {
		return nil, fmt.Errorf("get lesson: %w", err)
	}
	return lesson, nil
}

func (r *PgLessonRepository) ListLessons(ctx context.Context, workspaceID string) ([]Lesson, error) {
	var rows pgx.Rows
	var err error
	if workspaceID != "" {
		rows, err = r.db.Query(ctx, `
			SELECT id, workspace_id, title, status, created_at, updated_at
			FROM learning_lessons WHERE workspace_id = $1
			ORDER BY id ASC`, workspaceID)
	} else {
		rows, err = r.db.Query(ctx, `
			SELECT id, workspace_id, title, status, created_at, updated_at
			FROM learning_lessons
			ORDER BY id ASC`)
	}
	if err != nil {
		return nil, fmt.Errorf("list lessons: %w", err)
	}
	defer rows.Close()

	var lessons []Lesson
	for rows.Next() {
		var lesson Lesson
		err := rows.Scan(&lesson.ID, &lesson.WorkspaceID, &lesson.Title, &lesson.Status,
			&lesson.CreatedAt, &lesson.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("scan lesson: %w", err)
		}
		lessons = append(lessons, lesson)
	}
	return lessons, rows.Err()
}

func (r *PgLessonRepository) UpdateLessonStatus(ctx context.Context, id string, status string) error {
	tag, err := r.db.Exec(ctx, `
		UPDATE learning_lessons SET status = $1, updated_at = $2 WHERE id = $3`,
		status, time.Now().UTC(), id)
	if err != nil {
		return fmt.Errorf("update lesson status: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("lesson not found: %s", id)
	}
	return nil
}

func (r *PgLessonRepository) BulkRetire(ctx context.Context, workspaceID string) (int, error) {
	var tag interface{ RowsAffected() int64 }
	var err error
	if workspaceID != "" {
		tag, err = r.db.Exec(ctx, `
			UPDATE learning_lessons SET status = 'retired', updated_at = $1
			WHERE workspace_id = $2 AND status != 'retired'`,
			time.Now().UTC(), workspaceID)
	} else {
		tag, err = r.db.Exec(ctx, `
			UPDATE learning_lessons SET status = 'retired', updated_at = $1
			WHERE status != 'retired'`,
			time.Now().UTC())
	}
	if err != nil {
		return 0, fmt.Errorf("bulk retire lessons: %w", err)
	}
	return int(tag.RowsAffected()), nil
}

func (r *PgLessonRepository) ActiveLessonCount(ctx context.Context, workspaceID string) (int, error) {
	row := r.db.QueryRow(ctx, `
		SELECT COUNT(*) FROM learning_lessons
		WHERE workspace_id = $1 AND status IN ('proposed', 'confirmed')`, workspaceID)

	var count int
	if err := row.Scan(&count); err != nil {
		return 0, fmt.Errorf("count active lessons: %w", err)
	}
	return count, nil
}

// Compile-time interface compliance check.
var _ LessonRepository = (*PgLessonRepository)(nil)

// PgConflictRepository implements ConflictRepository using PostgreSQL.
type PgConflictRepository struct {
	db database.Querier
}

// NewPgConflictRepository creates a new PgConflictRepository.
func NewPgConflictRepository(db database.Querier) *PgConflictRepository {
	return &PgConflictRepository{db: db}
}

func (r *PgConflictRepository) StoreConflict(ctx context.Context, conflict *LessonConflict) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO learning_conflicts (id, lesson_a, lesson_b, conflict_type, confidence, description, resolved, resolution)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (id) DO UPDATE SET
			resolved = EXCLUDED.resolved,
			resolution = EXCLUDED.resolution`,
		conflict.ID, conflict.LessonA, conflict.LessonB, conflict.ConflictType,
		conflict.Confidence, conflict.Description, conflict.Resolved, conflict.Resolution)
	if err != nil {
		return fmt.Errorf("store conflict: %w", err)
	}
	return nil
}

func (r *PgConflictRepository) GetConflict(ctx context.Context, id string) (*LessonConflict, error) {
	row := r.db.QueryRow(ctx, `
		SELECT id, lesson_a, lesson_b, conflict_type, confidence, description, resolved, resolution
		FROM learning_conflicts WHERE id = $1`, id)

	conflict := &LessonConflict{}
	err := row.Scan(&conflict.ID, &conflict.LessonA, &conflict.LessonB, &conflict.ConflictType,
		&conflict.Confidence, &conflict.Description, &conflict.Resolved, &conflict.Resolution)
	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("conflict not found: %s", id)
	}
	if err != nil {
		return nil, fmt.Errorf("get conflict: %w", err)
	}
	return conflict, nil
}

func (r *PgConflictRepository) ListUnresolved(ctx context.Context) ([]LessonConflict, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, lesson_a, lesson_b, conflict_type, confidence, description, resolved, resolution
		FROM learning_conflicts
		WHERE resolved = false
		ORDER BY id ASC`)
	if err != nil {
		return nil, fmt.Errorf("list unresolved conflicts: %w", err)
	}
	defer rows.Close()

	var conflicts []LessonConflict
	for rows.Next() {
		var c LessonConflict
		err := rows.Scan(&c.ID, &c.LessonA, &c.LessonB, &c.ConflictType,
			&c.Confidence, &c.Description, &c.Resolved, &c.Resolution)
		if err != nil {
			return nil, fmt.Errorf("scan conflict: %w", err)
		}
		conflicts = append(conflicts, c)
	}
	return conflicts, rows.Err()
}

func (r *PgConflictRepository) ResolveConflict(ctx context.Context, id string, resolution string) error {
	tag, err := r.db.Exec(ctx, `
		UPDATE learning_conflicts SET resolved = true, resolution = $1 WHERE id = $2`,
		resolution, id)
	if err != nil {
		return fmt.Errorf("resolve conflict: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("conflict not found: %s", id)
	}
	return nil
}

// Compile-time interface compliance check.
var _ ConflictRepository = (*PgConflictRepository)(nil)
