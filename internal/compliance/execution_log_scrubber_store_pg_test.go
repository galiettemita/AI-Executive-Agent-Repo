package compliance

import (
	"context"
	"testing"
)

func TestNewPGExecutionLogPIIScrubStoreRequiresDSN(t *testing.T) {
	t.Parallel()

	store, err := NewPGExecutionLogPIIScrubStore(context.Background(), "")
	if err == nil {
		t.Fatalf("expected empty dsn error")
	}
	if store != nil {
		t.Fatalf("expected nil store on error")
	}
}
