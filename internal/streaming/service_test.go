package streaming

import "testing"

func TestStreamingConfigLifecycle(t *testing.T) {
	s := NewService()

	cfg := s.UpsertConfig("ws_1", Config{
		AckEnabled:            true,
		TypingIndicator:       true,
		FirstByteSLAMillis:    450,
		ChunkSizeBytes:        4096,
		ProgressiveDisclosure: true,
	})
	if cfg.WorkspaceID != "ws_1" {
		t.Fatalf("unexpected workspace on config: %#v", cfg)
	}

	loaded, ok := s.GetConfig("ws_1")
	if !ok {
		t.Fatalf("expected streaming config lookup success")
	}
	if loaded.FirstByteSLAMillis != 450 {
		t.Fatalf("unexpected streaming config: %#v", loaded)
	}
}
