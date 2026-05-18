package dlq

import (
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Status represents the lifecycle state of a DLQ entry.
type Status string

const (
	StatusPending   Status = "pending"
	StatusRetrying  Status = "retrying"
	StatusExhausted Status = "exhausted"
	StatusResolved  Status = "resolved"
)

// Known queue names.
const (
	QueueInteractiveTurns     = "interactive_turns_dlq"
	QueueWorkflowTasks        = "workflow_tasks_dlq"
	QueueLedgerWrites         = "ledger_writes_dlq"
	QueueTrajectoryWrites     = "trajectory_writes_dlq"
	QueueRateLimitLedgerWrites = "rate_limit_ledger_writes_dlq"
)

var knownQueues = map[string]bool{
	QueueInteractiveTurns:      true,
	QueueWorkflowTasks:         true,
	QueueLedgerWrites:          true,
	QueueTrajectoryWrites:      true,
	QueueRateLimitLedgerWrites: true,
}

// Entry represents a single dead-letter queue item.
type Entry struct {
	ID              string    `json:"id"`
	QueueName       string    `json:"queue_name"`
	OriginalPayload []byte    `json:"original_payload"`
	ErrorMessage    string    `json:"error_message"`
	Attempts        int       `json:"attempts"`
	MaxAttempts     int       `json:"max_attempts"`
	FirstFailedAt   time.Time `json:"first_failed_at"`
	LastFailedAt    time.Time `json:"last_failed_at"`
	Status          Status    `json:"status"`
}

// Service manages dead-letter queues for failed processing across the system.
type Service struct {
	mu      sync.Mutex
	entries map[string]*Entry          // id -> entry
	queues  map[string][]string        // queueName -> ordered entry IDs
	now     func() time.Time
}

// NewService creates a new DLQ service.
func NewService() *Service {
	return &Service{
		entries: map[string]*Entry{},
		queues:  map[string][]string{},
		now:     func() time.Time { return time.Now().UTC() },
	}
}

// Enqueue adds a failed item to the specified dead-letter queue.
func (s *Service) Enqueue(queueName string, payload []byte, errMsg string) (*Entry, error) {
	if !knownQueues[queueName] {
		return nil, fmt.Errorf("unknown queue: %s", queueName)
	}
	if len(payload) == 0 {
		return nil, fmt.Errorf("payload must not be empty")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	now := s.now()
	entry := &Entry{
		ID:              uuid.Must(uuid.NewV7()).String(),
		QueueName:       queueName,
		OriginalPayload: make([]byte, len(payload)),
		ErrorMessage:    errMsg,
		Attempts:        1,
		MaxAttempts:     3,
		FirstFailedAt:   now,
		LastFailedAt:    now,
		Status:          StatusPending,
	}
	copy(entry.OriginalPayload, payload)

	s.entries[entry.ID] = entry
	s.queues[queueName] = append(s.queues[queueName], entry.ID)
	return entry, nil
}

// Dequeue removes and returns the oldest pending entry from the specified queue.
func (s *Service) Dequeue(queueName string) (*Entry, error) {
	if !knownQueues[queueName] {
		return nil, fmt.Errorf("unknown queue: %s", queueName)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	ids := s.queues[queueName]
	for i, id := range ids {
		entry := s.entries[id]
		if entry.Status == StatusPending || entry.Status == StatusRetrying {
			// Remove from queue list.
			s.queues[queueName] = append(ids[:i], ids[i+1:]...)
			delete(s.entries, id)
			cp := *entry
			return &cp, nil
		}
	}
	return nil, fmt.Errorf("no pending entries in queue: %s", queueName)
}

// Peek returns the oldest pending entry without removing it.
func (s *Service) Peek(queueName string) (*Entry, error) {
	if !knownQueues[queueName] {
		return nil, fmt.Errorf("unknown queue: %s", queueName)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	ids := s.queues[queueName]
	for _, id := range ids {
		entry := s.entries[id]
		if entry.Status == StatusPending || entry.Status == StatusRetrying {
			cp := *entry
			return &cp, nil
		}
	}
	return nil, fmt.Errorf("no pending entries in queue: %s", queueName)
}

// Retry increments the attempt counter and transitions status accordingly.
// If max attempts are reached, the entry is marked exhausted.
func (s *Service) Retry(entryID string) (*Entry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	entry, ok := s.entries[entryID]
	if !ok {
		return nil, fmt.Errorf("entry not found: %s", entryID)
	}
	if entry.Status == StatusResolved {
		return nil, fmt.Errorf("cannot retry resolved entry: %s", entryID)
	}
	if entry.Status == StatusExhausted {
		return nil, fmt.Errorf("cannot retry exhausted entry: %s", entryID)
	}

	entry.Attempts++
	entry.LastFailedAt = s.now()

	if entry.Attempts >= entry.MaxAttempts {
		entry.Status = StatusExhausted
	} else {
		entry.Status = StatusRetrying
	}

	cp := *entry
	return &cp, nil
}

// Resolve marks a DLQ entry as resolved.
func (s *Service) Resolve(entryID string) (*Entry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	entry, ok := s.entries[entryID]
	if !ok {
		return nil, fmt.Errorf("entry not found: %s", entryID)
	}
	entry.Status = StatusResolved

	cp := *entry
	return &cp, nil
}

// ListByQueue returns all entries for a given queue name.
func (s *Service) ListByQueue(queueName string) []Entry {
	s.mu.Lock()
	defer s.mu.Unlock()

	ids := s.queues[queueName]
	out := make([]Entry, 0, len(ids))
	for _, id := range ids {
		if entry, ok := s.entries[id]; ok {
			out = append(out, *entry)
		}
	}
	return out
}

// CountByQueue returns the number of entries in a given queue.
func (s *Service) CountByQueue(queueName string) int {
	s.mu.Lock()
	defer s.mu.Unlock()

	count := 0
	for _, id := range s.queues[queueName] {
		if _, ok := s.entries[id]; ok {
			count++
		}
	}
	return count
}

// PurgeResolved removes all resolved entries from the specified queue.
func (s *Service) PurgeResolved(queueName string) int {
	s.mu.Lock()
	defer s.mu.Unlock()

	ids := s.queues[queueName]
	remaining := make([]string, 0, len(ids))
	purged := 0
	for _, id := range ids {
		entry, ok := s.entries[id]
		if !ok {
			continue
		}
		if entry.Status == StatusResolved {
			delete(s.entries, id)
			purged++
		} else {
			remaining = append(remaining, id)
		}
	}
	s.queues[queueName] = remaining
	return purged
}
