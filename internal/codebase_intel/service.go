package codebase_intel

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

type Dependency struct {
	ID           string `json:"id"`
	WorkspaceID  string `json:"workspace_id"`
	RepositoryID string `json:"repository_id"`
	Name         string `json:"name"`
	Version      string `json:"version"`
	Category     string `json:"category"`
}

type Pattern struct {
	ID           string `json:"id"`
	WorkspaceID  string `json:"workspace_id"`
	RepositoryID string `json:"repository_id"`
	Name         string `json:"name"`
	Description  string `json:"description"`
	Scope        string `json:"scope"`
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
	ID                  string    `json:"id"`
	WorkspaceID         string    `json:"workspace_id"`
	RepositoryID        string    `json:"repository_id"`
	Scope               string    `json:"scope"`
	IncludeDependencies bool      `json:"include_dependencies"`
	Status              string    `json:"status"`
	Format              string    `json:"format"`
	CreatedAt           time.Time `json:"created_at"`
}

type RepositorySnapshot struct {
	WorkspaceID  string       `json:"workspace_id"`
	RepositoryID string       `json:"repository_id"`
	Dependencies []Dependency `json:"dependencies"`
	Patterns     []Pattern    `json:"patterns"`
	ScannedAt    time.Time    `json:"scanned_at"`
}

type SharedDependency struct {
	Name         string   `json:"name"`
	Version      string   `json:"version"`
	Repositories []string `json:"repositories"`
	Occurrences  int      `json:"occurrences"`
}

type SharedPattern struct {
	Name         string   `json:"name"`
	Repositories []string `json:"repositories"`
	Occurrences  int      `json:"occurrences"`
}

type CrossRepoReport struct {
	WorkspaceID        string             `json:"workspace_id"`
	GeneratedAt        time.Time          `json:"generated_at"`
	SharedDependencies []SharedDependency `json:"shared_dependencies"`
	SharedPatterns     []SharedPattern    `json:"shared_patterns"`
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
	repositories map[string]map[string]RepositorySnapshot
	reports      map[string]CrossRepoReport
}

func NewService() *Service {
	return &Service{
		nextID: 1,
		dependencies: []Dependency{
			{ID: "dep_001", WorkspaceID: "default", RepositoryID: "default_repo", Name: "pgx", Version: "v5", Category: "library"},
		},
		patterns: []Pattern{
			{ID: "pattern_001", WorkspaceID: "default", RepositoryID: "default_repo", Name: "deterministic_handlers", Description: "Pure deterministic request handlers", Scope: "cross_repo"},
		},
		debt:         map[string]DebtItem{},
		debtTasks:    map[string]map[string]DebtTask{},
		templates:    map[string]ProjectTemplate{},
		exports:      map[string]ContextExport{},
		repositories: map[string]map[string]RepositorySnapshot{},
		reports:      map[string]CrossRepoReport{},
	}
}

func (s *Service) ListDependencies(workspaceID string) []Dependency {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if workspaceID == "" {
		workspaceID = "default"
	}
	out := make([]Dependency, 0, len(s.dependencies))
	for _, dep := range s.dependencies {
		if dep.WorkspaceID != workspaceID && dep.WorkspaceID != "default" {
			continue
		}
		out = append(out, dep)
	}
	repoDeps := s.dependenciesFromSnapshotsLocked(workspaceID)
	out = append(out, repoDeps...)
	sort.Slice(out, func(i, j int) bool {
		left := out[i].Name + "|" + out[i].Version + "|" + out[i].RepositoryID + "|" + out[i].ID
		right := out[j].Name + "|" + out[j].Version + "|" + out[j].RepositoryID + "|" + out[j].ID
		return left < right
	})
	return out
}

func (s *Service) ListPatterns(workspaceID string) []Pattern {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if workspaceID == "" {
		workspaceID = "default"
	}
	out := make([]Pattern, 0, len(s.patterns))
	for _, pattern := range s.patterns {
		if pattern.WorkspaceID != workspaceID && pattern.WorkspaceID != "default" {
			continue
		}
		out = append(out, pattern)
	}
	repoPatterns := s.patternsFromSnapshotsLocked(workspaceID)
	out = append(out, repoPatterns...)
	sort.Slice(out, func(i, j int) bool {
		left := out[i].Name + "|" + out[i].RepositoryID + "|" + out[i].ID
		right := out[j].Name + "|" + out[j].RepositoryID + "|" + out[j].ID
		return left < right
	})
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
	template.Status = normalizeTemplateStatus(template.Status)
	s.templates[template.ID] = template
	return template
}

func (s *Service) CreateContextExport(export ContextExport) ContextExport {
	created, _ := s.CreateContextExportStrict(export)
	return created
}

func (s *Service) CreateContextExportStrict(export ContextExport) (ContextExport, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	workspaceID := normalizeWorkspace(export.WorkspaceID)
	createdToday := s.contextExportsTodayLocked(workspaceID, time.Now().UTC())
	if createdToday >= 10 {
		return ContextExport{}, fmt.Errorf("EXPORT_RATE_LIMIT")
	}

	export.ID = fmt.Sprintf("context_export_%06d", s.nextID)
	s.nextID++
	export.WorkspaceID = workspaceID
	if export.RepositoryID == "" {
		export.RepositoryID = "primary"
	}
	scope, ok := normalizeExportScope(export.Scope)
	if !ok {
		return ContextExport{}, fmt.Errorf("invalid context export scope")
	}
	export.Scope = scope
	if export.Status == "" {
		export.Status = "completed"
	}
	format, ok := normalizeExportFormat(export.Format)
	if !ok {
		return ContextExport{}, fmt.Errorf("invalid context export format")
	}
	export.Format = format
	export.CreatedAt = time.Now().UTC()
	s.exports[export.ID] = export
	return export, nil
}

func (s *Service) GetContextExport(id string) (ContextExport, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	export, ok := s.exports[id]
	return export, ok
}

func (s *Service) IngestRepository(workspaceID, repositoryID string, dependencies []Dependency, patterns []Pattern) RepositorySnapshot {
	s.mu.Lock()
	defer s.mu.Unlock()

	workspaceID = normalizeWorkspace(workspaceID)
	repositoryID = normalizeRepository(repositoryID)
	s.ensureWorkspaceRepoLocked(workspaceID)

	normalizedDependencies := make([]Dependency, 0, len(dependencies))
	for _, dep := range dependencies {
		name := strings.TrimSpace(dep.Name)
		if name == "" {
			continue
		}
		version := strings.TrimSpace(dep.Version)
		if version == "" {
			version = "latest"
		}
		category := strings.TrimSpace(dep.Category)
		if category == "" {
			category = "library"
		}
		normalizedDependencies = append(normalizedDependencies, Dependency{
			ID:           fmt.Sprintf("dep_%06d", s.nextID),
			WorkspaceID:  workspaceID,
			RepositoryID: repositoryID,
			Name:         strings.ToLower(name),
			Version:      version,
			Category:     category,
		})
		s.nextID++
	}

	normalizedPatterns := make([]Pattern, 0, len(patterns))
	for _, pattern := range patterns {
		name := strings.TrimSpace(pattern.Name)
		if name == "" {
			continue
		}
		scope := strings.TrimSpace(pattern.Scope)
		if scope == "" {
			scope = "cross_repo"
		}
		normalizedPatterns = append(normalizedPatterns, Pattern{
			ID:           fmt.Sprintf("pattern_%06d", s.nextID),
			WorkspaceID:  workspaceID,
			RepositoryID: repositoryID,
			Name:         strings.ToLower(name),
			Description:  strings.TrimSpace(pattern.Description),
			Scope:        scope,
		})
		s.nextID++
	}

	sort.Slice(normalizedDependencies, func(i, j int) bool {
		left := normalizedDependencies[i].Name + "|" + normalizedDependencies[i].Version + "|" + normalizedDependencies[i].ID
		right := normalizedDependencies[j].Name + "|" + normalizedDependencies[j].Version + "|" + normalizedDependencies[j].ID
		return left < right
	})
	sort.Slice(normalizedPatterns, func(i, j int) bool {
		left := normalizedPatterns[i].Name + "|" + normalizedPatterns[i].ID
		right := normalizedPatterns[j].Name + "|" + normalizedPatterns[j].ID
		return left < right
	})

	snapshot := RepositorySnapshot{
		WorkspaceID:  workspaceID,
		RepositoryID: repositoryID,
		Dependencies: normalizedDependencies,
		Patterns:     normalizedPatterns,
		ScannedAt:    time.Now().UTC(),
	}
	s.repositories[workspaceID][repositoryID] = snapshot
	return snapshot
}

func (s *Service) AnalyzeCrossRepo(workspaceID string) CrossRepoReport {
	s.mu.Lock()
	defer s.mu.Unlock()

	workspaceID = normalizeWorkspace(workspaceID)
	s.ensureWorkspaceRepoLocked(workspaceID)

	dependencyRepos := map[string]map[string]struct{}{}
	patternRepos := map[string]map[string]struct{}{}
	repoIDs := make([]string, 0, len(s.repositories[workspaceID]))
	for repoID := range s.repositories[workspaceID] {
		repoIDs = append(repoIDs, repoID)
	}
	sort.Strings(repoIDs)

	for _, repoID := range repoIDs {
		snapshot := s.repositories[workspaceID][repoID]
		for _, dep := range snapshot.Dependencies {
			key := dep.Name + "|" + dep.Version
			if _, ok := dependencyRepos[key]; !ok {
				dependencyRepos[key] = map[string]struct{}{}
			}
			dependencyRepos[key][repoID] = struct{}{}
		}
		for _, pattern := range snapshot.Patterns {
			key := pattern.Name
			if _, ok := patternRepos[key]; !ok {
				patternRepos[key] = map[string]struct{}{}
			}
			patternRepos[key][repoID] = struct{}{}
		}
	}

	sharedDependencies := make([]SharedDependency, 0)
	for key, repos := range dependencyRepos {
		if len(repos) < 2 {
			continue
		}
		parts := strings.SplitN(key, "|", 2)
		sharedDependencies = append(sharedDependencies, SharedDependency{
			Name:         parts[0],
			Version:      parts[1],
			Repositories: sortedKeys(repos),
			Occurrences:  len(repos),
		})
	}
	sort.Slice(sharedDependencies, func(i, j int) bool {
		left := sharedDependencies[i].Name + "|" + sharedDependencies[i].Version
		right := sharedDependencies[j].Name + "|" + sharedDependencies[j].Version
		return left < right
	})

	sharedPatterns := make([]SharedPattern, 0)
	for patternName, repos := range patternRepos {
		if len(repos) < 2 {
			continue
		}
		sharedPatterns = append(sharedPatterns, SharedPattern{
			Name:         patternName,
			Repositories: sortedKeys(repos),
			Occurrences:  len(repos),
		})
	}
	sort.Slice(sharedPatterns, func(i, j int) bool {
		return sharedPatterns[i].Name < sharedPatterns[j].Name
	})

	report := CrossRepoReport{
		WorkspaceID:        workspaceID,
		GeneratedAt:        time.Now().UTC(),
		SharedDependencies: sharedDependencies,
		SharedPatterns:     sharedPatterns,
	}
	s.reports[workspaceID] = report
	return report
}

func (s *Service) GetCrossRepoReport(workspaceID string) CrossRepoReport {
	s.mu.RLock()
	defer s.mu.RUnlock()
	workspaceID = normalizeWorkspace(workspaceID)
	report, ok := s.reports[workspaceID]
	if !ok {
		return CrossRepoReport{
			WorkspaceID:        workspaceID,
			SharedDependencies: []SharedDependency{},
			SharedPatterns:     []SharedPattern{},
		}
	}
	return report
}

func (s *Service) dependenciesFromSnapshotsLocked(workspaceID string) []Dependency {
	snapshots := s.repositories[workspaceID]
	if len(snapshots) == 0 {
		return nil
	}
	out := make([]Dependency, 0)
	repoIDs := make([]string, 0, len(snapshots))
	for repoID := range snapshots {
		repoIDs = append(repoIDs, repoID)
	}
	sort.Strings(repoIDs)
	for _, repoID := range repoIDs {
		out = append(out, snapshots[repoID].Dependencies...)
	}
	return out
}

func (s *Service) patternsFromSnapshotsLocked(workspaceID string) []Pattern {
	snapshots := s.repositories[workspaceID]
	if len(snapshots) == 0 {
		return nil
	}
	out := make([]Pattern, 0)
	repoIDs := make([]string, 0, len(snapshots))
	for repoID := range snapshots {
		repoIDs = append(repoIDs, repoID)
	}
	sort.Strings(repoIDs)
	for _, repoID := range repoIDs {
		out = append(out, snapshots[repoID].Patterns...)
	}
	return out
}

func (s *Service) ensureWorkspaceRepoLocked(workspaceID string) {
	if _, ok := s.repositories[workspaceID]; !ok {
		s.repositories[workspaceID] = map[string]RepositorySnapshot{}
	}
}

func normalizeWorkspace(workspaceID string) string {
	if strings.TrimSpace(workspaceID) == "" {
		return "default"
	}
	return workspaceID
}

func normalizeRepository(repositoryID string) string {
	if strings.TrimSpace(repositoryID) == "" {
		return "primary"
	}
	return repositoryID
}

func sortedKeys(values map[string]struct{}) []string {
	out := make([]string, 0, len(values))
	for key := range values {
		out = append(out, key)
	}
	sort.Strings(out)
	return out
}

func normalizeExportScope(scope string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(scope)) {
	case "":
		return "workspace", true
	case "workspace":
		return "workspace", true
	case "repository":
		return "repository", true
	case "cross_repo":
		return "cross_repo", true
	default:
		return "", false
	}
}

func normalizeExportFormat(format string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "":
		return "markdown", true
	case "markdown":
		return "markdown", true
	case "json":
		return "json", true
	case "yaml":
		return "yaml", true
	default:
		return "", false
	}
}

func normalizeTemplateStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "draft":
		return "draft"
	case "archived":
		return "archived"
	case "active":
		return "active"
	default:
		return "active"
	}
}

func (s *Service) contextExportsTodayLocked(workspaceID string, now time.Time) int {
	today := now.Format("2006-01-02")
	count := 0
	for _, export := range s.exports {
		if export.WorkspaceID != workspaceID {
			continue
		}
		if export.CreatedAt.Format("2006-01-02") == today {
			count++
		}
	}
	return count
}
