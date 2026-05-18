package contextlayer

import (
	"strings"
	"testing"
)

func TestCompressNoCompressionNeeded(t *testing.T) {
	t.Parallel()

	cc := NewConversationCompressor()
	turns := []Turn{
		{Role: "user", Content: "Hello"},
		{Role: "assistant", Content: "Hi there"},
	}
	result := cc.Compress(turns, 10000)
	if len(result) != 2 {
		t.Fatalf("expected 2 uncompressed turns, got %d", len(result))
	}
	if result[0].OriginalTurnCount != 1 {
		t.Fatal("expected original turn count of 1")
	}
}

func TestCompressPreservesFirstAndLastTurns(t *testing.T) {
	t.Parallel()

	cc := NewConversationCompressor()
	turns := []Turn{
		{Role: "user", Content: "First message about the Project Alpha deadline"},
		{Role: "assistant", Content: "Middle response about scheduling"},
		{Role: "user", Content: "Middle question about budget"},
		{Role: "assistant", Content: "Another middle response about resources"},
		{Role: "user", Content: "Last message confirming the plan"},
	}
	result := cc.Compress(turns, 10) // very low budget forces compression
	if len(result) != 3 {
		t.Fatalf("expected 3 compressed segments (first + middle + last), got %d", len(result))
	}
	if !strings.Contains(result[0].Summary, "First message") {
		t.Fatal("expected first turn preserved")
	}
	if !strings.Contains(result[2].Summary, "Last message") {
		t.Fatal("expected last turn preserved")
	}
	if result[1].OriginalTurnCount != 3 {
		t.Fatalf("expected 3 compressed middle turns, got %d", result[1].OriginalTurnCount)
	}
}

func TestCompressEntityExtraction(t *testing.T) {
	t.Parallel()

	cc := NewConversationCompressor()
	turns := []Turn{
		{Role: "user", Content: "Talk to Alice about the Project"},
		{Role: "assistant", Content: "I contacted Bob regarding the Report"},
		{Role: "user", Content: "Great, thanks"},
	}
	result := cc.Compress(turns, 5)
	allEntities := map[string]struct{}{}
	for _, ct := range result {
		for _, e := range ct.EntityRefs {
			allEntities[e] = struct{}{}
		}
	}
	if _, ok := allEntities["Alice"]; !ok {
		t.Fatal("expected Alice in entity refs")
	}
	if _, ok := allEntities["Project"]; !ok {
		t.Fatal("expected Project in entity refs")
	}
}

func TestCompressEmptyTurns(t *testing.T) {
	t.Parallel()

	cc := NewConversationCompressor()
	result := cc.Compress(nil, 1000)
	if result != nil {
		t.Fatalf("expected nil for empty turns, got %v", result)
	}
}

func TestCompressTwoTurnsNoMiddle(t *testing.T) {
	t.Parallel()

	cc := NewConversationCompressor()
	turns := []Turn{
		{Role: "user", Content: "Hello there General Kenobi"},
		{Role: "assistant", Content: "You are a bold one"},
	}
	result := cc.Compress(turns, 1)
	if len(result) != 2 {
		t.Fatalf("expected 2 turns when only 2 turns provided, got %d", len(result))
	}
}
