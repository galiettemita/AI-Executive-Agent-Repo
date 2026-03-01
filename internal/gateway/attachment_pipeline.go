package gateway

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path/filepath"
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
		return fmt.Errorf("I can't process that file type/size.")
	}
	if attachment.SizeBytes < 0 || attachment.SizeBytes > maxAttachmentBytesForMime(mime) {
		return fmt.Errorf("I can't process that file type/size.")
	}
	return nil
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
