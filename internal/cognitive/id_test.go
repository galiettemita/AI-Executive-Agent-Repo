package cognitive

import (
	"testing"
)

func TestNewIDReturnsNonEmpty(t *testing.T) {
	t.Parallel()

	id := newID()
	if id == "" {
		t.Fatal("expected non-empty ID")
	}
}

func TestNewIDReturnsUniqueValues(t *testing.T) {
	t.Parallel()

	ids := make(map[string]bool)
	for i := 0; i < 100; i++ {
		id := newID()
		if ids[id] {
			t.Fatalf("duplicate ID generated: %s", id)
		}
		ids[id] = true
	}
}

func TestNewIDHasValidFormat(t *testing.T) {
	t.Parallel()

	id := newID()
	// UUIDv7 should be 36 characters with hyphens.
	if len(id) != 36 {
		t.Fatalf("expected UUID length 36, got %d for %q", len(id), id)
	}
}
