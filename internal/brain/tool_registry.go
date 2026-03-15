package brain

import "github.com/brevio/brevio/internal/llm"

// DefaultToolRegistry returns the standard Brevio tool definitions.
func DefaultToolRegistry() []llm.ToolDefinition {
	mkSchema := func(required []string, props map[string]string) map[string]any {
		p := make(map[string]any, len(props))
		for k, t := range props {
			p[k] = map[string]any{"type": t}
		}
		return map[string]any{
			"type":                 "object",
			"required":             required,
			"properties":           p,
			"additionalProperties": false,
		}
	}
	return []llm.ToolDefinition{
		{Name: "email_read", Description: "Read/search emails from the user's mailbox.",
			InputSchema: mkSchema([]string{"workspace_id", "query"}, map[string]string{"workspace_id": "string", "query": "string"})},
		{Name: "email_send", Description: "Send an email. CRITICAL risk. Requires prior confirmation.",
			InputSchema: mkSchema([]string{"workspace_id", "to", "subject", "body"}, map[string]string{"workspace_id": "string", "to": "string", "subject": "string", "body": "string"})},
		{Name: "calendar_read", Description: "Read calendar events and check availability.",
			InputSchema: mkSchema([]string{"workspace_id"}, map[string]string{"workspace_id": "string", "date_range": "string"})},
		{Name: "calendar_write", Description: "Create or update a calendar event. Elevated risk.",
			InputSchema: mkSchema([]string{"workspace_id", "title", "start_time", "end_time"}, map[string]string{"workspace_id": "string", "title": "string", "start_time": "string", "end_time": "string", "attendees": "string"})},
		{Name: "document_read", Description: "Read or search documents, files, and notes.",
			InputSchema: mkSchema([]string{"workspace_id", "query"}, map[string]string{"workspace_id": "string", "query": "string"})},
		{Name: "document_write", Description: "Create or update a document. Elevated risk.",
			InputSchema: mkSchema([]string{"workspace_id", "title", "content"}, map[string]string{"workspace_id": "string", "title": "string", "content": "string"})},
		{Name: "web_search", Description: "Search the web for current information.",
			InputSchema: mkSchema([]string{"query"}, map[string]string{"query": "string"})},
		{Name: "verify_output", Description: "Verify that a previous action succeeded.",
			InputSchema: mkSchema([]string{"workspace_id"}, map[string]string{"workspace_id": "string", "action_ref": "string"})},
	}
}
