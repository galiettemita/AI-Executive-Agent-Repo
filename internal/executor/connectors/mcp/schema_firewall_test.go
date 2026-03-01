package mcp

import "testing"

func TestApplySchemaFirewall(t *testing.T) {
	t.Parallel()

	filtered := ApplySchemaFirewall(
		map[string]any{"id": "123", "status": "ok", "unregistered": "strip_me"},
		[]string{"id", "status"},
		50000,
	)
	if _, ok := filtered["unregistered"]; ok {
		t.Fatalf("expected unregistered field removal: %+v", filtered)
	}

	oversized := ApplySchemaFirewall(map[string]any{"payload": string(make([]byte, 8000))}, []string{"payload"}, 100)
	if oversized["error"] != "schema_firewall_response_too_large" {
		t.Fatalf("expected oversized response guard, got %+v", oversized)
	}
}
