package mcp

import (
	"encoding/json"
	"strings"
)

const defaultMaxResponseLen = 50000

// ApplySchemaFirewall strips unregistered fields and enforces max response length.
func ApplySchemaFirewall(response map[string]any, allowedFields []string, maxResponseLen int) map[string]any {
	if maxResponseLen <= 0 {
		maxResponseLen = defaultMaxResponseLen
	}
	allowed := map[string]struct{}{}
	for _, field := range allowedFields {
		allowed[strings.TrimSpace(field)] = struct{}{}
	}

	filtered := map[string]any{}
	for key, value := range response {
		if _, ok := allowed[key]; !ok {
			continue
		}
		filtered[key] = value
	}

	blob, err := json.Marshal(filtered)
	if err != nil || len(blob) <= maxResponseLen {
		return filtered
	}
	// Deterministically collapse oversized payloads to prevent downstream model abuse.
	return map[string]any{
		"error":   "schema_firewall_response_too_large",
		"max_len": maxResponseLen,
	}
}
