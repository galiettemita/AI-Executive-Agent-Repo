package edge

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
)

// OfflineTier classifies how a task behaves when offline.
type OfflineTier int

const (
	// T0 means full offline capability.
	T0 OfflineTier = iota
	// T1 means cache-only offline capability.
	T1
	// T2 means queue-only offline capability.
	T2
	// T3 means online-required.
	T3
)

// OfflineTask represents a task queued for offline processing and later sync.
type OfflineTask struct {
	ID          string    `json:"id"`
	WorkspaceID string    `json:"workspace_id,omitempty"`
	AgentID     string    `json:"agent_id"`
	TaskType    string    `json:"task_type"`
	Payload     []byte    `json:"payload"`
	Priority    string    `json:"priority"` // critical, high, normal, low (or numeric string)
	Status      string    `json:"status"`   // queued, syncing, synced, executed, failed
	Result      []byte    `json:"result,omitempty"`
	FailReason  string    `json:"fail_reason,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	QueuedAt    time.Time `json:"queued_at,omitempty"`
	SyncedAt    time.Time `json:"synced_at,omitempty"`
}

// OfflineQueueService manages offline task queuing and synchronization.
type OfflineQueueService struct {
	mu    sync.Mutex
	tasks map[string]OfflineTask
	now   func() time.Time
}

// NewOfflineQueueService creates a new OfflineQueueService.
func NewOfflineQueueService() *OfflineQueueService {
	return &OfflineQueueService{
		tasks: map[string]OfflineTask{},
		now:   func() time.Time { return time.Now().UTC() },
	}
}

var priorityOrder = map[string]int{
	"critical": 0,
	"high":     1,
	"normal":   2,
	"low":      3,
}

func validPriority(p string) bool {
	_, ok := priorityOrder[p]
	return ok
}

// Enqueue adds a new task to the offline queue.
func (s *OfflineQueueService) Enqueue(_ context.Context, workspaceID, agentID, taskType string, payload []byte, priority string) (*OfflineTask, error) {
	if workspaceID == "" {
		return nil, fmt.Errorf("workspace ID is required")
	}
	if agentID == "" {
		return nil, fmt.Errorf("agent ID is required")
	}
	if taskType == "" {
		return nil, fmt.Errorf("task type is required")
	}
	if !validPriority(priority) {
		return nil, fmt.Errorf("invalid priority: %s", priority)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	p := make([]byte, len(payload))
	copy(p, payload)

	task := OfflineTask{
		ID:          uuid.Must(uuid.NewV7()).String(),
		WorkspaceID: workspaceID,
		AgentID:     agentID,
		TaskType:    taskType,
		Payload:     p,
		Priority:    priority,
		Status:      "queued",
		CreatedAt:   s.now(),
	}
	s.tasks[task.ID] = task
	return &task, nil
}

// Sync marks a task as synced.
func (s *OfflineQueueService) Sync(_ context.Context, taskID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	task, ok := s.tasks[taskID]
	if !ok {
		return fmt.Errorf("task not found: %s", taskID)
	}
	if task.Status == "synced" {
		return fmt.Errorf("task already synced: %s", taskID)
	}
	task.Status = "synced"
	task.SyncedAt = s.now()
	s.tasks[taskID] = task
	return nil
}

// FailSync marks a task as failed with a reason.
func (s *OfflineQueueService) FailSync(_ context.Context, taskID string, reason string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	task, ok := s.tasks[taskID]
	if !ok {
		return fmt.Errorf("task not found: %s", taskID)
	}
	if task.Status == "synced" {
		return fmt.Errorf("cannot fail already synced task: %s", taskID)
	}
	task.Status = "failed"
	task.FailReason = reason
	s.tasks[taskID] = task
	return nil
}

// ListPending returns all queued tasks for a workspace, ordered by priority then created_at.
func (s *OfflineQueueService) ListPending(_ context.Context, workspaceID string) []OfflineTask {
	s.mu.Lock()
	defer s.mu.Unlock()

	var out []OfflineTask
	for _, t := range s.tasks {
		if t.WorkspaceID == workspaceID && t.Status == "queued" {
			cp := t
			p := make([]byte, len(cp.Payload))
			copy(p, cp.Payload)
			cp.Payload = p
			out = append(out, cp)
		}
	}

	sort.Slice(out, func(i, j int) bool {
		pi := priorityOrder[out[i].Priority]
		pj := priorityOrder[out[j].Priority]
		if pi != pj {
			return pi < pj
		}
		return out[i].CreatedAt.Before(out[j].CreatedAt)
	})

	return out
}

// PurgeSynced removes synced tasks older than the given duration. Returns the count removed.
func (s *OfflineQueueService) PurgeSynced(_ context.Context, olderThan time.Duration) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	cutoff := s.now().Add(-olderThan)
	count := 0
	for id, t := range s.tasks {
		if t.Status == "synced" && t.SyncedAt.Before(cutoff) {
			delete(s.tasks, id)
			count++
		}
	}
	return count, nil
}

// DetermineOfflineTier classifies a task type into an offline tier.
func DetermineOfflineTier(taskType string) OfflineTier {
	switch taskType {
	case "draft_response", "summarize", "classify":
		return T0
	case "search_cache", "lookup":
		return T1
	case "send_email", "create_event", "update_record":
		return T2
	default:
		return T3
	}
}
