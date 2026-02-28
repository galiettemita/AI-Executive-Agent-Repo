package errorlayer

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"
)

var (
	uuidPattern        = regexp.MustCompile(`\b[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[1-5][0-9a-fA-F]{3}-[89abAB][0-9a-fA-F]{3}-[0-9a-fA-F]{12}\b`)
	internalRefPattern = regexp.MustCompile(`\b(?:trace|span|workflow|request|internal|execution|job)_[A-Za-z0-9]{6,}\b`)
	hexTokenPattern    = regexp.MustCompile(`\b[a-fA-F0-9]{24,}\b`)
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

type Message struct {
	ErrorCode   string `json:"error_code"`
	UserMessage string `json:"user_message"`
	Retryable   bool   `json:"retryable"`
	NextAction  string `json:"next_action"`
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
	template.WorkspaceID = normalizeWorkspaceID(template.WorkspaceID)
	template.Persona = normalizePersona(template.Persona)
	if strings.TrimSpace(template.CodePattern) == "" {
		template.CodePattern = "*"
	}
	template.Template = normalizeUserMessage(template.Template)
	if template.Status == "" {
		template.Status = "active"
	}
	s.templates[template.ID] = template
	return template
}

func (s *Service) ListTemplates(workspaceID string) []Template {
	s.mu.RLock()
	defer s.mu.RUnlock()

	workspaceID = normalizeWorkspaceID(workspaceID)
	out := make([]Template, 0, len(s.templates))
	for _, template := range s.templates {
		if template.WorkspaceID != workspaceID {
			continue
		}
		out = append(out, template)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].ID < out[j].ID
	})
	return out
}

func (s *Service) RenderMessage(workspaceID, persona, errorCode, details string) Message {
	workspaceID = normalizeWorkspaceID(workspaceID)
	persona = normalizePersona(persona)
	errorCode = strings.TrimSpace(errorCode)

	taxonomyItem, hasTaxonomy := s.taxonomyByCode()[errorCode]
	message := Message{
		ErrorCode:   errorCode,
		UserMessage: "An unexpected issue occurred.",
		Retryable:   false,
		NextAction:  "Try again shortly or contact support.",
	}
	if hasTaxonomy {
		message.UserMessage = taxonomyItem.UserMessage
		message.Retryable = taxonomyItem.Retryable
		message.NextAction = nextActionFor(taxonomyItem.Code, taxonomyItem.Retryable)
	}

	if template, ok := s.findBestTemplate(workspaceID, persona, errorCode); ok {
		message.UserMessage = template.Template
	}

	sanitizedDetails := sanitizeDetails(details)
	if sanitizedDetails != "" {
		message.UserMessage = strings.TrimSpace(message.UserMessage + " " + sanitizedDetails)
	}
	message.UserMessage = normalizeUserMessage(message.UserMessage)
	if message.ErrorCode == "" {
		message.ErrorCode = "UNKNOWN_ERROR"
	}
	return message
}

func (s *Service) taxonomyByCode() map[string]TaxonomyItem {
	items := s.ListTaxonomy()
	out := make(map[string]TaxonomyItem, len(items))
	for _, item := range items {
		out[item.Code] = item
	}
	return out
}

func (s *Service) findBestTemplate(workspaceID, persona, errorCode string) (Template, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	type candidate struct {
		template Template
		score    int
	}
	candidates := make([]candidate, 0, len(s.templates))
	for _, template := range s.templates {
		if !strings.EqualFold(template.Status, "active") {
			continue
		}
		if !patternMatches(template.CodePattern, errorCode) {
			continue
		}
		score := matchScore(template, workspaceID, persona)
		if score < 0 {
			continue
		}
		candidates = append(candidates, candidate{template: template, score: score})
	}
	if len(candidates) == 0 {
		return Template{}, false
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].score == candidates[j].score {
			return candidates[i].template.ID < candidates[j].template.ID
		}
		return candidates[i].score > candidates[j].score
	})
	return candidates[0].template, true
}

func matchScore(template Template, workspaceID, persona string) int {
	workspaceMatch := template.WorkspaceID == workspaceID
	workspaceDefault := template.WorkspaceID == "default"
	if !workspaceMatch && !workspaceDefault {
		return -1
	}
	personaMatch := template.Persona == persona
	personaDefault := template.Persona == "default"
	if !personaMatch && !personaDefault {
		return -1
	}

	score := 0
	if workspaceMatch {
		score += 4
	} else {
		score += 2
	}
	if personaMatch {
		score += 2
	} else {
		score += 1
	}
	if template.CodePattern == "*" {
		score += 0
	} else if strings.Contains(template.CodePattern, "*") {
		score += 1
	} else {
		score += 2
	}
	return score
}

func patternMatches(pattern, code string) bool {
	pattern = strings.TrimSpace(pattern)
	code = strings.TrimSpace(code)
	if pattern == "" || pattern == "*" {
		return true
	}
	if strings.Contains(pattern, "*") {
		prefix := strings.TrimSuffix(pattern, "*")
		return strings.HasPrefix(code, prefix)
	}
	return pattern == code
}

func nextActionFor(errorCode string, retryable bool) string {
	switch errorCode {
	case "BUDGET_CALLS_EXHAUSTED":
		return "Review budget settings or request a limit increase approval."
	case "FEATURE_DISABLED":
		return "Ask a workspace admin to enable this feature."
	case "GUARDRAIL_BLOCK_ACTIVE":
		return "Revise the request to comply with safety policies."
	case "TOOL_QUARANTINED":
		return "Retry shortly while the tool recovers or use a fallback tool."
	}
	if retryable {
		return "Retry in a few moments."
	}
	return "Review the request and retry with a safer configuration."
}

func normalizeWorkspaceID(workspaceID string) string {
	if strings.TrimSpace(workspaceID) == "" {
		return "default"
	}
	return workspaceID
}

func normalizePersona(persona string) string {
	if strings.TrimSpace(persona) == "" {
		return "default"
	}
	return strings.ToLower(strings.TrimSpace(persona))
}

func normalizeUserMessage(message string) string {
	message = strings.TrimSpace(message)
	message = sanitizeDetails(message)
	if message == "" {
		message = "An unexpected issue occurred."
	}
	if len(message) > 700 {
		message = strings.TrimSpace(message[:700])
	}
	return message
}

func sanitizeDetails(details string) string {
	details = strings.TrimSpace(details)
	if details == "" {
		return ""
	}
	details = uuidPattern.ReplaceAllString(details, "[redacted]")
	details = internalRefPattern.ReplaceAllString(details, "[redacted]")
	details = hexTokenPattern.ReplaceAllString(details, "[redacted]")
	details = strings.Join(strings.Fields(details), " ")
	return details
}
