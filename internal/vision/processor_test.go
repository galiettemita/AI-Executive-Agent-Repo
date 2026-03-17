package vision_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/brevio/brevio/internal/vision"
)

type mockVisionLLM struct {
	response string
	err      error
	called   bool
	images   []vision.VisionImage
}

func (m *mockVisionLLM) CallVision(_ context.Context, _ string, images []vision.VisionImage, _ string) (string, error) {
	m.called = true
	m.images = images
	return m.response, m.err
}

func TestVisionProcessor_ReturnsEmptyForNoAttachments(t *testing.T) {
	llm := &mockVisionLLM{}
	vp := vision.NewVisionProcessor(llm)
	result, err := vp.Process(context.Background(), vision.ExtractionRequest{
		WorkspaceID: "ws-1",
		TurnID:      "turn-1",
	})
	require.NoError(t, err)
	assert.True(t, result.IsEmpty())
	assert.False(t, llm.called)
}

func TestVisionProcessor_CallsLLMWithBase64EncodedImage(t *testing.T) {
	llm := &mockVisionLLM{response: `{"image_type":"other","normalized_text":"hello","entities":[],"confidence":0.9}`}
	vp := vision.NewVisionProcessor(llm)
	result, err := vp.Process(context.Background(), vision.ExtractionRequest{
		WorkspaceID: "ws-1",
		TurnID:      "turn-1",
		Attachments: []vision.ImageAttachment{
			{Data: []byte("fake-image-data"), MimeType: "image/jpeg"},
		},
	})
	require.NoError(t, err)
	assert.True(t, llm.called)
	assert.Len(t, llm.images, 1)
	assert.Equal(t, "image/jpeg", llm.images[0].MediaType)
	assert.Equal(t, "hello", result.NormalizedText)
}

func TestVisionProcessor_ParsesBusinessCardCorrectly(t *testing.T) {
	resp, _ := json.Marshal(map[string]any{
		"image_type":      "business_card",
		"normalized_text": "John Smith\nCEO, Acme Corp\njohn@acme.com\n+1-555-0100",
		"entities": []map[string]string{
			{"type": "person", "value": "John Smith"},
			{"type": "organization", "value": "Acme Corp"},
			{"type": "email", "value": "john@acme.com"},
			{"type": "phone", "value": "+1-555-0100"},
		},
		"confidence": 0.95,
	})
	llm := &mockVisionLLM{response: string(resp)}
	vp := vision.NewVisionProcessor(llm)
	result, err := vp.Process(context.Background(), vision.ExtractionRequest{
		WorkspaceID: "ws-1",
		TurnID:      "turn-1",
		Attachments: []vision.ImageAttachment{{Data: []byte("img"), MimeType: "image/png"}},
	})
	require.NoError(t, err)
	assert.Equal(t, "business_card", result.ImageType)
	assert.Contains(t, result.NormalizedText, "John Smith")
	assert.Len(t, result.Entities, 4)
	assert.InDelta(t, 0.95, result.Confidence, 0.01)
}

func TestVisionProcessor_ParsesInvoiceEntities(t *testing.T) {
	resp, _ := json.Marshal(map[string]any{
		"image_type":      "invoice",
		"normalized_text": "Invoice #1234\nTotal: $500.00",
		"entities": []map[string]string{
			{"type": "amount", "value": "$500.00"},
			{"type": "date", "value": "2026-03-15"},
		},
		"confidence": 0.88,
	})
	llm := &mockVisionLLM{response: string(resp)}
	vp := vision.NewVisionProcessor(llm)
	result, err := vp.Process(context.Background(), vision.ExtractionRequest{
		WorkspaceID: "ws-1",
		TurnID:      "turn-1",
		Attachments: []vision.ImageAttachment{{Data: []byte("img"), MimeType: "image/jpeg"}},
	})
	require.NoError(t, err)
	assert.Equal(t, "invoice", result.ImageType)
	assert.Len(t, result.Entities, 2)
	assert.Equal(t, "amount", result.Entities[0].Type)
}

func TestVisionProcessor_FallsBackOnMalformedJSON(t *testing.T) {
	llm := &mockVisionLLM{response: "This is not valid JSON but contains extracted text"}
	vp := vision.NewVisionProcessor(llm)
	result, err := vp.Process(context.Background(), vision.ExtractionRequest{
		WorkspaceID: "ws-1",
		TurnID:      "turn-1",
		Attachments: []vision.ImageAttachment{{Data: []byte("img"), MimeType: "image/png"}},
	})
	require.NoError(t, err)
	assert.Equal(t, "other", result.ImageType)
	assert.Contains(t, result.NormalizedText, "extracted text")
}

func TestVisionProcessor_IgnoresUnsupportedMimeType(t *testing.T) {
	llm := &mockVisionLLM{}
	vp := vision.NewVisionProcessor(llm)
	result, err := vp.Process(context.Background(), vision.ExtractionRequest{
		WorkspaceID: "ws-1",
		TurnID:      "turn-1",
		Attachments: []vision.ImageAttachment{{Data: []byte("data"), MimeType: "application/zip"}},
	})
	require.NoError(t, err)
	assert.True(t, result.IsEmpty())
	assert.False(t, llm.called)
}

func TestExtractionResult_FormatForPrompt_Empty(t *testing.T) {
	r := &vision.ExtractionResult{}
	assert.Empty(t, r.FormatForPrompt())
}

func TestExtractionResult_FormatForPrompt_WithEntities(t *testing.T) {
	r := &vision.ExtractionResult{
		ImageType:      "business_card",
		NormalizedText: "John Smith\nCEO",
		Entities: []vision.ExtractedEntity{
			{Type: "person", Value: "John Smith"},
			{Type: "organization", Value: "Acme Corp"},
		},
	}
	formatted := r.FormatForPrompt()
	assert.Contains(t, formatted, "Image content extracted by Brevio Vision")
	assert.Contains(t, formatted, "business_card")
	assert.Contains(t, formatted, "John Smith")
	assert.Contains(t, formatted, "person: John Smith")
	assert.Contains(t, formatted, "organization: Acme Corp")
}

// Suppress unused import.
var _ = fmt.Sprint
