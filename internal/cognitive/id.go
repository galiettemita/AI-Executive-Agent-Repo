package cognitive

import "github.com/brevio/brevio/internal/determinism"

// newID returns a new UUIDv7 string for use as an entity identifier.
func newID() string {
	id, _ := determinism.NewUUIDv7()
	return id.String()
}
