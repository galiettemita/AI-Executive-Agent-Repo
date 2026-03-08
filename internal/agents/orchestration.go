package agents

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"
)

func generateID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// ---------------------------------------------------------------------------
// Agent Council — multi-agent deliberation
// ---------------------------------------------------------------------------

// AgentMember represents a single agent participating in a council.
type AgentMember struct {
	AgentID      string
	Role         string
	Capabilities []string
	Vote         string
}

// AgentCouncil is a group of agents convened to deliberate on a topic.
type AgentCouncil struct {
	ID          string
	WorkspaceID string
	Topic       string
	Agents      []AgentMember
	Status      string // convening, deliberating, resolved
	Resolution  string
	CreatedAt   time.Time
}

// Resolution captures the outcome of a council deliberation.
type Resolution struct {
	Decision   string
	Confidence float64
	Dissenting []string
}

// OrchestrationEngine coordinates multi-agent decision-making.
type OrchestrationEngine struct {
	mu       sync.RWMutex
	councils map[string]*AgentCouncil
}

// NewOrchestrationEngine creates an OrchestrationEngine.
func NewOrchestrationEngine() *OrchestrationEngine {
	return &OrchestrationEngine{councils: make(map[string]*AgentCouncil)}
}

// ConveneCouncil creates a new council for the given topic.
func (o *OrchestrationEngine) ConveneCouncil(workspaceID, topic string, agents []AgentMember) (*AgentCouncil, error) {
	if workspaceID == "" {
		return nil, errors.New("workspaceID is required")
	}
	if topic == "" {
		return nil, errors.New("topic is required")
	}
	if len(agents) < 2 {
		return nil, errors.New("at least 2 agents are required for a council")
	}

	council := &AgentCouncil{
		ID:          generateID(),
		WorkspaceID: workspaceID,
		Topic:       topic,
		Agents:      agents,
		Status:      "convening",
		CreatedAt:   time.Now().UTC(),
	}

	o.mu.Lock()
	o.councils[council.ID] = council
	o.mu.Unlock()
	return council, nil
}

// Deliberate triggers each agent to provide input on the topic.  Votes are
// assigned based on agent role (simulated).
func (o *OrchestrationEngine) Deliberate(councilID string) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	c, ok := o.councils[councilID]
	if !ok {
		return fmt.Errorf("council %s not found", councilID)
	}
	if c.Status != "convening" {
		return fmt.Errorf("council %s is not in convening status (status=%s)", councilID, c.Status)
	}

	// Each agent votes based on its role.
	for i := range c.Agents {
		switch c.Agents[i].Role {
		case "analyst":
			c.Agents[i].Vote = "approve"
		case "critic":
			c.Agents[i].Vote = "reject"
		case "mediator":
			c.Agents[i].Vote = "approve"
		default:
			c.Agents[i].Vote = "approve"
		}
	}
	c.Status = "deliberating"
	return nil
}

// ResolveCouncil tallies votes and produces a resolution.
func (o *OrchestrationEngine) ResolveCouncil(councilID string) (*Resolution, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	c, ok := o.councils[councilID]
	if !ok {
		return nil, fmt.Errorf("council %s not found", councilID)
	}
	if c.Status != "deliberating" {
		return nil, fmt.Errorf("council %s is not deliberating (status=%s)", councilID, c.Status)
	}

	votes := make(map[string]int)
	for _, a := range c.Agents {
		votes[a.Vote]++
	}

	// Majority vote.
	bestVote := ""
	bestCount := 0
	for v, count := range votes {
		if count > bestCount {
			bestVote = v
			bestCount = count
		}
	}

	var dissenting []string
	for _, a := range c.Agents {
		if a.Vote != bestVote {
			dissenting = append(dissenting, a.AgentID)
		}
	}

	confidence := float64(bestCount) / float64(len(c.Agents))

	c.Status = "resolved"
	c.Resolution = bestVote

	return &Resolution{
		Decision:   bestVote,
		Confidence: confidence,
		Dissenting: dissenting,
	}, nil
}

// ---------------------------------------------------------------------------
// God Mode — supervisor agent with elevated privileges
// ---------------------------------------------------------------------------

// GodModeSession represents an elevated-privilege session.
type GodModeSession struct {
	ID          string
	WorkspaceID string
	Reason      string
	ActivatedBy string
	Active      bool
	Actions     []string
}

// GodModeService manages privileged agent sessions.
type GodModeService struct {
	mu       sync.RWMutex
	sessions map[string]*GodModeSession
}

// NewGodModeService creates a GodModeService.
func NewGodModeService() *GodModeService {
	return &GodModeService{sessions: make(map[string]*GodModeSession)}
}

// ActivateGodMode creates a new elevated session.
func (g *GodModeService) ActivateGodMode(workspaceID, reason string) (*GodModeSession, error) {
	if workspaceID == "" {
		return nil, errors.New("workspaceID is required")
	}
	if reason == "" {
		return nil, errors.New("reason is required for god mode activation")
	}

	sess := &GodModeSession{
		ID:          generateID(),
		WorkspaceID: workspaceID,
		Reason:      reason,
		ActivatedBy: "supervisor",
		Active:      true,
		Actions:     []string{},
	}

	g.mu.Lock()
	g.sessions[sess.ID] = sess
	g.mu.Unlock()
	return sess, nil
}

// ExecutePrivileged runs a privileged action within a god mode session.
func (g *GodModeService) ExecutePrivileged(sessionID, action string) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	sess, ok := g.sessions[sessionID]
	if !ok {
		return fmt.Errorf("god mode session %s not found", sessionID)
	}
	if !sess.Active {
		return fmt.Errorf("god mode session %s is no longer active", sessionID)
	}
	if action == "" {
		return errors.New("action is required")
	}
	sess.Actions = append(sess.Actions, action)
	return nil
}

// DeactivateGodMode ends an elevated session.
func (g *GodModeService) DeactivateGodMode(sessionID string) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	sess, ok := g.sessions[sessionID]
	if !ok {
		return fmt.Errorf("god mode session %s not found", sessionID)
	}
	if !sess.Active {
		return fmt.Errorf("god mode session %s already deactivated", sessionID)
	}
	sess.Active = false
	return nil
}

// ---------------------------------------------------------------------------
// Delegation
// ---------------------------------------------------------------------------

// DelegationTask describes a task to be delegated.
type DelegationTask struct {
	Description          string
	RequiredCapabilities []string
	Priority             string
	Deadline             time.Time
}

// DelegatedTask is a task that has been assigned to an agent.
type DelegatedTask struct {
	ID            string
	AssignedAgent string
	Description   string
	Status        string // pending, assigned, in_progress, completed, failed
	Priority      string
	Deadline      time.Time
}

// DelegationService assigns tasks to the best-fit agent.
type DelegationService struct {
	mu    sync.RWMutex
	tasks map[string]*DelegatedTask
	// registry of agents and their capabilities.
	agents map[string][]string
}

// NewDelegationService creates a DelegationService.
func NewDelegationService() *DelegationService {
	return &DelegationService{
		tasks: make(map[string]*DelegatedTask),
		agents: map[string][]string{
			"agent-research":  {"research", "analysis", "summarization"},
			"agent-code":      {"coding", "debugging", "testing"},
			"agent-comms":     {"email", "messaging", "scheduling"},
			"agent-data":      {"data_analysis", "visualization", "reporting"},
		},
	}
}

// DelegateTask assigns a task to the most capable agent.
func (d *DelegationService) DelegateTask(workspaceID string, task DelegationTask) (*DelegatedTask, error) {
	if workspaceID == "" {
		return nil, errors.New("workspaceID is required")
	}
	if task.Description == "" {
		return nil, errors.New("task description is required")
	}

	// Score each agent by how many required capabilities it has.
	type scored struct {
		agentID string
		score   int
	}
	var scores []scored
	for agentID, caps := range d.agents {
		s := 0
		capSet := make(map[string]bool, len(caps))
		for _, c := range caps {
			capSet[c] = true
		}
		for _, req := range task.RequiredCapabilities {
			if capSet[req] {
				s++
			}
		}
		scores = append(scores, scored{agentID, s})
	}
	sort.Slice(scores, func(i, j int) bool { return scores[i].score > scores[j].score })

	assignedAgent := "agent-general"
	if len(scores) > 0 && scores[0].score > 0 {
		assignedAgent = scores[0].agentID
	}

	dt := &DelegatedTask{
		ID:            generateID(),
		AssignedAgent: assignedAgent,
		Description:   task.Description,
		Status:        "assigned",
		Priority:      task.Priority,
		Deadline:      task.Deadline,
	}

	d.mu.Lock()
	d.tasks[dt.ID] = dt
	d.mu.Unlock()
	return dt, nil
}

// GetTaskStatus returns the current status of a delegated task.
func (d *DelegationService) GetTaskStatus(taskID string) (*DelegatedTask, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	t, ok := d.tasks[taskID]
	if !ok {
		return nil, fmt.Errorf("task %s not found", taskID)
	}
	return t, nil
}

// ---------------------------------------------------------------------------
// Parallel execution
// ---------------------------------------------------------------------------

// ParallelTask describes a single task in a parallel batch.
type ParallelTask struct {
	ID     string
	Action string
	Args   map[string]any
}

// TaskResult is the outcome of a single parallel task.
type TaskResult struct {
	TaskID   string
	Success  bool
	Output   map[string]any
	Duration time.Duration
	Error    string
}

// ParallelResult aggregates the results of parallel execution.
type ParallelResult struct {
	Results       []TaskResult
	TotalDuration time.Duration
	SuccessCount  int
	FailCount     int
}

// ParallelExecutor runs multiple tasks concurrently.
type ParallelExecutor struct{}

// NewParallelExecutor creates a ParallelExecutor.
func NewParallelExecutor() *ParallelExecutor { return &ParallelExecutor{} }

// ExecuteParallel runs all tasks concurrently and collects results.
func (p *ParallelExecutor) ExecuteParallel(workspaceID string, tasks []ParallelTask) (*ParallelResult, error) {
	if workspaceID == "" {
		return nil, errors.New("workspaceID is required")
	}
	if len(tasks) == 0 {
		return nil, errors.New("at least one task is required")
	}

	start := time.Now()
	results := make([]TaskResult, len(tasks))
	var wg sync.WaitGroup

	for i, task := range tasks {
		wg.Add(1)
		go func(idx int, t ParallelTask) {
			defer wg.Done()
			taskStart := time.Now()

			// Simulate task execution.  Tasks with action "fail" will fail.
			if t.Action == "fail" {
				results[idx] = TaskResult{
					TaskID:   t.ID,
					Success:  false,
					Duration: time.Since(taskStart),
					Error:    "simulated failure",
				}
				return
			}

			results[idx] = TaskResult{
				TaskID:   t.ID,
				Success:  true,
				Output:   map[string]any{"action": t.Action, "status": "completed"},
				Duration: time.Since(taskStart),
			}
		}(i, task)
	}
	wg.Wait()

	successCount := 0
	failCount := 0
	for _, r := range results {
		if r.Success {
			successCount++
		} else {
			failCount++
		}
	}

	return &ParallelResult{
		Results:       results,
		TotalDuration: time.Since(start),
		SuccessCount:  successCount,
		FailCount:     failCount,
	}, nil
}
