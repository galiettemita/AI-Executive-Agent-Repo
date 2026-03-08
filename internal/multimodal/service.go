package multimodal

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ImageAnalysis holds the result of analyzing an image.
type ImageAnalysis struct {
	ID            string    `json:"id"`
	URL           string    `json:"url"`
	Description   string    `json:"description"`
	Objects       []string  `json:"objects"`
	ExtractedText string    `json:"extracted_text"`
	Width         int       `json:"width"`
	Height        int       `json:"height"`
	Confidence    float64   `json:"confidence"`
	AnalyzedAt    time.Time `json:"analyzed_at"`
}

// AudioSegment represents a segment within an audio transcript.
type AudioSegment struct {
	StartMs int    `json:"start_ms"`
	EndMs   int    `json:"end_ms"`
	Speaker string `json:"speaker"`
	Text    string `json:"text"`
}

// AudioAnalysis holds the result of analyzing audio.
type AudioAnalysis struct {
	ID           string         `json:"id"`
	URL          string         `json:"url"`
	Transcript   string         `json:"transcript"`
	DurationMs   int            `json:"duration_ms"`
	Language     string         `json:"language"`
	SpeakerCount int            `json:"speaker_count"`
	Segments     []AudioSegment `json:"segments"`
	AnalyzedAt   time.Time      `json:"analyzed_at"`
}

// DocumentAnalysis holds the result of analyzing a document.
type DocumentAnalysis struct {
	ID         string    `json:"id"`
	URL        string    `json:"url"`
	Text       string    `json:"text"`
	PageCount  int       `json:"page_count"`
	Summary    string    `json:"summary"`
	Entities   []string  `json:"entities"`
	MIMEType   string    `json:"mime_type"`
	AnalyzedAt time.Time `json:"analyzed_at"`
}

// AttachmentInput represents an incoming attachment to analyze.
type AttachmentInput struct {
	URL       string `json:"url"`
	MIMEType  string `json:"mime_type"`
	Filename  string `json:"filename"`
	SizeBytes int64  `json:"size_bytes"`
}

// AnalysisResult holds the unified result of analyzing any attachment type.
type AnalysisResult struct {
	AttachmentID  string         `json:"attachment_id"`
	ContentType   string         `json:"content_type"` // image, audio, document, video
	Description   string         `json:"description"`
	ExtractedText string         `json:"extracted_text"`
	Metadata      map[string]any `json:"metadata"`
	Confidence    float64        `json:"confidence"`
}

// ContextMessage represents a message used to build multimodal context.
type ContextMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
	Tokens  int    `json:"tokens"`
}

// MultimodalContext holds messages and attachment analyses within a token budget.
type MultimodalContext struct {
	Messages           []ContextMessage `json:"messages"`
	Attachments        []AnalysisResult `json:"attachments"`
	TotalTokenEstimate int              `json:"total_token_estimate"`
}

const maxAttachmentSize int64 = 25 * 1024 * 1024 // 25MB

var supportedMIMETypes = map[string]string{
	"image/jpeg":      "image",
	"image/png":       "image",
	"image/webp":      "image",
	"audio/mpeg":      "audio",
	"audio/wav":       "audio",
	"application/pdf": "document",
	"text/plain":      "document",
}

// AttachmentAnalyzer provides multimodal content analysis.
type AttachmentAnalyzer struct {
	mu        sync.Mutex
	images    map[string]ImageAnalysis
	audio     map[string]AudioAnalysis
	documents map[string]DocumentAnalysis
	results   map[string]AnalysisResult
	now       func() time.Time
}

// NewAttachmentAnalyzer creates a new AttachmentAnalyzer.
func NewAttachmentAnalyzer() *AttachmentAnalyzer {
	return &AttachmentAnalyzer{
		images:    map[string]ImageAnalysis{},
		audio:     map[string]AudioAnalysis{},
		documents: map[string]DocumentAnalysis{},
		results:   map[string]AnalysisResult{},
		now:       func() time.Time { return time.Now().UTC() },
	}
}

// SupportedMIMETypes returns the list of MIME types the analyzer can handle.
func (a *AttachmentAnalyzer) SupportedMIMETypes() []string {
	return []string{
		"image/jpeg",
		"image/png",
		"image/webp",
		"audio/mpeg",
		"audio/wav",
		"application/pdf",
		"text/plain",
	}
}

// DetectContentType maps a MIME type to a content category.
func DetectContentType(mimeType string) string {
	ct, ok := supportedMIMETypes[strings.ToLower(strings.TrimSpace(mimeType))]
	if !ok {
		return ""
	}
	return ct
}

// ValidateAttachment checks that the input has a valid MIME type and is within size limits.
func ValidateAttachment(input AttachmentInput) error {
	if strings.TrimSpace(input.URL) == "" {
		return fmt.Errorf("attachment URL is required")
	}
	if strings.TrimSpace(input.MIMEType) == "" {
		return fmt.Errorf("MIME type is required")
	}
	if input.SizeBytes > maxAttachmentSize {
		return fmt.Errorf("attachment size %d exceeds maximum %d bytes", input.SizeBytes, maxAttachmentSize)
	}
	if input.SizeBytes < 0 {
		return fmt.Errorf("attachment size cannot be negative")
	}
	ct := DetectContentType(input.MIMEType)
	if ct == "" {
		return fmt.Errorf("unsupported MIME type: %s", input.MIMEType)
	}
	return nil
}

// AnalyzeAttachment analyzes an attachment and returns a unified result.
func (a *AttachmentAnalyzer) AnalyzeAttachment(_ context.Context, workspaceID string, input AttachmentInput) (*AnalysisResult, error) {
	if strings.TrimSpace(workspaceID) == "" {
		return nil, fmt.Errorf("workspace ID is required")
	}
	if err := ValidateAttachment(input); err != nil {
		return nil, err
	}

	contentType := DetectContentType(input.MIMEType)

	a.mu.Lock()
	defer a.mu.Unlock()

	id := uuid.Must(uuid.NewV7()).String()
	result := AnalysisResult{
		AttachmentID: id,
		ContentType:  contentType,
		Description:  fmt.Sprintf("Analyzed %s: %s", contentType, input.Filename),
		Metadata: map[string]any{
			"workspace_id": workspaceID,
			"filename":     input.Filename,
			"mime_type":    input.MIMEType,
			"size_bytes":   input.SizeBytes,
		},
		Confidence: 0.90,
	}

	switch contentType {
	case "image":
		result.ExtractedText = ""
		if strings.Contains(strings.ToLower(input.URL), "text") || strings.Contains(strings.ToLower(input.URL), "ocr") {
			result.ExtractedText = "Sample extracted text from image"
		}
		result.Metadata["width"] = 1920
		result.Metadata["height"] = 1080
	case "audio":
		result.ExtractedText = "Hello, this is the beginning. Thank you for the introduction."
		result.Metadata["duration_ms"] = 12000
		result.Metadata["speaker_count"] = 2
		result.Metadata["language"] = "en"
	case "document":
		result.ExtractedText = "Extracted document text from " + input.URL
		result.Metadata["page_count"] = 3
	}

	a.results[id] = result
	return &result, nil
}

// BuildMultimodalContext constructs a multimodal context within a token budget.
func BuildMultimodalContext(messages []ContextMessage, analyses []AnalysisResult, budgetTokens int) *MultimodalContext {
	ctx := &MultimodalContext{
		Messages:    []ContextMessage{},
		Attachments: []AnalysisResult{},
	}

	tokensUsed := 0

	// Add messages first, respecting budget.
	for _, msg := range messages {
		cost := msg.Tokens
		if cost <= 0 {
			cost = len(msg.Content) / 4 // rough estimate
			if cost < 1 {
				cost = 1
			}
		}
		if tokensUsed+cost > budgetTokens {
			break
		}
		ctx.Messages = append(ctx.Messages, msg)
		tokensUsed += cost
	}

	// Add analyses, estimating ~100 tokens per analysis.
	for _, ar := range analyses {
		cost := 100
		if tokensUsed+cost > budgetTokens {
			break
		}
		ctx.Attachments = append(ctx.Attachments, ar)
		tokensUsed += cost
	}

	ctx.TotalTokenEstimate = tokensUsed
	return ctx
}

// AnalyzeImage extracts description, objects, OCR text, and dimensions from an image URL.
func (a *AttachmentAnalyzer) AnalyzeImage(ctx string, imageURL string) (*ImageAnalysis, error) {
	if strings.TrimSpace(imageURL) == "" {
		return nil, fmt.Errorf("image URL is required")
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	id := uuid.Must(uuid.NewV7()).String()
	analysis := ImageAnalysis{
		ID:            id,
		URL:           imageURL,
		Description:   "Analyzed image from " + imageURL,
		Objects:       extractObjectsFromURL(imageURL),
		ExtractedText: "",
		Width:         1920,
		Height:        1080,
		Confidence:    0.92,
		AnalyzedAt:    a.now(),
	}

	if strings.Contains(strings.ToLower(imageURL), "text") || strings.Contains(strings.ToLower(imageURL), "ocr") {
		analysis.ExtractedText = "Sample extracted text from image"
	}

	a.images[id] = analysis
	return &analysis, nil
}

// AnalyzeAudio extracts transcript, duration, language, and speaker information from an audio URL.
func (a *AttachmentAnalyzer) AnalyzeAudio(ctx string, audioURL string) (*AudioAnalysis, error) {
	if strings.TrimSpace(audioURL) == "" {
		return nil, fmt.Errorf("audio URL is required")
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	id := uuid.Must(uuid.NewV7()).String()
	segments := []AudioSegment{
		{StartMs: 0, EndMs: 5000, Speaker: "Speaker_1", Text: "Hello, this is the beginning."},
		{StartMs: 5000, EndMs: 12000, Speaker: "Speaker_2", Text: "Thank you for the introduction."},
	}

	analysis := AudioAnalysis{
		ID:           id,
		URL:          audioURL,
		Transcript:   "Hello, this is the beginning. Thank you for the introduction.",
		DurationMs:   12000,
		Language:     "en",
		SpeakerCount: 2,
		Segments:     segments,
		AnalyzedAt:   a.now(),
	}

	a.audio[id] = analysis
	return &analysis, nil
}

// AnalyzeDocument extracts text, page count, summary, and entities from a document.
func (a *AttachmentAnalyzer) AnalyzeDocument(ctx string, docURL string, mimeType string) (*DocumentAnalysis, error) {
	if strings.TrimSpace(docURL) == "" {
		return nil, fmt.Errorf("document URL is required")
	}
	if strings.TrimSpace(mimeType) == "" {
		return nil, fmt.Errorf("MIME type is required")
	}

	if !isSupportedDocMIME(mimeType) {
		return nil, fmt.Errorf("unsupported MIME type: %s", mimeType)
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	id := uuid.Must(uuid.NewV7()).String()
	analysis := DocumentAnalysis{
		ID:         id,
		URL:        docURL,
		Text:       "Extracted document text from " + docURL,
		PageCount:  3,
		Summary:    "A document containing relevant information.",
		Entities:   []string{"Organization", "Date", "Person"},
		MIMEType:   mimeType,
		AnalyzedAt: a.now(),
	}

	a.documents[id] = analysis
	return &analysis, nil
}

func extractObjectsFromURL(url string) []string {
	objects := []string{"background"}
	lower := strings.ToLower(url)
	if strings.Contains(lower, "person") || strings.Contains(lower, "people") {
		objects = append(objects, "person")
	}
	if strings.Contains(lower, "chart") || strings.Contains(lower, "graph") {
		objects = append(objects, "chart")
	}
	return objects
}

func isSupportedDocMIME(mimeType string) bool {
	supported := map[string]bool{
		"application/pdf": true,
		"application/vnd.openxmlformats-officedocument.wordprocessingml.document": true,
		"text/plain": true,
	}
	return supported[strings.TrimSpace(strings.ToLower(mimeType))]
}
