package colbert

import (
	"strings"
	"testing"
)

func TestSegment_TypicalChunk_3to5Segments(t *testing.T) {
	words := make([]string, 150)
	for i := range words {
		words[i] = "word"
	}
	text := strings.Join(words, " ")
	segs := Segment(text, 50, 10)
	if len(segs) < 3 || len(segs) > 5 {
		t.Fatalf("expected 3-5 segments for 150 words, got %d", len(segs))
	}
}

func TestSegment_ShortChunk_SingleSegment(t *testing.T) {
	text := "this is a short chunk with only a few words"
	segs := Segment(text, 50, 10)
	if len(segs) != 1 {
		t.Fatalf("expected 1 segment for short chunk, got %d", len(segs))
	}
	if segs[0] != text {
		t.Fatalf("single segment should be original text")
	}
}

func TestSegment_Overlap_ConsecutiveShare(t *testing.T) {
	words := make([]string, 120)
	for i := range words {
		words[i] = strings.Repeat("a", i+1)
	}
	text := strings.Join(words, " ")
	segs := Segment(text, 50, 10)
	if len(segs) < 2 {
		t.Fatal("expected at least 2 segments")
	}
	// Check overlap: last words of seg[0] should appear in start of seg[1]
	words0 := strings.Fields(segs[0])
	words1 := strings.Fields(segs[1])
	lastOfFirst := words0[len(words0)-1]
	found := false
	for _, w := range words1[:min(15, len(words1))] {
		if w == lastOfFirst {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected overlap between consecutive segments")
	}
}

func TestSegment_MaxCap_SixSegments(t *testing.T) {
	words := make([]string, 500)
	for i := range words {
		words[i] = "word"
	}
	text := strings.Join(words, " ")
	segs := Segment(text, 50, 10)
	if len(segs) > 6 {
		t.Fatalf("expected max 6 segments, got %d", len(segs))
	}
}

func TestNormalizeSegment_CollapsesSpaces(t *testing.T) {
	result := NormalizeSegment("hello  world   test")
	if result != "hello world test" {
		t.Fatalf("expected collapsed spaces, got %q", result)
	}
}

func TestNormalizeSegment_RemovesNonPrint(t *testing.T) {
	result := NormalizeSegment("hello\x00world")
	if strings.Contains(result, "\x00") {
		t.Fatal("non-printable char not removed")
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
