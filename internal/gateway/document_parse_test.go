package gateway

import (
	"strings"
	"testing"
)

func TestDocumentParseHelpers(t *testing.T) {
	t.Parallel()

	if !IsParseableDocumentMime("application/pdf") {
		t.Fatal("expected pdf mime to be parseable")
	}
	if ParseMethodForDocument("application/pdf", false) != "ocr" {
		t.Fatal("expected OCR parse method for image-only PDF")
	}
	if ParseMethodForDocument("text/plain", true) != "text_extraction" {
		t.Fatal("expected text extraction parse method")
	}
	if OCRConfidenceThreshold() != 0.7 {
		t.Fatalf("unexpected OCR threshold: %f", OCRConfidenceThreshold())
	}
	if !ShouldAcceptOCRExtraction(0.7) || ShouldAcceptOCRExtraction(0.69) {
		t.Fatal("unexpected OCR confidence acceptance behavior")
	}

	longText := strings.Repeat("a", 100001)
	truncated := TruncateExtractedDocumentText(longText)
	if !strings.Contains(truncated, "[document truncated at 100K chars]") {
		t.Fatal("expected truncation marker")
	}
}
