package kg

import "time"

// Triple is a subject-predicate-object knowledge triple.
type Triple struct {
	ID           string    `json:"id"           db:"id"`
	WorkspaceID  string    `json:"workspace_id" db:"workspace_id"`
	Subject      string    `json:"subject"      db:"subject"`
	Predicate    string    `json:"predicate"    db:"predicate"`
	Object       string    `json:"object"       db:"object"`
	SubjectType  string    `json:"subject_type" db:"subject_type"`
	ObjectType   string    `json:"object_type"  db:"object_type"`
	Confidence   float64   `json:"confidence"   db:"confidence"`
	SourceTurnID string    `json:"source_turn_id,omitempty" db:"source_turn_id"`
	CreatedAt    time.Time `json:"created_at"   db:"created_at"`

	HopDistance    int     `json:"hop_distance,omitempty"    db:"-"`
	TraversalScore float64 `json:"traversal_score,omitempty" db:"-"`
}

// ExtractionRequest is the input to LLM triple extraction.
type ExtractionRequest struct {
	WorkspaceID string
	TurnID      string
	Content     string
	Role        string
}

// KGQueryResult is returned by the graph retrieval path.
type KGQueryResult struct {
	SeedEntities   []string
	Triples        []Triple
	ContextSnippet string
	TraversalHops  int
}

// ExtractedTriple is the raw LLM output before validation.
type ExtractedTriple struct {
	Subject     string  `json:"subject"`
	Predicate   string  `json:"predicate"`
	Object      string  `json:"object"`
	SubjectType string  `json:"subject_type"`
	ObjectType  string  `json:"object_type"`
	Confidence  float64 `json:"confidence"`
}

// Logger is the minimal logging interface for the KG package.
type Logger interface {
	Info(msg string, args ...any)
	Warn(msg string, args ...any)
	Error(msg string, args ...any)
	Debug(msg string, args ...any)
}
