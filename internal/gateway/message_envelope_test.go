package gateway

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestBuildMessageEnvelopePreservesMultipleMediaParts(t *testing.T) {
	t.Parallel()

	envelope, err := BuildMessageEnvelope(BuildMessageEnvelopeInput{
		ID:              uuid.MustParse("018f3f6a-9a0f-7cc6-8f2f-1f0f2d2f2d2f"),
		Channel:         "api",
		UserID:          uuid.MustParse("018f3f6a-9a0f-7cc6-8f2f-1f0f2d2f2d30"),
		Timestamp:       time.Date(2026, 4, 22, 12, 0, 0, 0, time.UTC),
		MessageText:     "please compare these",
		SessionID:       uuid.MustParse("018f3f6a-9a0f-7cc6-8f2f-1f0f2d2f2d31"),
		UserProfileHash: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		Attachments: []AttachmentReference{
			{
				ID:        "asset-image",
				AssetID:   "asset-image",
				SourceURL: "https://cdn.example.com/screenshot.png",
				S3URI:     "s3://attachments/a/screenshot.png",
				SHA256:    "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
				MIMEType:  "image/png",
				SizeBytes: 1024,
			},
			{
				ID:        "asset-video",
				AssetID:   "asset-video",
				SourceURL: "https://cdn.example.com/demo.mp4",
				S3URI:     "s3://attachments/a/demo.mp4",
				SHA256:    "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
				MIMEType:  "video/mp4",
				SizeBytes: 2048,
			},
		},
	})
	if err != nil {
		t.Fatalf("build envelope: %v", err)
	}
	if envelope.Content.Type != "MULTIMODAL" {
		t.Fatalf("expected multimodal content type, got %s", envelope.Content.Type)
	}
	if len(envelope.Content.Parts) != 3 {
		t.Fatalf("expected text plus two media parts, got %d", len(envelope.Content.Parts))
	}
	if envelope.Content.Parts[1].Type != "image" || envelope.Content.Parts[2].Type != "video" {
		t.Fatalf("unexpected content parts: %+v", envelope.Content.Parts)
	}
	if len(envelope.Content.MediaAssets) != 2 {
		t.Fatalf("expected two media assets, got %d", len(envelope.Content.MediaAssets))
	}
}
