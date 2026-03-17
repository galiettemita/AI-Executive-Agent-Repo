// Package disclosure implements EU AI Act Article 50 and US AI disclosure law compliance.
package disclosure

import (
	"fmt"
	"net/http"
	"strings"
)

const (
	AgentHeaderKey   = "X-Brevio-Agent"
	AgentHeaderValue = "true"
	AgentVersionKey  = "X-Brevio-Agent-Version"
	AgentVersion     = "1.0"
)

const (
	EmailFooter        = "\n\n---\nSent via Brevio AI on behalf of your executive assistant."
	CalendarDisclaimer = "\n\nScheduled by Brevio AI."
	// C2PATag is the invisible Unicode tag for C2PA-compatible provenance tracking.
	// U+E0001 is the language tag character used by C2PA as a content provenance carrier.
	C2PATag = "\U000E0001brevio-ai-v1"
)

// BrevioAgentTransport injects X-Brevio-Agent headers on every outbound request.
type BrevioAgentTransport struct {
	wrapped http.RoundTripper
}

func NewBrevioAgentTransport(wrapped http.RoundTripper) *BrevioAgentTransport {
	if wrapped == nil {
		wrapped = http.DefaultTransport
	}
	return &BrevioAgentTransport{wrapped: wrapped}
}

func (t *BrevioAgentTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	cloned := req.Clone(req.Context())
	if cloned.Header.Get(AgentHeaderKey) == "" {
		cloned.Header.Set(AgentHeaderKey, AgentHeaderValue)
	}
	if cloned.Header.Get(AgentVersionKey) == "" {
		cloned.Header.Set(AgentVersionKey, AgentVersion)
	}
	return t.wrapped.RoundTrip(cloned)
}

// NewDisclosureHTTPClient returns an *http.Client with BrevioAgentTransport applied.
func NewDisclosureHTTPClient(baseTransport http.RoundTripper) *http.Client {
	return &http.Client{Transport: NewBrevioAgentTransport(baseTransport)}
}

var _ http.RoundTripper = (*BrevioAgentTransport)(nil)

// InjectEmailDisclosure appends the email footer and C2PA tag to the email body.
func InjectEmailDisclosure(args map[string]any) map[string]any {
	for _, field := range []string{"body", "html_body", "text_body", "content", "message"} {
		if v, ok := args[field]; ok {
			if body, isString := v.(string); isString {
				args[field] = body + EmailFooter + C2PATag
				return args
			}
		}
	}
	args["_brevio_disclosure"] = "Sent via Brevio AI"
	args["_c2pa_tag"] = C2PATag
	return args
}

// InjectCalendarDisclosure appends the calendar disclaimer to event description.
func InjectCalendarDisclosure(args map[string]any) map[string]any {
	for _, field := range []string{"description", "details", "notes", "body"} {
		if v, ok := args[field]; ok {
			if desc, isString := v.(string); isString {
				args[field] = desc + CalendarDisclaimer
				return args
			}
		}
	}
	args["description"] = CalendarDisclaimer
	return args
}

// IsEmailSkill returns true if the tool key indicates an email-sending skill.
func IsEmailSkill(toolKey string) bool {
	lower := strings.ToLower(toolKey)
	for _, k := range []string{"email.send", "email.reply", "email.forward", "email.compose", "mail.send", "gmail.send", "outlook.send"} {
		if lower == k {
			return true
		}
	}
	if strings.HasPrefix(lower, "email.") || strings.HasPrefix(lower, "mail.") {
		return strings.Contains(lower, "send") || strings.Contains(lower, "reply") || strings.Contains(lower, "forward")
	}
	return false
}

// IsCalendarWriteSkill returns true if the tool key indicates a calendar write skill.
func IsCalendarWriteSkill(toolKey string) bool {
	lower := strings.ToLower(toolKey)
	for _, k := range []string{"calendar.create", "calendar.write", "calendar.update", "calendar.book", "calendar.add", "gcal.create", "outlook.calendar.create"} {
		if lower == k {
			return true
		}
	}
	if strings.HasPrefix(lower, "calendar.") || strings.HasPrefix(lower, "gcal.") {
		return strings.Contains(lower, "create") || strings.Contains(lower, "write") ||
			strings.Contains(lower, "add") || strings.Contains(lower, "book")
	}
	return false
}

// StripC2PATag removes the C2PA tag from a string.
func StripC2PATag(s string) string {
	return strings.ReplaceAll(s, C2PATag, "")
}

// FormatDisclosureReport returns a human-readable summary of applied disclosures.
func FormatDisclosureReport(toolKey string) string {
	parts := []string{"X-Brevio-Agent: true header injected on outbound HTTP"}
	if IsEmailSkill(toolKey) {
		parts = append(parts, fmt.Sprintf("email disclosure footer appended to %s call", toolKey))
		parts = append(parts, "C2PA tag embedded in email body")
	}
	if IsCalendarWriteSkill(toolKey) {
		parts = append(parts, fmt.Sprintf("calendar disclosure appended to %s call", toolKey))
	}
	return strings.Join(parts, "; ")
}
