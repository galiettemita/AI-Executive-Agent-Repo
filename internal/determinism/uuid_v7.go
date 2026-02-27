package determinism

import "github.com/google/uuid"

func NewUUIDv7() (uuid.UUID, error) {
	return uuid.NewV7()
}
