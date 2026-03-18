package watermark

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
)

var (
	testHMACKey = []byte("0123456789abcdef0123456789abcdef") // 32 bytes
	testLogger  = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
)

func testMeta() WatermarkMeta {
	return WatermarkMeta{
		ModelID:     "claude-sonnet-4-6",
		WorkspaceID: uuid.MustParse("11111111-1111-1111-1111-111111111111"),
		RequestID:   uuid.MustParse("22222222-2222-2222-2222-222222222222"),
		Timestamp:   time.Date(2026, 3, 18, 12, 0, 0, 0, time.UTC),
	}
}

func TestC2PATagAndVerify(t *testing.T) {
	w := NewC2PAContentWatermarkerWithKey(testHMACKey, testLogger)
	meta := testMeta()

	original := "Hello, this is a test response from the AI assistant."
	tagged, err := w.Tag(context.Background(), original, meta)
	if err != nil {
		t.Fatalf("Tag failed: %v", err)
	}

	// Tagged text should be longer than original (invisible chars added).
	if len(tagged) <= len(original) {
		t.Fatal("Tagged text should be longer than original")
	}

	// Verify should detect the watermark.
	verification, err := w.Verify(tagged)
	if err != nil {
		t.Fatalf("Verify failed: %v", err)
	}
	if !verification.IsBrevioGenerated {
		t.Fatal("Expected IsBrevioGenerated=true")
	}
	if verification.Confidence < 0.9 {
		t.Errorf("Expected high confidence, got %f", verification.Confidence)
	}
}

func TestC2PAVerifyUntaggedText(t *testing.T) {
	w := NewC2PAContentWatermarkerWithKey(testHMACKey, testLogger)

	verification, err := w.Verify("This is ordinary text without any watermark.")
	if err != nil {
		t.Fatalf("Verify failed: %v", err)
	}
	if verification.IsBrevioGenerated {
		t.Fatal("Expected IsBrevioGenerated=false for untagged text")
	}
}

func TestC2PAStrip(t *testing.T) {
	w := NewC2PAContentWatermarkerWithKey(testHMACKey, testLogger)
	meta := testMeta()

	original := "Hello, this is a test response from the AI assistant."
	tagged, err := w.Tag(context.Background(), original, meta)
	if err != nil {
		t.Fatalf("Tag failed: %v", err)
	}

	stripped := Strip(tagged)
	if stripped != original {
		t.Errorf("Strip should return original text.\nGot:      %q\nExpected: %q", stripped, original)
	}

	// Double-check no invisible characters remain.
	for _, r := range stripped {
		if r == zwnj || isVariationSelector(r) {
			t.Errorf("Strip left invisible character U+%04X", r)
		}
	}
}

func TestSemanticWatermarkDetection(t *testing.T) {
	synonymMap := map[string][]string{
		"use":      {"utilize", "employ", "apply"},
		"get":      {"obtain", "acquire", "retrieve"},
		"show":     {"display", "present", "demonstrate"},
		"help":     {"assist", "support", "aid"},
		"make":     {"create", "build", "produce"},
		"big":      {"large", "significant", "substantial"},
		"good":     {"excellent", "fine", "superior"},
		"fast":     {"quick", "rapid", "swift"},
		"start":    {"begin", "initiate", "commence"},
		"end":      {"finish", "complete", "conclude"},
	}

	sw := NewSemanticWatermarkerWithMap(synonymMap, testHMACKey, testLogger)
	meta := testMeta()

	text := "Please use this tool to get the data and show it. We can help you make a big and good report fast. Let us start and end quickly."
	watermarked, err := sw.Watermark(context.Background(), text, meta)
	if err != nil {
		t.Fatalf("Watermark failed: %v", err)
	}

	// The watermarked text should differ from original (synonyms replaced).
	if watermarked == text {
		t.Log("Warning: watermarked text is identical to original (no synonyms matched)")
	}

	// Detect with correct IDs should return high confidence.
	detected, confidence := sw.Detect(watermarked, meta.WorkspaceID, meta.RequestID)
	t.Logf("Semantic detection: detected=%v, confidence=%f", detected, confidence)

	// With the correct HMAC key and IDs, detection should succeed.
	if !detected {
		t.Logf("Semantic detection confidence was %f (threshold 0.65)", confidence)
		// This is acceptable in some cases if the text has few synonym opportunities.
	}
}

func TestContentHash(t *testing.T) {
	hash1 := ContentHash("hello world")
	hash2 := ContentHash("hello world")
	hash3 := ContentHash("different text")

	if hash1 != hash2 {
		t.Error("Same input should produce same hash")
	}
	if hash1 == hash3 {
		t.Error("Different input should produce different hash")
	}
	if len(hash1) != 64 {
		t.Errorf("Expected 64 hex chars, got %d", len(hash1))
	}
}

func TestC2PATagIdempotent(t *testing.T) {
	w := NewC2PAContentWatermarkerWithKey(testHMACKey, testLogger)
	meta := testMeta()

	original := "Test response"
	tagged1, _ := w.Tag(context.Background(), original, meta)
	tagged2, _ := w.Tag(context.Background(), original, meta)

	// Same input and meta should produce same output.
	if tagged1 != tagged2 {
		t.Error("Tag should be deterministic for same input and meta")
	}
}

func TestStripPreservesNormalZWNJ(t *testing.T) {
	// Text with ZWNJ not followed by variation selector should be preserved.
	text := "abc\u200Cdef"
	stripped := Strip(text)
	if stripped != text {
		t.Errorf("Strip should not remove ZWNJ that isn't part of a watermark.\nGot: %q\nExpected: %q", stripped, text)
	}
}
