package canvas

import "testing"

func TestCanvasProtocolConstants(t *testing.T) {
	t.Parallel()

	if len(AllowedMessageTypes()) != 8 {
		t.Fatalf("unexpected message type count: %d", len(AllowedMessageTypes()))
	}
	if len(CanvasSurfaceTypes()) != 6 {
		t.Fatalf("unexpected surface type count: %d", len(CanvasSurfaceTypes()))
	}
	if CanvasInteractionRateLimitPerMinute() != 60 {
		t.Fatalf("unexpected canvas interaction rate limit: %d", CanvasInteractionRateLimitPerMinute())
	}
	if !IsValidCanvasMessageType("canvas.interaction") || IsValidCanvasMessageType("bad.message") {
		t.Fatal("unexpected canvas message-type validation result")
	}
}
