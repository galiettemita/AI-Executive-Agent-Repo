package multimodal

import (
	"context"
	"testing"
)

func TestAnalyzeImage(t *testing.T) {
	t.Parallel()
	a := NewAttachmentAnalyzer()

	result, err := a.AnalyzeImage("ctx", "https://example.com/photo.png")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ID == "" {
		t.Fatal("expected non-empty ID")
	}
	if result.Width != 1920 || result.Height != 1080 {
		t.Fatalf("unexpected dimensions: %dx%d", result.Width, result.Height)
	}
	if result.Confidence < 0.5 {
		t.Fatalf("unexpected low confidence: %f", result.Confidence)
	}
}

func TestAnalyzeImageEmptyURL(t *testing.T) {
	t.Parallel()
	a := NewAttachmentAnalyzer()

	_, err := a.AnalyzeImage("ctx", "")
	if err == nil {
		t.Fatal("expected error for empty URL")
	}
}

func TestAnalyzeImageOCR(t *testing.T) {
	t.Parallel()
	a := NewAttachmentAnalyzer()

	result, err := a.AnalyzeImage("ctx", "https://example.com/text-document.png")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ExtractedText == "" {
		t.Fatal("expected OCR text for text-containing URL")
	}
}

func TestAnalyzeAudio(t *testing.T) {
	t.Parallel()
	a := NewAttachmentAnalyzer()

	result, err := a.AnalyzeAudio("ctx", "https://example.com/meeting.mp3")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Transcript == "" {
		t.Fatal("expected non-empty transcript")
	}
	if result.DurationMs <= 0 {
		t.Fatalf("expected positive duration, got %d", result.DurationMs)
	}
	if result.SpeakerCount != 2 {
		t.Fatalf("expected 2 speakers, got %d", result.SpeakerCount)
	}
	if len(result.Segments) != 2 {
		t.Fatalf("expected 2 segments, got %d", len(result.Segments))
	}
}

func TestAnalyzeAudioEmptyURL(t *testing.T) {
	t.Parallel()
	a := NewAttachmentAnalyzer()

	_, err := a.AnalyzeAudio("ctx", "")
	if err == nil {
		t.Fatal("expected error for empty URL")
	}
}

func TestAnalyzeDocument(t *testing.T) {
	t.Parallel()
	a := NewAttachmentAnalyzer()

	result, err := a.AnalyzeDocument("ctx", "https://example.com/report.pdf", "application/pdf")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.PageCount != 3 {
		t.Fatalf("expected 3 pages, got %d", result.PageCount)
	}
	if len(result.Entities) == 0 {
		t.Fatal("expected entities to be extracted")
	}
	if result.MIMEType != "application/pdf" {
		t.Fatalf("unexpected MIME type: %s", result.MIMEType)
	}
}

func TestAnalyzeDocumentUnsupportedMIME(t *testing.T) {
	t.Parallel()
	a := NewAttachmentAnalyzer()

	_, err := a.AnalyzeDocument("ctx", "https://example.com/file.xyz", "application/octet-stream")
	if err == nil {
		t.Fatal("expected error for unsupported MIME type")
	}
}

func TestSupportedMIMETypes(t *testing.T) {
	t.Parallel()
	a := NewAttachmentAnalyzer()

	types := a.SupportedMIMETypes()
	if len(types) < 5 {
		t.Fatalf("expected at least 5 supported MIME types, got %d", len(types))
	}
}

func TestDetectContentType(t *testing.T) {
	t.Parallel()

	tests := []struct {
		mime     string
		expected string
	}{
		{"image/jpeg", "image"},
		{"image/png", "image"},
		{"image/webp", "image"},
		{"audio/mpeg", "audio"},
		{"audio/wav", "audio"},
		{"application/pdf", "document"},
		{"text/plain", "document"},
		{"application/octet-stream", ""},
		{"", ""},
	}
	for _, tc := range tests {
		got := DetectContentType(tc.mime)
		if got != tc.expected {
			t.Errorf("DetectContentType(%q) = %q, want %q", tc.mime, got, tc.expected)
		}
	}
}

func TestValidateAttachment(t *testing.T) {
	t.Parallel()

	// Valid attachment
	err := ValidateAttachment(AttachmentInput{
		URL:       "https://example.com/photo.jpg",
		MIMEType:  "image/jpeg",
		Filename:  "photo.jpg",
		SizeBytes: 1024,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Missing URL
	err = ValidateAttachment(AttachmentInput{MIMEType: "image/jpeg", SizeBytes: 1024})
	if err == nil {
		t.Fatal("expected error for missing URL")
	}

	// Missing MIME type
	err = ValidateAttachment(AttachmentInput{URL: "https://example.com/f", SizeBytes: 1024})
	if err == nil {
		t.Fatal("expected error for missing MIME type")
	}

	// Exceeds size
	err = ValidateAttachment(AttachmentInput{
		URL:       "https://example.com/big.jpg",
		MIMEType:  "image/jpeg",
		SizeBytes: 26 * 1024 * 1024,
	})
	if err == nil {
		t.Fatal("expected error for oversized attachment")
	}

	// Unsupported MIME
	err = ValidateAttachment(AttachmentInput{
		URL:       "https://example.com/f",
		MIMEType:  "application/octet-stream",
		SizeBytes: 100,
	})
	if err == nil {
		t.Fatal("expected error for unsupported MIME type")
	}
}

func TestAnalyzeAttachmentImage(t *testing.T) {
	t.Parallel()
	a := NewAttachmentAnalyzer()

	result, err := a.AnalyzeAttachment(context.Background(), "ws1", AttachmentInput{
		URL:       "https://example.com/photo.jpg",
		MIMEType:  "image/jpeg",
		Filename:  "photo.jpg",
		SizeBytes: 2048,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.AttachmentID == "" {
		t.Fatal("expected non-empty attachment ID")
	}
	if result.ContentType != "image" {
		t.Fatalf("expected image content type, got %s", result.ContentType)
	}
	if result.Confidence < 0.5 {
		t.Fatalf("low confidence: %f", result.Confidence)
	}
}

func TestAnalyzeAttachmentAudio(t *testing.T) {
	t.Parallel()
	a := NewAttachmentAnalyzer()

	result, err := a.AnalyzeAttachment(context.Background(), "ws1", AttachmentInput{
		URL:       "https://example.com/meeting.mp3",
		MIMEType:  "audio/mpeg",
		Filename:  "meeting.mp3",
		SizeBytes: 5000,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ContentType != "audio" {
		t.Fatalf("expected audio content type, got %s", result.ContentType)
	}
	if result.ExtractedText == "" {
		t.Fatal("expected transcript in extracted text")
	}
}

func TestAnalyzeAttachmentDocument(t *testing.T) {
	t.Parallel()
	a := NewAttachmentAnalyzer()

	result, err := a.AnalyzeAttachment(context.Background(), "ws1", AttachmentInput{
		URL:       "https://example.com/report.pdf",
		MIMEType:  "application/pdf",
		Filename:  "report.pdf",
		SizeBytes: 10000,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ContentType != "document" {
		t.Fatalf("expected document content type, got %s", result.ContentType)
	}
}

func TestAnalyzeAttachmentMissingWorkspace(t *testing.T) {
	t.Parallel()
	a := NewAttachmentAnalyzer()

	_, err := a.AnalyzeAttachment(context.Background(), "", AttachmentInput{
		URL:       "https://example.com/photo.jpg",
		MIMEType:  "image/jpeg",
		Filename:  "photo.jpg",
		SizeBytes: 1024,
	})
	if err == nil {
		t.Fatal("expected error for empty workspace ID")
	}
}

func TestAnalyzeAttachmentValidationError(t *testing.T) {
	t.Parallel()
	a := NewAttachmentAnalyzer()

	_, err := a.AnalyzeAttachment(context.Background(), "ws1", AttachmentInput{
		URL:       "https://example.com/file",
		MIMEType:  "video/mp4",
		Filename:  "file.mp4",
		SizeBytes: 1024,
	})
	if err == nil {
		t.Fatal("expected error for unsupported MIME type")
	}
}

func TestBuildMultimodalContext(t *testing.T) {
	t.Parallel()

	messages := []ContextMessage{
		{Role: "user", Content: "Hello", Tokens: 10},
		{Role: "assistant", Content: "Hi there", Tokens: 15},
		{Role: "user", Content: "Analyze this", Tokens: 20},
	}
	analyses := []AnalysisResult{
		{AttachmentID: "a1", ContentType: "image", Description: "photo"},
		{AttachmentID: "a2", ContentType: "document", Description: "report"},
	}

	ctx := BuildMultimodalContext(messages, analyses, 300)
	if len(ctx.Messages) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(ctx.Messages))
	}
	if len(ctx.Attachments) != 2 {
		t.Fatalf("expected 2 attachments, got %d", len(ctx.Attachments))
	}
	if ctx.TotalTokenEstimate <= 0 {
		t.Fatal("expected positive token estimate")
	}
}

func TestBuildMultimodalContextBudgetLimit(t *testing.T) {
	t.Parallel()

	messages := []ContextMessage{
		{Role: "user", Content: "Hello", Tokens: 50},
		{Role: "assistant", Content: "World", Tokens: 50},
		{Role: "user", Content: "Third", Tokens: 50},
	}
	analyses := []AnalysisResult{
		{AttachmentID: "a1", ContentType: "image"},
	}

	ctx := BuildMultimodalContext(messages, analyses, 110)
	if len(ctx.Messages) != 2 {
		t.Fatalf("expected 2 messages within budget, got %d", len(ctx.Messages))
	}
	if len(ctx.Attachments) != 0 {
		t.Fatalf("expected 0 attachments (no budget left), got %d", len(ctx.Attachments))
	}
}

func TestValidateAttachmentNegativeSize(t *testing.T) {
	t.Parallel()

	err := ValidateAttachment(AttachmentInput{
		URL:       "https://example.com/f.jpg",
		MIMEType:  "image/jpeg",
		SizeBytes: -1,
	})
	if err == nil {
		t.Fatal("expected error for negative size")
	}
}
