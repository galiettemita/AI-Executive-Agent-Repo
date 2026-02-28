package event_schemas

import "testing"

func TestEventSchemaLifecycle(t *testing.T) {
	t.Parallel()

	s := NewService()

	registered, err := s.RegisterVersionStrict("BREVIO.test.event.v1", `{"type":"object","properties":{"type":{"type":"string"},"version":{"type":"integer"}},"required":["type","version"],"additionalProperties":true}`, "active")
	if err != nil {
		t.Fatalf("register strict schema: %v", err)
	}
	if registered.Version != 1 {
		t.Fatalf("expected version 1 registration, got %#v", registered)
	}

	types := s.ListTypes()
	if len(types) != 1 {
		t.Fatalf("expected one event type, got %d", len(types))
	}

	versions := s.ListVersions("BREVIO.test.event.v1")
	if len(versions) != 1 {
		t.Fatalf("expected one version, got %d", len(versions))
	}

	valid := s.Validate("BREVIO.test.event.v1", map[string]any{
		"type":    "BREVIO.test.event.v1",
		"version": 1,
	})
	if !valid.Valid {
		t.Fatalf("expected validation success, got %#v", valid)
	}

	invalid := s.Validate("BREVIO.test.event.v1", map[string]any{
		"type": "BREVIO.other.event.v1",
	})
	if invalid.Valid {
		t.Fatalf("expected validation failure, got %#v", invalid)
	}
}

func TestRegisterVersionStrictRejectsBreakingChange(t *testing.T) {
	t.Parallel()

	s := NewService()
	_, err := s.RegisterVersionStrict("BREVIO.compat.event.v1", `{"type":"object","properties":{"type":{"type":"string"},"version":{"type":"integer"},"entity_id":{"type":"string"}},"required":["type","version","entity_id"],"additionalProperties":true}`, "active")
	if err != nil {
		t.Fatalf("unexpected initial registration error: %v", err)
	}

	_, err = s.RegisterVersionStrict("BREVIO.compat.event.v1", `{"type":"object","properties":{"type":{"type":"string"},"version":{"type":"integer"},"entity_id":{"type":"integer"}},"required":["type","version"],"additionalProperties":true}`, "active")
	if err == nil {
		t.Fatal("expected breaking-change registration error")
	}
}

func TestValidateEnforcesRequiredAndAdditionalProperties(t *testing.T) {
	t.Parallel()

	s := NewService()
	_, err := s.RegisterVersionStrict("BREVIO.strict.event.v1", `{"type":"object","properties":{"type":{"type":"string"},"version":{"type":"integer"},"workspace_id":{"type":"string"}},"required":["type","version","workspace_id"],"additionalProperties":false}`, "active")
	if err != nil {
		t.Fatalf("register strict schema: %v", err)
	}

	missingRequired := s.Validate("BREVIO.strict.event.v1", map[string]any{
		"type":    "BREVIO.strict.event.v1",
		"version": 1,
	})
	if missingRequired.Valid {
		t.Fatalf("expected required-field validation failure: %#v", missingRequired)
	}

	unknownField := s.Validate("BREVIO.strict.event.v1", map[string]any{
		"type":         "BREVIO.strict.event.v1",
		"version":      1,
		"workspace_id": "ws_1",
		"extra":        "unexpected",
	})
	if unknownField.Valid {
		t.Fatalf("expected unknown-field validation failure: %#v", unknownField)
	}

	good := s.Validate("BREVIO.strict.event.v1", map[string]any{
		"type":         "BREVIO.strict.event.v1",
		"version":      1,
		"workspace_id": "ws_1",
	})
	if !good.Valid {
		t.Fatalf("expected strict schema validation success: %#v", good)
	}
}
