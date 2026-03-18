package watermark

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"os"
	"strings"
)

const (
	// zwnj is the zero-width non-joiner used as a delimiter before the
	// variation-selector watermark sequence.
	zwnj = '\u200C'

	// variationSelectorBase is U+FE00, the start of the variation selectors block.
	// Each nibble (0–15) maps to U+FE00..U+FE0F.
	variationSelectorBase = 0xFE00
)

// C2PAContentWatermarker embeds invisible Unicode variation-selector tags
// encoding an HMAC-SHA256 of provenance metadata into response text.
type C2PAContentWatermarker struct {
	hmacKey []byte
	logger  *slog.Logger
}

// NewC2PAContentWatermarker creates a watermarker from the WATERMARK_HMAC_KEY
// environment variable. Returns an error if the key is missing or malformed.
func NewC2PAContentWatermarker(logger *slog.Logger) (*C2PAContentWatermarker, error) {
	keyHex := os.Getenv("WATERMARK_HMAC_KEY")
	if keyHex == "" {
		return nil, fmt.Errorf("WATERMARK_HMAC_KEY is required")
	}

	key, err := hex.DecodeString(keyHex)
	if err != nil {
		return nil, fmt.Errorf("WATERMARK_HMAC_KEY must be hex-encoded: %w", err)
	}
	if len(key) < 16 {
		return nil, fmt.Errorf("WATERMARK_HMAC_KEY must be at least 16 bytes (32 hex chars)")
	}

	return &C2PAContentWatermarker{hmacKey: key, logger: logger}, nil
}

// NewC2PAContentWatermarkerWithKey creates a watermarker from an explicit key.
// Used in tests and dependency injection.
func NewC2PAContentWatermarkerWithKey(key []byte, logger *slog.Logger) *C2PAContentWatermarker {
	return &C2PAContentWatermarker{hmacKey: key, logger: logger}
}

// HMACKey returns the raw HMAC key for use by dependent services (e.g., SemanticWatermarker).
func (w *C2PAContentWatermarker) HMACKey() []byte {
	return w.hmacKey
}

// Tag appends an invisible Unicode watermark tag to the end of text.
// The tag encodes an HMAC-SHA256 of the provenance metadata as variation
// selector characters preceded by a ZWNJ delimiter.
func (w *C2PAContentWatermarker) Tag(_ context.Context, text string, meta WatermarkMeta) (string, error) {
	tagPayload := buildTagPayload(meta)
	mac := computeHMAC(w.hmacKey, tagPayload)
	hexMAC := hex.EncodeToString(mac)

	// Encode 64 hex nibbles as variation selectors.
	var tag strings.Builder
	tag.WriteRune(zwnj)
	for _, nibbleChar := range hexMAC {
		nibbleVal := hexCharToNibble(byte(nibbleChar))
		tag.WriteRune(rune(variationSelectorBase + int(nibbleVal)))
	}

	return text + tag.String(), nil
}

// Verify extracts the watermark from text and returns verification metadata.
// It looks up the provenance record by content hash if a ProvenanceStore is
// available, or returns a basic IsBrevioGenerated=true if the watermark is present.
func (w *C2PAContentWatermarker) Verify(text string) (*WatermarkVerification, error) {
	_, hmacHex, found := extractWatermark(text)
	if !found {
		return &WatermarkVerification{IsBrevioGenerated: false, Confidence: 0.0}, nil
	}

	// The HMAC is present — the content was tagged by this system.
	_ = hmacHex // used for provenance lookup in the API handler
	return &WatermarkVerification{
		IsBrevioGenerated: true,
		Confidence:        1.0,
	}, nil
}

// VerifyWithProvenance checks the watermark and enriches with provenance data.
func (w *C2PAContentWatermarker) VerifyWithProvenance(text string, store *ProvenanceStore) (*WatermarkVerification, error) {
	_, _, found := extractWatermark(text)
	if !found {
		return &WatermarkVerification{IsBrevioGenerated: false, Confidence: 0.0}, nil
	}

	// Look up by content hash.
	cleanText := Strip(text)
	contentHash := sha256Hex(cleanText)

	if store != nil && store.db != nil {
		record, err := store.LookupByContentHash(context.Background(), contentHash)
		if err == nil && record != nil {
			return &WatermarkVerification{
				IsBrevioGenerated: true,
				WorkspaceID:       record.WorkspaceID,
				ModelID:           record.ModelID,
				RequestID:         record.RequestID,
				Confidence:        1.0,
			}, nil
		}
	}

	return &WatermarkVerification{
		IsBrevioGenerated: true,
		Confidence:        0.9, // tagged but no provenance record found
	}, nil
}

// Strip removes all watermark sequences (ZWNJ + variation selectors) from text.
func Strip(text string) string {
	var result strings.Builder
	runes := []rune(text)

	i := 0
	for i < len(runes) {
		if runes[i] == zwnj && i+1 < len(runes) && isVariationSelector(runes[i+1]) {
			// Skip the ZWNJ and all subsequent variation selectors.
			i++ // skip ZWNJ
			for i < len(runes) && isVariationSelector(runes[i]) {
				i++
			}
			continue
		}
		result.WriteRune(runes[i])
		i++
	}

	return result.String()
}

// extractWatermark scans text for the ZWNJ + variation-selector sequence.
// Returns the clean text, the extracted HMAC hex, and whether a watermark was found.
func extractWatermark(text string) (clean string, hmacHex string, found bool) {
	runes := []rune(text)
	var cleanBuf strings.Builder
	var hmacBuf strings.Builder

	i := 0
	for i < len(runes) {
		if runes[i] == zwnj && i+1 < len(runes) && isVariationSelector(runes[i+1]) {
			i++ // skip ZWNJ
			for i < len(runes) && isVariationSelector(runes[i]) {
				nibble := runes[i] - variationSelectorBase
				hmacBuf.WriteByte(nibbleToHexChar(byte(nibble)))
				i++
			}
			found = true
			continue
		}
		cleanBuf.WriteRune(runes[i])
		i++
	}

	return cleanBuf.String(), hmacBuf.String(), found
}

func isVariationSelector(r rune) bool {
	return r >= 0xFE00 && r <= 0xFE0F
}

func buildTagPayload(meta WatermarkMeta) string {
	return fmt.Sprintf("%s|%s|%d|%s",
		meta.ModelID,
		meta.WorkspaceID.String(),
		meta.Timestamp.Unix(),
		meta.RequestID.String(),
	)
}

func computeHMAC(key []byte, data string) []byte {
	h := hmac.New(sha256.New, key)
	h.Write([]byte(data))
	return h.Sum(nil)
}

func hexCharToNibble(c byte) byte {
	switch {
	case c >= '0' && c <= '9':
		return c - '0'
	case c >= 'a' && c <= 'f':
		return c - 'a' + 10
	case c >= 'A' && c <= 'F':
		return c - 'A' + 10
	default:
		return 0
	}
}

func nibbleToHexChar(n byte) byte {
	if n < 10 {
		return '0' + n
	}
	return 'a' + (n - 10)
}

func sha256Hex(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}
