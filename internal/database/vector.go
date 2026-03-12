package database

import pgvector "github.com/pgvector/pgvector-go"

// ToVector converts a []float32 slice to a pgvector.Vector for type-safe
// insertion into vector(1536) columns via pgx.
func ToVector(f []float32) pgvector.Vector {
	return pgvector.NewVector(f)
}

// FromVector extracts the underlying []float32 from a pgvector.Vector.
func FromVector(v pgvector.Vector) []float32 {
	return v.Slice()
}

// NullableVector returns a *pgvector.Vector suitable for nullable embedding
// columns. Returns nil when the input slice is empty.
func NullableVector(f []float32) *pgvector.Vector {
	if len(f) == 0 {
		return nil
	}
	v := pgvector.NewVector(f)
	return &v
}
