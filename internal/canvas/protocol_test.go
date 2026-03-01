package canvas

import "testing"

func TestCanvasProtocolEnumerations(t *testing.T) {
	t.Parallel()

	msgTypes := SupportedMessageTypes()
	if len(msgTypes) != 8 {
		t.Fatalf("unexpected message type count: %d", len(msgTypes))
	}

	surfaces := SupportedSurfaceTypes()
	if len(surfaces) != 6 {
		t.Fatalf("unexpected surface type count: %d", len(surfaces))
	}
}
