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
		{Code: "ACCESS_DENIED", Category: "authz", Severity: "medium", Retryable: false, UserMessage: "You do not have permission for this action."},
		{Code: "ADMIN_ACTION_AUDIT", Category: "admin", Severity: "low", Retryable: false, UserMessage: "Admin action is allowed and has been audited."},
		{Code: "BUDGET_CALLS_EXHAUSTED", Category: "budget", Severity: "high", Retryable: false, UserMessage: "Monthly budget limit reached."},
		{Code: "CACHE_ENTRY_TOO_LARGE", Category: "cache", Severity: "medium", Retryable: false, UserMessage: "Cache entry exceeds the size limit."},
		{Code: "CONFLICT_REQUIRES_MANUAL_REVIEW", Category: "crdt", Severity: "medium", Retryable: false, UserMessage: "Conflict needs operator review before continuing."},
		{Code: "CONTEXT_BUDGET_EXCEEDED", Category: "context", Severity: "medium", Retryable: true, UserMessage: "Request context exceeded current budget."},
		{Code: "DSR_SLA_AT_RISK", Category: "compliance", Severity: "high", Retryable: false, UserMessage: "DSR processing is at risk of missing SLA."},
		{Code: "EVENT_SCHEMA_INVALID", Category: "events", Severity: "medium", Retryable: false, UserMessage: "Event payload failed schema validation."},
		{Code: "EVIDENCE_HASH_MISSING", Category: "compliance", Severity: "high", Retryable: false, UserMessage: "Compliance evidence is missing a required hash."},
		{Code: "EXPORT_RATE_LIMIT", Category: "codebase", Severity: "medium", Retryable: true, UserMessage: "Code context export limit reached for this workspace."},
		{Code: "FEATURE_DISABLED", Category: "feature_flag", Severity: "low", Retryable: false, UserMessage: "Feature is disabled for this workspace."},
		{Code: "FIRST_BYTE_SLA_BREACH", Category: "streaming", Severity: "low", Retryable: true, UserMessage: "Streaming first-byte latency exceeded SLA."},
		{Code: "GOAL_RATE_LIMIT", Category: "goals", Severity: "medium", Retryable: true, UserMessage: "Goal creation limit reached for today."},
		{Code: "GUARDRAIL_BLOCK_ACTIVE", Category: "guardrails", Severity: "high", Retryable: false, UserMessage: "Request was blocked for safety reasons."},
		{Code: "INVALID_REQUEST", Category: "request", Severity: "low", Retryable: false, UserMessage: "Request payload is invalid."},
		{Code: "INVALID_REQUEST_JSON", Category: "request", Severity: "low", Retryable: false, UserMessage: "Request JSON body is invalid."},
		{Code: "LESSON_CAP_REACHED", Category: "learning", Severity: "medium", Retryable: false, UserMessage: "Learning lesson cap reached for this workspace."},
		{Code: "MAX_STEPS_REACHED", Category: "react", Severity: "low", Retryable: true, UserMessage: "Execution reached max ReAct steps and returned partial results."},
		{Code: "METHOD_NOT_ALLOWED", Category: "request", Severity: "low", Retryable: false, UserMessage: "Method is not allowed for this endpoint."},
		{Code: "MODEL_TIER_EXCEEDED", Category: "model_tier", Severity: "medium", Retryable: false, UserMessage: "Requested model tier exceeds workspace policy."},
		{Code: "PII_ENCRYPTION_REQUIRED", Category: "security", Severity: "high", Retryable: false, UserMessage: "PII encryption policy requires encrypted handling."},
		{Code: "PROMOTION_EXCEEDS_SYSTEM_CAP", Category: "trust", Severity: "medium", Retryable: false, UserMessage: "Autonomy promotion exceeds system-wide cap."},
		{Code: "RAG_BUDGET_EXCEEDED", Category: "rag", Severity: "medium", Retryable: true, UserMessage: "RAG token budget exceeded."},
		{Code: "RATE_LIMIT_EXCEEDED", Category: "rate_limit", Severity: "medium", Retryable: true, UserMessage: "Request rate limit exceeded."},
		{Code: "RESOURCE_NOT_FOUND", Category: "request", Severity: "low", Retryable: false, UserMessage: "Requested resource was not found."},
		{Code: "SANDBOX_VIOLATION", Category: "security", Severity: "high", Retryable: false, UserMessage: "Sandbox policy blocked the operation."},
		{Code: "SELF_MODIFICATION_DENIED", Category: "self_modification", Severity: "medium", Retryable: false, UserMessage: "Self-modification action was denied by policy."},
		{Code: "SESSION_EXPIRED", Category: "session", Severity: "low", Retryable: true, UserMessage: "Session expired. Start a new session and retry."},
		{Code: "TEMPORAL_CONSTRAINT_VIOLATION", Category: "temporal", Severity: "medium", Retryable: false, UserMessage: "Temporal constraints prevent this schedule."},
		{Code: "TOOL_QUARANTINED", Category: "tool_health", Severity: "high", Retryable: true, UserMessage: "Tool is temporarily quarantined."},
		{Code: "UNAUTHORIZED", Category: "authn", Severity: "medium", Retryable: false, UserMessage: "Authentication is required."},
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
	case "CONTEXT_BUDGET_EXCEEDED", "RAG_BUDGET_EXCEEDED":
		return "Reduce requested context or retry with a narrower scope."
	case "FEATURE_DISABLED":
		return "Ask a workspace admin to enable this feature."
	case "GUARDRAIL_BLOCK_ACTIVE":
		return "Revise the request to comply with safety policies."
	case "TOOL_QUARANTINED":
		return "Retry shortly while the tool recovers or use a fallback tool."
	case "GOAL_RATE_LIMIT", "EXPORT_RATE_LIMIT", "RATE_LIMIT_EXCEEDED":
		return "Retry later after the current rate-limit window resets."
	case "METHOD_NOT_ALLOWED", "INVALID_REQUEST", "INVALID_REQUEST_JSON":
		return "Update the request method/payload and try again."
	case "SESSION_EXPIRED":
		return "Start a new session and retry the request."
	case "PROMOTION_EXCEEDS_SYSTEM_CAP", "SELF_MODIFICATION_DENIED":
		return "Request operator/admin review to proceed."
	case "PII_ENCRYPTION_REQUIRED", "SANDBOX_VIOLATION":
		return "Adjust the request to satisfy security controls."
	case "EVENT_SCHEMA_INVALID":
		return "Fix event payload schema before retry."
	case "CACHE_ENTRY_TOO_LARGE":
		return "Reduce cache payload size and retry."
	case "MAX_STEPS_REACHED":
		return "Refine the task and rerun with a narrower objective."
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
