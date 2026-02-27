package codebase_intel

import (
	"fmt"
	"sort"
	"sync"
)

type Dependency struct {
	ID          string `json:"id"`
	WorkspaceID string `json:"workspace_id"`
	Name        string `json:"name"`
	Version     string `json:"version"`
}

type Pattern struct {
	ID          string `json:"id"`
	WorkspaceID string `json:"workspace_id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

type DebtItem struct {
	ID          string `json:"id"`
	WorkspaceID string `json:"workspace_id"`
	Title       string `json:"title"`
	Severity    string `json:"severity"`
	Status      string `json:"status"`
}

type DebtTask struct {
	ID          string `json:"id"`
	DebtID      string `json:"debt_id"`
	WorkspaceID string `json:"workspace_id"`
	Title       string `json:"title"`
	Status      string `json:"status"`
}

type ProjectTemplate struct {
	ID          string `json:"id"`
	WorkspaceID string `json:"workspace_id"`
	Name        string `json:"name"`
	Status      string `json:"status"`
}

type ContextExport struct {
	ID          string `json:"id"`
	WorkspaceID string `json:"workspace_id"`
	Status      string `json:"status"`
	Format      string `json:"format"`
}

type Service struct {
	mu           sync.RWMutex
	nextID       int
	dependencies []Dependency
	patterns     []Pattern
	debt         map[string]DebtItem
	debtTasks    map[string]map[string]DebtTask
	templates    map[string]ProjectTemplate
	exports      map[string]ContextExport
}

func NewService() *Service {
	return &Service{
		nextID: 1,
		dependencies: []Dependency{
			{ID: "dep_001", WorkspaceID: "default", Name: "pgx", Version: "v5"},
		},
		patterns: []Pattern{
			{ID: "pattern_001", WorkspaceID: "default", Name: "deterministic_handlers", Description: "Pure deterministic request handlers"},
		},
		debt:      map[string]DebtItem{},
		debtTasks: map[string]map[string]DebtTask{},
		templates: map[string]ProjectTemplate{},
		exports:   map[string]ContextExport{},
	}
}

func (s *Service) ListDependencies(workspaceID string) []Dependency {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Dependency, 0, len(s.dependencies))
	for _, dep := range s.dependencies {
		if workspaceID != "" && dep.WorkspaceID != workspaceID && dep.WorkspaceID != "default" {
			continue
		}
		out = append(out, dep)
	}
	return out
}

func (s *Service) ListPatterns(workspaceID string) []Pattern {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Pattern, 0, len(s.patterns))
	for _, pattern := range s.patterns {
		if workspaceID != "" && pattern.WorkspaceID != workspaceID && pattern.WorkspaceID != "default" {
			continue
		}
		out = append(out, pattern)
	}
	return out
}

func (s *Service) ListDebt(workspaceID string) []DebtItem {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]DebtItem, 0, len(s.debt))
	for _, item := range s.debt {
		if workspaceID != "" && item.WorkspaceID != workspaceID {
			continue
		}
		out = append(out, item)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].ID < out[j].ID
	})
	return out
}

func (s *Service) UpsertDebt(id string, item DebtItem) DebtItem {
	s.mu.Lock()
	defer s.mu.Unlock()
	if id == "" {
		id = fmt.Sprintf("debt_%06d", s.nextID)
		s.nextID++
	}
	item.ID = id
	if item.WorkspaceID == "" {
		item.WorkspaceID = "default"
	}
	if item.Severity == "" {
		item.Severity = "medium"
	}
	if item.Status == "" {
		item.Status = "open"
	}
	s.debt[id] = item
	return item
}

func (s *Service) ListDebtTasks(debtID string) []DebtTask {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]DebtTask, 0, len(s.debtTasks[debtID]))
	for _, task := range s.debtTasks[debtID] {
		out = append(out, task)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].ID < out[j].ID
	})
	return out
}

func (s *Service) AddDebtTask(debtID string, task DebtTask) DebtTask {
	s.mu.Lock()
	defer s.mu.Unlock()
	task.ID = fmt.Sprintf("debt_task_%06d", s.nextID)
	s.nextID++
	task.DebtID = debtID
	if task.Status == "" {
		task.Status = "open"
	}
	if _, ok := s.debtTasks[debtID]; !ok {
		s.debtTasks[debtID] = map[string]DebtTask{}
	}
	s.debtTasks[debtID][task.ID] = task
	return task
}

func (s *Service) GetDebtTask(debtID, taskID string) (DebtTask, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	task, ok := s.debtTasks[debtID][taskID]
	return task, ok
}

func (s *Service) UpsertDebtTask(debtID, taskID string, task DebtTask) DebtTask {
	s.mu.Lock()
	defer s.mu.Unlock()
	if taskID == "" {
		taskID = fmt.Sprintf("debt_task_%06d", s.nextID)
		s.nextID++
	}
	task.ID = taskID
	task.DebtID = debtID
	if task.Status == "" {
		task.Status = "open"
	}
	if _, ok := s.debtTasks[debtID]; !ok {
		s.debtTasks[debtID] = map[string]DebtTask{}
	}
	s.debtTasks[debtID][taskID] = task
	return task
}

func (s *Service) ListTemplates(workspaceID string) []ProjectTemplate {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]ProjectTemplate, 0, len(s.templates))
	for _, template := range s.templates {
		if workspaceID != "" && template.WorkspaceID != workspaceID {
			continue
		}
		out = append(out, template)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].ID < out[j].ID
	})
	return out
}

func (s *Service) AddTemplate(template ProjectTemplate) ProjectTemplate {
	s.mu.Lock()
	defer s.mu.Unlock()
	template.ID = fmt.Sprintf("template_%06d", s.nextID)
	s.nextID++
	if template.WorkspaceID == "" {
		template.WorkspaceID = "default"
	}
	if template.Status == "" {
		template.Status = "active"
	}
	s.templates[template.ID] = template
	return template
}

func (s *Service) CreateContextExport(export ContextExport) ContextExport {
	s.mu.Lock()
	defer s.mu.Unlock()
	export.ID = fmt.Sprintf("context_export_%06d", s.nextID)
	s.nextID++
	if export.WorkspaceID == "" {
		export.WorkspaceID = "default"
	}
	if export.Status == "" {
		export.Status = "completed"
	}
	if export.Format == "" {
		export.Format = "markdown"
	}
	s.exports[export.ID] = export
	return export
}

func (s *Service) GetContextExport(id string) (ContextExport, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	export, ok := s.exports[id]
	return export, ok
}
