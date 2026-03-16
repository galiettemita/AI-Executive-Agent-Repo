package gateway

import (
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestAttachmentValidationAndS3Key(t *testing.T) {
	t.Parallel()

	okAttachment := AttachmentInput{
		URL:       "https://example.com/file.pdf",
		MIMEType:  "application/pdf",
		SizeBytes: 1024,
	}
	if err := ValidateAttachmentInput(okAttachment); err != nil {
		t.Fatalf("expected valid attachment, got %v", err)
	}

	tooLargeImage := AttachmentInput{
		URL:       "https://example.com/img.jpg",
		MIMEType:  "image/jpeg",
		SizeBytes: 21 * 1024 * 1024, // exceeds 20 MB image cap
	}
	if err := ValidateAttachmentInput(tooLargeImage); err == nil {
		t.Fatal("expected oversized image rejection")
	}

	workspaceID := uuid.MustParse("018f3f6a-9a0f-7cc6-8f2f-1f0f2d2f2d2f")
	ingressTurnID := uuid.MustParse("018f3f6a-9a0f-7cc6-8f2f-1f0f2d2f2d2e")
	key := AttachmentS3Key(workspaceID, ingressTurnID, "abcdef", "pdf")
	if !strings.Contains(key, "attachments/018f3f6a-9a0f-7cc6-8f2f-1f0f2d2f2d2f/018f3f6a-9a0f-7cc6-8f2f-1f0f2d2f2d2e/abcdef.pdf") {
		t.Fatalf("unexpected attachment s3 key: %s", key)
	}
	if ext := ExtensionFromFilename("Report.DOCX"); ext != "docx" {
		t.Fatalf("unexpected extension parse result: %s", ext)
	}
	if !ValidateAttachmentMagic("application/pdf", []byte{0x25, 0x50, 0x44, 0x46, 0x2d}) {
		t.Fatal("expected valid PDF magic bytes")
	}
	if ValidateAttachmentMagic("image/png", []byte{0x25, 0x50, 0x44, 0x46}) {
		t.Fatal("expected invalid PNG magic bytes")
	}
}
