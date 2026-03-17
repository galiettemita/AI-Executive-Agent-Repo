package vision

import "time"

// ImageAttachment represents an image received from WhatsApp, iMessage, or API.
type ImageAttachment struct {
	Data      []byte `json:"data"`
	MimeType  string `json:"mime_type"`
	SourceURL string `json:"source_url,omitempty"`
	Filename  string `json:"filename,omitempty"`
}

// ExtractionRequest is input to the vision processor.
type ExtractionRequest struct {
	WorkspaceID string            `json:"workspace_id"`
	TurnID      string            `json:"turn_id"`
	Attachments []ImageAttachment `json:"attachments"`
	Hint        string            `json:"hint,omitempty"`
}

// ExtractedEntity is a named entity found in the image.
type ExtractedEntity struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

// ExtractionResult is the structured output of the vision processor.
type ExtractionResult struct {
	WorkspaceID    string            `json:"workspace_id"`
	TurnID         string            `json:"turn_id"`
	NormalizedText string            `json:"normalized_text"`
	Entities       []ExtractedEntity `json:"entities"`
	ImageType      string            `json:"image_type"`
	Confidence     float64           `json:"confidence"`
	ProcessedAt    time.Time         `json:"processed_at"`
}

// IsEmpty returns true when no meaningful content was extracted.
func (r *ExtractionResult) IsEmpty() bool {
	return r == nil || r.NormalizedText == ""
}

// FormatForPrompt returns a prompt-injectable string describing the image content.
func (r *ExtractionResult) FormatForPrompt() string {
	if r.IsEmpty() {
		return ""
	}
	out := "[Image content extracted by Brevio Vision]\n"
	out += "Type: " + r.ImageType + "\n"
	out += "Text:\n" + r.NormalizedText + "\n"
	if len(r.Entities) > 0 {
		out += "Entities:\n"
		for _, e := range r.Entities {
			out += "  " + e.Type + ": " + e.Value + "\n"
		}
	}
	return out
}
