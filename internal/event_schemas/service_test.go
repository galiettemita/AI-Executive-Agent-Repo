package event_schemas

import "testing"

func TestEventSchemaLifecycle(t *testing.T) {
	s := NewService()

	registered := s.RegisterVersion("BREVIO.test.event.v1", `{"type":"object"}`, "active")
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
