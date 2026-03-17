package a2a

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// TaskStore persists A2A tasks. Uses Postgres when pool is set, in-memory otherwise.
type TaskStore struct {
	pool *pgxpool.Pool
	mu   sync.Mutex
	mem  map[string]*Task // in-memory fallback
}

func NewTaskStore(pool *pgxpool.Pool) *TaskStore {
	return &TaskStore{pool: pool, mem: make(map[string]*Task)}
}

// Create inserts a new task in 'submitted' status.
func (s *TaskStore) Create(ctx context.Context, t Task) (*Task, error) {
	now := time.Now().UTC()
	t.CreatedAt = now
	t.UpdatedAt = now
	t.Status = TaskStatusSubmitted

	if s.pool == nil {
		s.mu.Lock()
		defer s.mu.Unlock()
		copy := t
		s.mem[t.ID] = &copy
		return &copy, nil
	}

	inputJSON, err := json.Marshal(t.InputPayload)
	if err != nil {
		return nil, fmt.Errorf("task_store: marshal input: %w", err)
	}
	_, err = s.pool.Exec(ctx, `
		INSERT INTO a2a_tasks
			(id, workspace_id, requesting_agent_id, capability, input_payload, status, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5::jsonb,$6,$7,$8)
	`, t.ID, t.WorkspaceID, t.RequestingAgentID, t.Capability,
		string(inputJSON), string(t.Status), now, now)
	if err != nil {
		return nil, fmt.Errorf("task_store: insert: %w", err)
	}
	return &t, nil
}

// Get retrieves a task by ID.
func (s *TaskStore) Get(ctx context.Context, taskID string) (*Task, error) {
	if s.pool == nil {
		s.mu.Lock()
		defer s.mu.Unlock()
		t, ok := s.mem[taskID]
		if !ok {
			return nil, fmt.Errorf("task_store: not found: %s", taskID)
		}
		copy := *t
		return &copy, nil
	}

	var t Task
	var inputJSON, outputJSON []byte
	var completedAt *time.Time
	var errorMsg *string
	err := s.pool.QueryRow(ctx, `
		SELECT id, workspace_id, requesting_agent_id, capability,
		       input_payload, status, output_payload, error_message,
		       created_at, updated_at, completed_at
		FROM a2a_tasks WHERE id = $1
	`, taskID).Scan(
		&t.ID, &t.WorkspaceID, &t.RequestingAgentID, &t.Capability,
		&inputJSON, &t.Status, &outputJSON, &errorMsg,
		&t.CreatedAt, &t.UpdatedAt, &completedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("task_store: get: %w", err)
	}
	_ = json.Unmarshal(inputJSON, &t.InputPayload)
	if outputJSON != nil {
		_ = json.Unmarshal(outputJSON, &t.OutputPayload)
	}
	if errorMsg != nil {
		t.ErrorMessage = *errorMsg
	}
	t.CompletedAt = completedAt
	return &t, nil
}

// Update transitions a task to a new status with optional output.
func (s *TaskStore) Update(ctx context.Context, taskID string, req TaskUpdateRequest) (*Task, error) {
	now := time.Now().UTC()

	if s.pool == nil {
		s.mu.Lock()
		defer s.mu.Unlock()
		t, ok := s.mem[taskID]
		if !ok {
			return nil, fmt.Errorf("task_store: not found: %s", taskID)
		}
		t.Status = req.Status
		t.UpdatedAt = now
		if req.Output != nil {
			t.OutputPayload = req.Output
		}
		t.ErrorMessage = req.Error
		if isTerminalStatus(req.Status) {
			t.CompletedAt = &now
		}
		copy := *t
		return &copy, nil
	}

	var outputJSON []byte
	if req.Output != nil {
		outputJSON, _ = json.Marshal(req.Output)
	}
	var completedAt *time.Time
	if isTerminalStatus(req.Status) {
		completedAt = &now
	}
	_, err := s.pool.Exec(ctx, `
		UPDATE a2a_tasks SET status=$2, output_payload=$3::jsonb, error_message=$4,
		       updated_at=$5, completed_at=$6
		WHERE id = $1
	`, taskID, string(req.Status), nullableJSON(outputJSON), req.Error, now, completedAt)
	if err != nil {
		return nil, fmt.Errorf("task_store: update: %w", err)
	}
	return s.Get(ctx, taskID)
}

func nullableJSON(b []byte) interface{} {
	if b == nil {
		return nil
	}
	return string(b)
}
