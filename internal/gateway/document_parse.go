package gateway

import "strings"

const maxExtractedDocumentChars = 100000
const ocrConfidenceThreshold = 0.7

func SupportedDocumentFormats() []string {
	return []string{"pdf", "docx", "xlsx", "csv", "txt", "html", "md"}
}

func IsParseableDocumentMime(mime string) bool {
	switch strings.ToLower(strings.TrimSpace(mime)) {
	case "application/pdf",
		"application/vnd.openxmlformats-officedocument.wordprocessingml.document",
		"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
		"text/csv",
		"text/plain",
		"text/html",
		"text/markdown":
		return true
	default:
		return false
	}
}

func ParseMethodForDocument(mime string, hasExtractableText bool) string {
	if strings.EqualFold(strings.TrimSpace(mime), "application/pdf") && !hasExtractableText {
		return "ocr"
	}
	return "text_extraction"
}

func OCRConfidenceThreshold() float64 {
	return ocrConfidenceThreshold
}

func ShouldAcceptOCRExtraction(confidence float64) bool {
	return confidence >= ocrConfidenceThreshold
}

func TruncateExtractedDocumentText(text string) string {
	if len(text) <= maxExtractedDocumentChars {
		return text
	}
	return text[:maxExtractedDocumentChars] + "[document truncated at 100K chars]"
}
