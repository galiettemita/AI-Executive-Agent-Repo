package vision

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// VisionLLM is the interface to the Claude vision API.
type VisionLLM interface {
	CallVision(ctx context.Context, workspaceID string, images []VisionImage, prompt string) (string, error)
}

// VisionImage is the wire format for an image in a Claude vision API call.
type VisionImage struct {
	Type      string `json:"type"`
	MediaType string `json:"media_type"`
	Data      string `json:"data"`
}

// VisionProcessor calls Claude vision API to extract text and entities from images.
type VisionProcessor struct {
	llm    VisionLLM
	maxPDF int
}

// NewVisionProcessor creates a VisionProcessor.
func NewVisionProcessor(llm VisionLLM) *VisionProcessor {
	return &VisionProcessor{llm: llm, maxPDF: 5}
}

// VisionSystemPrompt is the system prompt used for vision extraction.
const VisionSystemPrompt = `You are a document and image extraction assistant for an AI executive.
Your task is to extract ALL text and structured information from the provided image(s).

Respond ONLY with a JSON object in this exact format:
{
  "image_type": "<business_card|invoice|document|qr_code|whiteboard|screenshot|other>",
  "normalized_text": "<complete text content extracted from the image, formatted cleanly>",
  "entities": [
    {"type": "<person|organization|email|phone|date|amount|url|address>", "value": "<value>"}
  ],
  "confidence": <0.0 to 1.0>
}

Rules:
- Extract ALL visible text, preserving structure where helpful.
- For business cards: extract name, title, company, email, phone, address.
- For invoices: extract vendor, date, line items, amounts, totals.
- confidence: 0.9+ for clear images, 0.5-0.9 for partially legible, <0.5 for poor quality.`

// Process extracts text and entities from image attachments.
func (vp *VisionProcessor) Process(ctx context.Context, req ExtractionRequest) (*ExtractionResult, error) {
	if len(req.Attachments) == 0 {
		return &ExtractionResult{
			WorkspaceID: req.WorkspaceID,
			TurnID:      req.TurnID,
			ProcessedAt: time.Now().UTC(),
		}, nil
	}

	var images []VisionImage
	for _, att := range req.Attachments {
		if !IsSupportedMimeType(att.MimeType) {
			continue
		}
		images = append(images, VisionImage{
			Type:      "base64",
			MediaType: att.MimeType,
			Data:      base64.StdEncoding.EncodeToString(att.Data),
		})
	}
	if len(images) == 0 {
		return &ExtractionResult{WorkspaceID: req.WorkspaceID, TurnID: req.TurnID, ProcessedAt: time.Now().UTC()}, nil
	}

	userPrompt := "Extract all text and information from the provided image(s)."
	if req.Hint != "" {
		userPrompt += " Context: " + req.Hint
	}

	raw, err := vp.llm.CallVision(ctx, req.WorkspaceID, images, userPrompt)
	if err != nil {
		return nil, fmt.Errorf("vision_processor: llm call: %w", err)
	}

	result := parseVisionResponse(raw)
	result.WorkspaceID = req.WorkspaceID
	result.TurnID = req.TurnID
	result.ProcessedAt = time.Now().UTC()
	return result, nil
}

func parseVisionResponse(raw string) *ExtractionResult {
	result := &ExtractionResult{Confidence: 0.5}

	raw = strings.TrimSpace(raw)
	if start := strings.Index(raw, "{"); start >= 0 {
		if end := strings.LastIndex(raw, "}"); end > start {
			raw = raw[start : end+1]
		}
	}

	var parsed struct {
		ImageType      string `json:"image_type"`
		NormalizedText string `json:"normalized_text"`
		Entities       []struct {
			Type  string `json:"type"`
			Value string `json:"value"`
		} `json:"entities"`
		Confidence float64 `json:"confidence"`
	}
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		result.NormalizedText = raw
		result.ImageType = "other"
		return result
	}

	result.ImageType = parsed.ImageType
	result.NormalizedText = parsed.NormalizedText
	result.Confidence = parsed.Confidence
	for _, e := range parsed.Entities {
		result.Entities = append(result.Entities, ExtractedEntity{Type: e.Type, Value: e.Value})
	}
	return result
}

// IsSupportedMimeType returns true for MIME types Claude vision supports.
func IsSupportedMimeType(mime string) bool {
	switch mime {
	case "image/jpeg", "image/jpg", "image/png", "image/gif",
		"image/webp", "application/pdf":
		return true
	default:
		return strings.HasPrefix(mime, "image/")
	}
}

// DetectImageAttachments examines a raw message payload and returns image attachments.
func DetectImageAttachments(messagePayload []byte, mimeType string) []ImageAttachment {
	if IsSupportedMimeType(mimeType) {
		return []ImageAttachment{{Data: messagePayload, MimeType: mimeType}}
	}
	return nil
}
