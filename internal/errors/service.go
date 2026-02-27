package errorlayer

import (
	"fmt"
	"sort"
	"sync"
)

type TaxonomyItem struct {
	Code        string `json:"code"`
	Category    string `json:"category"`
	Severity    string `json:"severity"`
	Retryable   bool   `json:"retryable"`
	UserMessage string `json:"user_message"`
}

type Template struct {
	ID          string `json:"id"`
	WorkspaceID string `json:"workspace_id"`
	Persona     string `json:"persona"`
	CodePattern string `json:"code_pattern"`
	Template    string `json:"template"`
	Status      string `json:"status"`
}

type Service struct {
	mu        sync.RWMutex
	nextID    int
	templates map[string]Template
}

func NewService() *Service {
	return &Service{
		nextID:    1,
		templates: map[string]Template{},
	}
}

func (s *Service) ListTaxonomy() []TaxonomyItem {
	items := []TaxonomyItem{
		{Code: "BUDGET_CALLS_EXHAUSTED", Category: "budget", Severity: "high", Retryable: false, UserMessage: "Monthly budget limit reached."},
		{Code: "CONTEXT_BUDGET_EXCEEDED", Category: "context", Severity: "medium", Retryable: true, UserMessage: "Request context exceeded current budget."},
		{Code: "FEATURE_DISABLED", Category: "feature_flag", Severity: "low", Retryable: false, UserMessage: "Feature is disabled for this workspace."},
		{Code: "GUARDRAIL_BLOCK_ACTIVE", Category: "guardrails", Severity: "high", Retryable: false, UserMessage: "Blocked for safety reasons."},
		{Code: "TOOL_QUARANTINED", Category: "tool_health", Severity: "high", Retryable: true, UserMessage: "Tool is temporarily unavailable."},
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].Code < items[j].Code
	})
	return items
}

func (s *Service) UpsertTemplate(template Template) Template {
	s.mu.Lock()
	defer s.mu.Unlock()

	if template.ID == "" {
		template.ID = fmt.Sprintf("error_template_%06d", s.nextID)
		s.nextID++
	}
	if template.WorkspaceID == "" {
		template.WorkspaceID = "default"
	}
	if template.Persona == "" {
		template.Persona = "default"
	}
	if template.Status == "" {
		template.Status = "active"
	}
	s.templates[template.ID] = template
	return template
}

func (s *Service) ListTemplates(workspaceID string) []Template {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]Template, 0, len(s.templates))
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
