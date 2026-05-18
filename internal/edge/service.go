package edge

import (
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Offline tier constants.
const (
	T0FullOffline = "T0_FULL_OFFLINE"
	T1QueueOnly   = "T1_QUEUE_ONLY"
	T2ReadCache   = "T2_READ_CACHE"
	T3Connected   = "T3_CONNECTED"
)

// EdgeAgent represents an edge agent registration.
type EdgeAgent struct {
	ID           string    `json:"id"`
	WorkspaceID  string    `json:"workspace_id"`
	Platform     string    `json:"platform"`
	Version      string    `json:"version"`
	Status       string    `json:"status"` // online, offline, syncing
	LastSeen     time.Time `json:"last_seen"`
	Capabilities []string  `json:"capabilities"`
}

// EdgeService manages edge agent registrations and offline task queues.
type EdgeService struct {
	mu     sync.Mutex
	agents map[string]EdgeAgent
	tasks  map[string]OfflineTask
	now    func() time.Time
}

// NewEdgeService creates a new EdgeService.
func NewEdgeService() *EdgeService {
	return &EdgeService{
		agents: map[string]EdgeAgent{},
		tasks:  map[string]OfflineTask{},
		now:    func() time.Time { return time.Now().UTC() },
	}
}

// RegisterAgent registers a new edge agent.
func (s *EdgeService) RegisterAgent(workspaceID, platform, version string, capabilities []string) (*EdgeAgent, error) {
	if workspaceID == "" {
		return nil, fmt.Errorf("workspace ID is required")
	}
	if platform == "" {
		return nil, fmt.Errorf("platform is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	caps := make([]string, len(capabilities))
	copy(caps, capabilities)

	agent := EdgeAgent{
		ID:           uuid.Must(uuid.NewV7()).String(),
		WorkspaceID:  workspaceID,
		Platform:     platform,
		Version:      version,
		Status:       "online",
		LastSeen:     s.now(),
		Capabilities: caps,
	}
	s.agents[agent.ID] = agent
	return &agent, nil
}

// UpdateHeartbeat updates the last-seen timestamp and sets status to online.
func (s *EdgeService) UpdateHeartbeat(agentID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	agent, ok := s.agents[agentID]
	if !ok {
		return fmt.Errorf("agent not found: %s", agentID)
	}
	agent.LastSeen = s.now()
	agent.Status = "online"
	s.agents[agentID] = agent
	return nil
}

// QueueOfflineTask queues a task for an edge agent to execute offline.
func (s *EdgeService) QueueOfflineTask(agentID string, taskType string, payload []byte, priority int) (*OfflineTask, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.agents[agentID]; !ok {
		return nil, fmt.Errorf("agent not found: %s", agentID)
	}
	if taskType == "" {
		return nil, fmt.Errorf("task type is required")
	}

	p := make([]byte, len(payload))
	copy(p, payload)

	task := OfflineTask{
		ID:       uuid.Must(uuid.NewV7()).String(),
		AgentID:  agentID,
		TaskType: taskType,
		Payload:  p,
		Priority: strconv.Itoa(priority),
		Status:   "queued",
		QueuedAt: s.now(),
	}
	s.tasks[task.ID] = task
	return &task, nil
}

// SyncTasks returns queued tasks for an agent and marks them as synced.
func (s *EdgeService) SyncTasks(agentID string) ([]OfflineTask, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.agents[agentID]; !ok {
		return nil, fmt.Errorf("agent not found: %s", agentID)
	}

	agent := s.agents[agentID]
	agent.Status = "syncing"
	s.agents[agentID] = agent

	var out []OfflineTask
	for id, task := range s.tasks {
		if task.AgentID == agentID && task.Status == "queued" {
			task.Status = "synced"
			task.SyncedAt = s.now()
			s.tasks[id] = task
			out = append(out, task)
		}
	}
	return out, nil
}

// ReportTaskResult reports the result of a task execution.
func (s *EdgeService) ReportTaskResult(taskID string, success bool, result []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	task, ok := s.tasks[taskID]
	if !ok {
		return fmt.Errorf("task not found: %s", taskID)
	}
	if task.Status != "synced" {
		return fmt.Errorf("task is not in synced state: %s", task.Status)
	}

	r := make([]byte, len(result))
	copy(r, result)
	task.Result = r

	if success {
		task.Status = "executed"
	} else {
		task.Status = "failed"
	}
	s.tasks[taskID] = task
	return nil
}

// GetOfflineTier determines the offline tier for an agent based on its capabilities.
func (s *EdgeService) GetOfflineTier(agentID string) string {
	s.mu.Lock()
	defer s.mu.Unlock()

	agent, ok := s.agents[agentID]
	if !ok {
		return T3Connected
	}

	capSet := map[string]bool{}
	for _, c := range agent.Capabilities {
		capSet[c] = true
	}

	if capSet["full_offline"] {
		return T0FullOffline
	}
	if capSet["queue_offline"] {
		return T1QueueOnly
	}
	if capSet["read_cache"] {
		return T2ReadCache
	}
	return T3Connected
}
