package audit

import (
	"context"
	"testing"
)

func TestNewPGSinkRequiresDSN(t *testing.T) {
	t.Parallel()

	_, err := NewPGSink(context.Background(), "")
	if err == nil {
		t.Fatal("expected error for empty dsn")
	}
}

func TestPGSinkRejectsUninitializedStore(t *testing.T) {
	t.Parallel()

	sink := &PGSink{}
	err := sink.PersistMutation(context.Background(), MutationEntry{
		ID:          "018f3f6a-9a0f-7cc6-8f2f-1f0f2d2f2d2f",
		WorkspaceID: "018f3f6a-9a0f-7cc6-8f2f-1f0f2d2f2d30",
		Action:      "feature_flag.upsert",
		Hash:        "abc",
		Timestamp:   "2026-03-03T00:00:00Z",
	})
	if err == nil {
		t.Fatal("expected uninitialized sink error")
	}
}
