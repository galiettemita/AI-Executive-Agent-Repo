// Package preference implements the PAHF (Personalized Agent from Human Feedback)
// preference learning loop.
package preference

import "time"

// PreferenceSignal carries a user correction event for preference extraction.
type PreferenceSignal struct {
	WorkspaceID       string    `json:"workspace_id"`
	UserID            string    `json:"user_id"`
	WorkflowRunID     string    `json:"workflow_run_id"`
	OriginalResponse  string    `json:"original_response"`
	CorrectedResponse string    `json:"corrected_response"`
	OriginalIntent    string    `json:"original_intent"`
	SignalType        string    `json:"signal_type"` // undo/edit/retry/skip/explicit_thumbsdown
	ToolKeyUsed       string    `json:"tool_key_used,omitempty"`
	Timestamp         time.Time `json:"timestamp"`
}

// PreferenceFact is a structured insight extracted from a correction signal.
type PreferenceFact struct {
	WorkspaceID string  `json:"workspace_id"`
	UserID      string  `json:"user_id"`
	Category    string  `json:"category"`
	Preference  string  `json:"preference"`
	Confidence  float64 `json:"confidence"` // 0.95 for direct corrections
	EvidenceID  string  `json:"evidence_id"`
}

// PreferenceContext is the structured preference context injected into GeneratePlanActivity.
type PreferenceContext struct {
	WorkspaceID   string           `json:"workspace_id"`
	UserID        string           `json:"user_id"`
	Facts         []PreferenceFact `json:"facts"`
	FormattedText string           `json:"formatted_text"`
}

// UpdateInput is the activity input for PreferenceUpdateActivity.
type UpdateInput struct {
	Signal PreferenceSignal `json:"signal"`
}

// RetrievalInput is the activity input for PreferenceRetrievalActivity.
type RetrievalInput struct {
	WorkspaceID string `json:"workspace_id"`
	UserID      string `json:"user_id"`
	Intent      string `json:"intent"`
	TopK        int    `json:"top_k"`
}
