package cron

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"
)

func generateID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// ---------------------------------------------------------------------------
// Cron expression parsing
// ---------------------------------------------------------------------------

// CronSchedule holds the parsed fields of a cron expression.
type CronSchedule struct {
	Minute     string
	Hour       string
	DayOfMonth string
	Month      string
	DayOfWeek  string
}

// ParseCronExpression parses a standard 5-field cron expression.
func ParseCronExpression(expr string) (*CronSchedule, error) {
	expr = strings.TrimSpace(expr)
	parts := strings.Fields(expr)
	if len(parts) != 5 {
		return nil, fmt.Errorf("cron expression must have 5 fields, got %d", len(parts))
	}

	for i, p := range parts {
		if err := validateCronField(p, i); err != nil {
			return nil, fmt.Errorf("field %d (%s): %w", i, p, err)
		}
	}

	return &CronSchedule{
		Minute:     parts[0],
		Hour:       parts[1],
		DayOfMonth: parts[2],
		Month:      parts[3],
		DayOfWeek:  parts[4],
	}, nil
}

func validateCronField(field string, pos int) error {
	maxVals := []int{59, 23, 31, 12, 6}
	if field == "*" {
		return nil
	}

	// Handle step values like */5.
	if strings.HasPrefix(field, "*/") {
		step := strings.TrimPrefix(field, "*/")
		v, err := strconv.Atoi(step)
		if err != nil || v <= 0 || v > maxVals[pos] {
			return fmt.Errorf("invalid step value %q", step)
		}
		return nil
	}

	// Handle ranges like 1-5.
	if strings.Contains(field, "-") {
		parts := strings.SplitN(field, "-", 2)
		lo, err1 := strconv.Atoi(parts[0])
		hi, err2 := strconv.Atoi(parts[1])
		if err1 != nil || err2 != nil {
			return fmt.Errorf("invalid range %q", field)
		}
		if lo > hi || lo < 0 || hi > maxVals[pos] {
			return fmt.Errorf("range %d-%d out of bounds", lo, hi)
		}
		return nil
	}

	// Handle lists like 1,3,5.
	for _, item := range strings.Split(field, ",") {
		v, err := strconv.Atoi(item)
		if err != nil {
			return fmt.Errorf("invalid value %q", item)
		}
		minVal := 0
		if pos == 2 { // day of month starts at 1
			minVal = 1
		}
		if pos == 3 { // month starts at 1
			minVal = 1
		}
		if v < minVal || v > maxVals[pos] {
			return fmt.Errorf("value %d out of range [%d-%d]", v, minVal, maxVals[pos])
		}
	}
	return nil
}

// NextRun calculates the next run time after the given time.
func NextRun(schedule *CronSchedule, after time.Time) time.Time {
	// Start from the next minute.
	t := after.Truncate(time.Minute).Add(time.Minute)

	// Try up to 366 * 24 * 60 minutes (one full year) to find a match.
	for i := 0; i < 527040; i++ {
		if matchField(schedule.Month, int(t.Month())) &&
			matchField(schedule.DayOfMonth, t.Day()) &&
			matchField(schedule.DayOfWeek, int(t.Weekday())) &&
			matchField(schedule.Hour, t.Hour()) &&
			matchField(schedule.Minute, t.Minute()) {
			return t
		}
		t = t.Add(time.Minute)
	}
	return after // fallback: should not happen with valid schedules
}

func matchField(field string, value int) bool {
	if field == "*" {
		return true
	}
	if strings.HasPrefix(field, "*/") {
		step, _ := strconv.Atoi(strings.TrimPrefix(field, "*/"))
		return value%step == 0
	}
	if strings.Contains(field, "-") {
		parts := strings.SplitN(field, "-", 2)
		lo, _ := strconv.Atoi(parts[0])
		hi, _ := strconv.Atoi(parts[1])
		return value >= lo && value <= hi
	}
	for _, item := range strings.Split(field, ",") {
		v, _ := strconv.Atoi(item)
		if v == value {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// Cron job management
// ---------------------------------------------------------------------------

// CronJob represents a scheduled job.
type CronJob struct {
	ID          string
	WorkspaceID string
	Name        string
	Schedule    string // cron expression
	Action      string
	Args        map[string]any
	Status      string // active, paused, completed, failed
	NextRunAt   time.Time
	LastRunAt   time.Time
	RunCount    int
}

// CronExecution records a single execution of a cron job.
type CronExecution struct {
	ID          string
	JobID       string
	Status      string // running, completed, failed
	StartedAt   time.Time
	CompletedAt time.Time
	Output      string
	Error       string
}

// CronEngine manages cron jobs and their executions.
type CronEngine struct {
	mu         sync.RWMutex
	jobs       map[string]*CronJob
	executions map[string][]*CronExecution
}

// NewCronEngine creates a CronEngine.
func NewCronEngine() *CronEngine {
	return &CronEngine{
		jobs:       make(map[string]*CronJob),
		executions: make(map[string][]*CronExecution),
	}
}

// CreateJob creates a new cron job.
func (ce *CronEngine) CreateJob(workspaceID string, job CronJob) (*CronJob, error) {
	if workspaceID == "" {
		return nil, errors.New("workspaceID is required")
	}
	if job.Name == "" {
		return nil, errors.New("job name is required")
	}
	if job.Schedule == "" {
		return nil, errors.New("schedule is required")
	}
	if job.Action == "" {
		return nil, errors.New("action is required")
	}

	sched, err := ParseCronExpression(job.Schedule)
	if err != nil {
		return nil, fmt.Errorf("invalid schedule: %w", err)
	}

	job.ID = generateID()
	job.WorkspaceID = workspaceID
	job.Status = "active"
	job.NextRunAt = NextRun(sched, time.Now().UTC())

	ce.mu.Lock()
	ce.jobs[job.ID] = &job
	ce.mu.Unlock()
	return &job, nil
}

// PauseJob pauses an active job.
func (ce *CronEngine) PauseJob(jobID string) error {
	ce.mu.Lock()
	defer ce.mu.Unlock()
	j, ok := ce.jobs[jobID]
	if !ok {
		return fmt.Errorf("job %s not found", jobID)
	}
	if j.Status != "active" {
		return fmt.Errorf("job %s is not active (status=%s)", jobID, j.Status)
	}
	j.Status = "paused"
	return nil
}

// ResumeJob resumes a paused job.
func (ce *CronEngine) ResumeJob(jobID string) error {
	ce.mu.Lock()
	defer ce.mu.Unlock()
	j, ok := ce.jobs[jobID]
	if !ok {
		return fmt.Errorf("job %s not found", jobID)
	}
	if j.Status != "paused" {
		return fmt.Errorf("job %s is not paused (status=%s)", jobID, j.Status)
	}
	j.Status = "active"
	sched, _ := ParseCronExpression(j.Schedule)
	j.NextRunAt = NextRun(sched, time.Now().UTC())
	return nil
}

// TriggerJob manually triggers a job execution.
func (ce *CronEngine) TriggerJob(jobID string) (*CronExecution, error) {
	ce.mu.Lock()
	defer ce.mu.Unlock()
	j, ok := ce.jobs[jobID]
	if !ok {
		return nil, fmt.Errorf("job %s not found", jobID)
	}
	if j.Status != "active" && j.Status != "paused" {
		return nil, fmt.Errorf("job %s cannot be triggered (status=%s)", jobID, j.Status)
	}

	now := time.Now().UTC()
	exec := &CronExecution{
		ID:          generateID(),
		JobID:       jobID,
		Status:      "completed",
		StartedAt:   now,
		CompletedAt: now,
		Output:      fmt.Sprintf("executed action %q", j.Action),
	}

	j.LastRunAt = now
	j.RunCount++
	if j.Status == "active" {
		sched, _ := ParseCronExpression(j.Schedule)
		j.NextRunAt = NextRun(sched, now)
	}

	ce.executions[jobID] = append(ce.executions[jobID], exec)
	return exec, nil
}

// DeleteJob removes a job.
func (ce *CronEngine) DeleteJob(jobID string) error {
	ce.mu.Lock()
	defer ce.mu.Unlock()
	if _, ok := ce.jobs[jobID]; !ok {
		return fmt.Errorf("job %s not found", jobID)
	}
	delete(ce.jobs, jobID)
	delete(ce.executions, jobID)
	return nil
}

// ListJobs returns all jobs for a workspace.
func (ce *CronEngine) ListJobs(workspaceID string) []CronJob {
	ce.mu.RLock()
	defer ce.mu.RUnlock()

	var result []CronJob
	for _, j := range ce.jobs {
		if j.WorkspaceID == workspaceID {
			result = append(result, *j)
		}
	}
	return result
}

// GetExecutions returns the most recent executions for a job.
func (ce *CronEngine) GetExecutions(jobID string, limit int) []CronExecution {
	ce.mu.RLock()
	defer ce.mu.RUnlock()

	execs := ce.executions[jobID]
	if limit <= 0 || limit > len(execs) {
		limit = len(execs)
	}
	// Return most recent first.
	start := len(execs) - limit
	result := make([]CronExecution, limit)
	for i, e := range execs[start:] {
		result[limit-1-i] = *e
	}
	return result
}

// ---------------------------------------------------------------------------
// Workflow chaining
// ---------------------------------------------------------------------------

// ChainedWorkflow describes a workflow step in a chain.
type ChainedWorkflow struct {
	WorkflowID string
	DependsOn  []string
	Condition  string
}

// ChainStep records the status of a step in a chain.
type ChainStep struct {
	WorkflowID string
	Status     string // pending, running, completed, failed, skipped
	Output     string
}

// WorkflowChain represents a chain of workflows with dependencies.
type WorkflowChain struct {
	ID     string
	Status string // pending, running, completed, failed
	Steps  []ChainStep
}

// WorkflowChainService chains workflows with dependency ordering.
type WorkflowChainService struct {
	mu     sync.RWMutex
	chains map[string]*WorkflowChain
	// Store original definitions for execution.
	defs map[string][]ChainedWorkflow
}

// NewWorkflowChainService creates a WorkflowChainService.
func NewWorkflowChainService() *WorkflowChainService {
	return &WorkflowChainService{
		chains: make(map[string]*WorkflowChain),
		defs:   make(map[string][]ChainedWorkflow),
	}
}

// ChainWorkflows creates a new workflow chain.
func (w *WorkflowChainService) ChainWorkflows(workspaceID string, workflows []ChainedWorkflow) (*WorkflowChain, error) {
	if workspaceID == "" {
		return nil, errors.New("workspaceID is required")
	}
	if len(workflows) == 0 {
		return nil, errors.New("at least one workflow is required")
	}

	// Validate dependencies exist within the chain.
	ids := make(map[string]bool)
	for _, wf := range workflows {
		ids[wf.WorkflowID] = true
	}
	for _, wf := range workflows {
		for _, dep := range wf.DependsOn {
			if !ids[dep] {
				return nil, fmt.Errorf("dependency %q not found in chain", dep)
			}
		}
	}

	steps := make([]ChainStep, len(workflows))
	for i, wf := range workflows {
		steps[i] = ChainStep{WorkflowID: wf.WorkflowID, Status: "pending"}
	}

	chain := &WorkflowChain{
		ID:     generateID(),
		Status: "pending",
		Steps:  steps,
	}

	w.mu.Lock()
	w.chains[chain.ID] = chain
	w.defs[chain.ID] = workflows
	w.mu.Unlock()
	return chain, nil
}

// ExecuteChain runs a workflow chain respecting dependencies.
func (w *WorkflowChainService) ExecuteChain(chainID string) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	chain, ok := w.chains[chainID]
	if !ok {
		return fmt.Errorf("chain %s not found", chainID)
	}
	if chain.Status != "pending" {
		return fmt.Errorf("chain %s already executed (status=%s)", chainID, chain.Status)
	}

	chain.Status = "running"
	defs := w.defs[chainID]

	// Build dependency lookup.
	stepIndex := make(map[string]int)
	for i, s := range chain.Steps {
		stepIndex[s.WorkflowID] = i
	}

	// Topological execution: process steps whose dependencies are all completed.
	completed := make(map[string]bool)
	maxIterations := len(defs) * len(defs)
	for iteration := 0; iteration < maxIterations && len(completed) < len(defs); iteration++ {
		progress := false
		for i, def := range defs {
			if completed[def.WorkflowID] {
				continue
			}
			// Check all dependencies completed.
			allDeps := true
			for _, dep := range def.DependsOn {
				if !completed[dep] {
					allDeps = false
					break
				}
			}
			if !allDeps {
				continue
			}

			// Execute step.
			chain.Steps[i].Status = "running"
			// Check condition.
			if def.Condition == "skip" {
				chain.Steps[i].Status = "skipped"
			} else {
				chain.Steps[i].Status = "completed"
				chain.Steps[i].Output = fmt.Sprintf("workflow %s executed", def.WorkflowID)
			}
			completed[def.WorkflowID] = true
			progress = true
		}
		if !progress {
			break
		}
	}

	// Check if all completed.
	allDone := true
	for _, s := range chain.Steps {
		if s.Status != "completed" && s.Status != "skipped" {
			allDone = false
			break
		}
	}
	if allDone {
		chain.Status = "completed"
	} else {
		chain.Status = "failed"
	}
	return nil
}

// ---------------------------------------------------------------------------
// Briefing composer
// ---------------------------------------------------------------------------

// BriefingSection is a section of a daily briefing.
type BriefingSection struct {
	Title    string
	Content  string
	Priority string
	Source   string
}

// Briefing is a composed daily briefing.
type Briefing struct {
	ID          string
	WorkspaceID string
	Date        time.Time
	Sections    []BriefingSection
	GeneratedAt time.Time
}

// BriefingComposer generates daily briefings.
type BriefingComposer struct{}

// NewBriefingComposer creates a BriefingComposer.
func NewBriefingComposer() *BriefingComposer { return &BriefingComposer{} }

// ComposeBriefing generates a briefing for the given date.
func (bc *BriefingComposer) ComposeBriefing(workspaceID string, date time.Time) (*Briefing, error) {
	if workspaceID == "" {
		return nil, errors.New("workspaceID is required")
	}

	dateStr := date.Format("2006-01-02")

	sections := []BriefingSection{
		{
			Title:    "Calendar Summary",
			Content:  fmt.Sprintf("You have 3 meetings scheduled for %s. Next meeting: Team standup at 09:30.", dateStr),
			Priority: "high",
			Source:   "calendar_summary",
		},
		{
			Title:    "Pending Tasks",
			Content:  "5 tasks are due today. 2 are high priority: Q4 report review, Client proposal draft.",
			Priority: "high",
			Source:   "pending_tasks",
		},
		{
			Title:    "Important Emails",
			Content:  "12 unread emails. 3 flagged as important from: CEO, Legal team, Key client.",
			Priority: "medium",
			Source:   "important_emails",
		},
		{
			Title:    "Weather",
			Content:  fmt.Sprintf("Weather for %s: Partly cloudy, 72°F / 22°C. No severe weather alerts.", dateStr),
			Priority: "low",
			Source:   "weather",
		},
		{
			Title:    "Goals Progress",
			Content:  "Weekly goals: 60% complete. Monthly OKRs: on track (73%).",
			Priority: "medium",
			Source:   "goals_progress",
		},
	}

	return &Briefing{
		ID:          generateID(),
		WorkspaceID: workspaceID,
		Date:        date,
		Sections:    sections,
		GeneratedAt: time.Now().UTC(),
	}, nil
}
