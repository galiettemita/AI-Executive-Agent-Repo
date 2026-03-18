package learning

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// DBPreferenceTransferReader counts distinct workspaces that adopted a lesson
// via the preference transfer mechanism.
type DBPreferenceTransferReader struct {
	db *pgxpool.Pool
}

// NewDBPreferenceTransferReader creates a reader backed by the database.
func NewDBPreferenceTransferReader(db *pgxpool.Pool) *DBPreferenceTransferReader {
	return &DBPreferenceTransferReader{db: db}
}

// GetWorkspaceTransferCount returns the number of distinct workspaces
// that have used a lesson via preference transfer.
func (r *DBPreferenceTransferReader) GetWorkspaceTransferCount(ctx context.Context, lessonID uuid.UUID) (int, error) {
	if r.db == nil {
		return 0, nil
	}

	var count int
	err := r.db.QueryRow(ctx,
		`SELECT COUNT(DISTINCT workspace_id) FROM lesson_usages WHERE lesson_id=$1`,
		lessonID,
	).Scan(&count)
	if err != nil {
		return 0, err
	}
	return count, nil
}
