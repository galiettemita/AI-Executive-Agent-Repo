package browser

import (
	"crypto/md5"
	"fmt"
)

const stateChangeThreshold = 0.05 // 5% hash difference triggers LLM call

// ACIStateTracker detects meaningful visual changes between frames.
// Used to skip redundant LLM vision calls when the page hasn't changed.
type ACIStateTracker struct {
	lastHash string
}

// NewACIStateTracker creates a new state tracker.
func NewACIStateTracker() *ACIStateTracker {
	return &ACIStateTracker{}
}

// HasMeaningfulChange returns true if the new screenshot differs enough
// from the previous one to warrant an LLM vision call.
// Always returns true for the first frame.
func (t *ACIStateTracker) HasMeaningfulChange(screenshotBytes []byte) bool {
	// Hash the first 4KB of the screenshot (fast, captures header/layout changes).
	sampleSize := 4096
	if len(screenshotBytes) < sampleSize {
		sampleSize = len(screenshotBytes)
	}
	if sampleSize == 0 {
		return true
	}
	hash := fmt.Sprintf("%x", md5.Sum(screenshotBytes[:sampleSize]))

	if t.lastHash == "" {
		t.lastHash = hash
		return true // always process first frame
	}

	// Compute Hamming distance ratio between old and new hash (hex strings).
	diffBits := hammingDistanceHex(t.lastHash, hash)
	totalBits := len(t.lastHash) * 4 // each hex char = 4 bits
	if totalBits == 0 {
		return true
	}
	ratio := float64(diffBits) / float64(totalBits)

	if ratio > stateChangeThreshold {
		t.lastHash = hash
		return true
	}
	return false
}

// hammingDistanceHex counts the number of differing bits between two equal-length hex strings.
func hammingDistanceHex(a, b string) int {
	if len(a) != len(b) {
		return len(a) * 4
	}
	bits := 0
	for i := range a {
		xor := hexCharToByte(a[i]) ^ hexCharToByte(b[i])
		bits += countBits(xor)
	}
	return bits
}

func hexCharToByte(c byte) byte {
	switch {
	case c >= '0' && c <= '9':
		return c - '0'
	case c >= 'a' && c <= 'f':
		return c - 'a' + 10
	default:
		return 0
	}
}

func countBits(b byte) int {
	count := 0
	for b != 0 {
		count += int(b & 1)
		b >>= 1
	}
	return count
}
