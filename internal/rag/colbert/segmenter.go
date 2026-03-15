package colbert

import (
	"strings"
	"unicode"
)

// Segment splits a chunk into overlapping sub-sentence segments for ColBERT-lite.
// Returns 3-5 segments for a typical 150-word chunk. Returns original text as single
// segment for very short chunks.
func Segment(text string, targetWords, overlap int) []string {
	if targetWords <= 0 {
		targetWords = 50
	}
	if overlap < 0 {
		overlap = 10
	}

	words := strings.Fields(text)
	if len(words) <= targetWords {
		return []string{text}
	}

	var segments []string
	step := targetWords - overlap
	if step <= 0 {
		step = targetWords / 2
	}

	for start := 0; start < len(words); start += step {
		end := start + targetWords
		if end > len(words) {
			end = len(words)
		}
		segment := strings.Join(words[start:end], " ")
		segments = append(segments, segment)

		if end == len(words) {
			break
		}
		remaining := len(words) - (start + step)
		if remaining < 10 {
			break
		}
	}

	if len(segments) > 6 {
		segments = segments[:6]
	}

	return segments
}

// NormalizeSegment cleans a segment for embedding.
func NormalizeSegment(s string) string {
	var b strings.Builder
	prevSpace := false
	for _, r := range strings.TrimSpace(s) {
		if unicode.IsSpace(r) {
			if !prevSpace {
				b.WriteRune(' ')
				prevSpace = true
			}
		} else if unicode.IsPrint(r) {
			b.WriteRune(r)
			prevSpace = false
		}
	}
	return b.String()
}
