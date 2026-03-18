package watermark

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"unicode"

	"github.com/google/uuid"
)

// SemanticWatermarker replaces words with deterministic synonym choices
// driven by HMAC, creating a statistical fingerprint in the text.
type SemanticWatermarker struct {
	synonymMap map[string][]string
	hmacKey    []byte
	logger     *slog.Logger
}

// NewSemanticWatermarker loads the synonym map and constructs the watermarker.
func NewSemanticWatermarker(hmacKey []byte, logger *slog.Logger) (*SemanticWatermarker, error) {
	synonymMap, err := loadSynonymMap()
	if err != nil {
		return nil, fmt.Errorf("load synonym map: %w", err)
	}

	return &SemanticWatermarker{
		synonymMap: synonymMap,
		hmacKey:    hmacKey,
		logger:     logger,
	}, nil
}

// NewSemanticWatermarkerWithMap creates a watermarker with an explicit synonym map.
func NewSemanticWatermarkerWithMap(synonymMap map[string][]string, hmacKey []byte, logger *slog.Logger) *SemanticWatermarker {
	return &SemanticWatermarker{
		synonymMap: synonymMap,
		hmacKey:    hmacKey,
		logger:     logger,
	}
}

func loadSynonymMap() (map[string][]string, error) {
	data, err := os.ReadFile("internal/watermark/data/synonym_map.json")
	if err != nil {
		return nil, err
	}

	var m map[string][]string
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	return m, nil
}

// Watermark applies synonym substitution driven by HMAC to create a
// statistical fingerprint in the text.
func (sw *SemanticWatermarker) Watermark(_ context.Context, text string, meta WatermarkMeta) (string, error) {
	hmacInput := fmt.Sprintf("%s|%s", meta.WorkspaceID.String(), meta.RequestID.String())
	mac := computeHMAC(sw.hmacKey, hmacInput)

	words := tokenize(text)
	var result strings.Builder

	for i, token := range words {
		lower := strings.ToLower(token.word)

		synonyms, hasSynonym := sw.synonymMap[lower]
		if hasSynonym && len(synonyms) > 0 {
			// Use byte at position (i mod 32) to select synonym index.
			byteIdx := i % len(mac)
			synonymIdx := int(mac[byteIdx]) % len(synonyms)
			replacement := synonyms[synonymIdx]

			// Preserve original capitalization.
			replacement = matchCase(token.word, replacement)

			result.WriteString(token.prefix)
			result.WriteString(replacement)
			result.WriteString(token.suffix)
		} else {
			result.WriteString(token.prefix)
			result.WriteString(token.word)
			result.WriteString(token.suffix)
		}
	}

	return result.String(), nil
}

// Detect checks whether text contains the expected synonym pattern for a
// known workspace and request ID. Returns (isDetected, confidence).
func (sw *SemanticWatermarker) Detect(text string, knownWorkspaceID uuid.UUID, knownRequestID uuid.UUID) (bool, float64) {
	hmacInput := fmt.Sprintf("%s|%s", knownWorkspaceID.String(), knownRequestID.String())
	mac := computeHMAC(sw.hmacKey, hmacInput)

	words := tokenize(text)
	totalOpportunities := 0
	matches := 0

	for i, token := range words {
		lower := strings.ToLower(token.word)

		// Check if this word is either a key or a synonym value.
		synonyms, isKey := sw.synonymMap[lower]
		if !isKey {
			// Check if the word is a synonym of some key.
			for key, syns := range sw.synonymMap {
				for _, s := range syns {
					if strings.ToLower(s) == lower {
						synonyms = syns
						// The expected choice for this position:
						byteIdx := i % len(mac)
						expectedIdx := int(mac[byteIdx]) % len(syns)
						if strings.ToLower(syns[expectedIdx]) == lower {
							matches++
						}
						totalOpportunities++
						_ = key
						goto nextWord
					}
				}
			}
			continue
		}

		// Word is a key in the synonym map.
		if len(synonyms) > 0 {
			totalOpportunities++
			byteIdx := i % len(mac)
			expectedIdx := int(mac[byteIdx]) % len(synonyms)
			if strings.ToLower(synonyms[expectedIdx]) == lower {
				matches++
			}
		}

	nextWord:
	}

	if totalOpportunities == 0 {
		return false, 0.0
	}

	confidence := float64(matches) / float64(totalOpportunities)
	return confidence > 0.65, confidence
}

// token represents a word with its surrounding whitespace/punctuation.
type token struct {
	prefix string // whitespace before the word
	word   string // the word itself
	suffix string // trailing punctuation
}

// tokenize splits text into tokens preserving whitespace and punctuation.
func tokenize(text string) []token {
	var tokens []token
	runes := []rune(text)
	i := 0

	for i < len(runes) {
		// Collect leading whitespace.
		var prefix strings.Builder
		for i < len(runes) && unicode.IsSpace(runes[i]) {
			prefix.WriteRune(runes[i])
			i++
		}

		if i >= len(runes) {
			if prefix.Len() > 0 {
				tokens = append(tokens, token{prefix: prefix.String()})
			}
			break
		}

		// Collect word characters.
		var word strings.Builder
		for i < len(runes) && !unicode.IsSpace(runes[i]) {
			word.WriteRune(runes[i])
			i++
		}

		w := word.String()
		if w == "" {
			continue
		}

		// Split trailing punctuation from the word.
		wordStr, suffixStr := splitTrailingPunct(w)

		tokens = append(tokens, token{
			prefix: prefix.String(),
			word:   wordStr,
			suffix: suffixStr,
		})
	}

	return tokens
}

func splitTrailingPunct(w string) (string, string) {
	runes := []rune(w)
	end := len(runes)

	for end > 0 && isPunct(runes[end-1]) {
		end--
	}

	if end == 0 {
		return w, ""
	}

	return string(runes[:end]), string(runes[end:])
}

func isPunct(r rune) bool {
	return r == '.' || r == ',' || r == ';' || r == ':' || r == '!' ||
		r == '?' || r == '"' || r == '\'' || r == ')' || r == ']' || r == '}'
}

func matchCase(original, replacement string) string {
	if len(original) == 0 || len(replacement) == 0 {
		return replacement
	}

	origRunes := []rune(original)
	if unicode.IsUpper(origRunes[0]) {
		allUpper := true
		for _, r := range origRunes {
			if unicode.IsLetter(r) && !unicode.IsUpper(r) {
				allUpper = false
				break
			}
		}
		if allUpper && len(origRunes) > 1 {
			return strings.ToUpper(replacement)
		}
		repRunes := []rune(replacement)
		repRunes[0] = unicode.ToUpper(repRunes[0])
		return string(repRunes)
	}

	return strings.ToLower(replacement)
}
