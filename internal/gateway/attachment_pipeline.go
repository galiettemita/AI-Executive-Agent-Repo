package gateway

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/google/uuid"
)

var attachmentMimeAllowlist = map[string]struct{}{
	"image/jpeg":      {},
	"image/png":       {},
	"image/webp":      {},
	"application/pdf": {},
	"audio/ogg":       {},
	"audio/mpeg":      {},
	"video/mp4":       {},
	"text/plain":      {},
	"text/csv":        {},
	"application/vnd.openxmlformats-officedocument.wordprocessingml.document": {},
	"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet":       {},
}

func maxAttachmentBytesForMime(mime string) int64 {
	mime = strings.ToLower(strings.TrimSpace(mime))
	switch {
	case strings.HasPrefix(mime, "image/"):
		return 5 * 1024 * 1024
	case strings.HasPrefix(mime, "audio/"):
		return 16 * 1024 * 1024
	case strings.HasPrefix(mime, "video/"):
		return 16 * 1024 * 1024
	default:
		return 100 * 1024 * 1024
	}
}

func ValidateAttachmentInput(attachment AttachmentInput) error {
	mime := strings.ToLower(strings.TrimSpace(attachment.MIMEType))
	if _, ok := attachmentMimeAllowlist[mime]; !ok {
		return fmt.Errorf("unsupported attachment type or size")
	}
	if attachment.SizeBytes < 0 || attachment.SizeBytes > maxAttachmentBytesForMime(mime) {
		return fmt.Errorf("unsupported attachment type or size")
	}
	return nil
}

func ValidateAttachmentMagic(mimeType string, leadingBytes []byte) bool {
	allowed := map[string][][]byte{
		"image/jpeg":      {{0xFF, 0xD8, 0xFF}},
		"image/png":       {{0x89, 0x50, 0x4E, 0x47}},
		"image/webp":      {{0x52, 0x49, 0x46, 0x46}},
		"application/pdf": {{0x25, 0x50, 0x44, 0x46}},
		"audio/ogg":       {{0x4F, 0x67, 0x67, 0x53}},
		"audio/mpeg":      {{0x49, 0x44, 0x33}, {0xFF, 0xFB}},
		"video/mp4":       {{0x00, 0x00, 0x00}},
		"text/plain":      {},
		"text/csv":        {},
		"application/vnd.openxmlformats-officedocument.wordprocessingml.document": {},
		"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet":       {},
	}

	mime := strings.ToLower(strings.TrimSpace(mimeType))
	prefixes, ok := allowed[mime]
	if !ok {
		return false
	}
	if len(prefixes) == 0 {
		return true
	}
	for _, prefix := range prefixes {
		if len(leadingBytes) >= len(prefix) && slices.Equal(leadingBytes[:len(prefix)], prefix) {
			return true
		}
	}
	return false
}

func AttachmentSHA256(content []byte) string {
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:])
}

func AttachmentS3Key(workspaceID, ingressTurnID uuid.UUID, hash, ext string) string {
	ext = strings.TrimSpace(ext)
	if ext != "" && !strings.HasPrefix(ext, ".") {
		ext = "." + ext
	}
	return fmt.Sprintf("attachments/%s/%s/%s%s", workspaceID.String(), ingressTurnID.String(), hash, ext)
}

func ExtensionFromFilename(filename string) string {
	return strings.TrimPrefix(strings.ToLower(filepath.Ext(strings.TrimSpace(filename))), ".")
}
