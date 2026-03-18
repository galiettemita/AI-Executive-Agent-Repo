package iso27001

import (
	"context"
	"log/slog"
	"os"
	"testing"
)

func TestISO27001ControlCount(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	collector := NewISO27001Collector(nil, logger)
	evidences, err := collector.CollectAll(context.Background())
	if err != nil {
		t.Fatalf("CollectAll failed: %v", err)
	}
	if len(evidences) < 15 {
		t.Errorf("Expected >= 15 ISO 27001 controls, got %d", len(evidences))
	}
	t.Logf("ISO 27001 controls collected: %d", len(evidences))
	for _, ev := range evidences {
		t.Logf("  %s: pass=%v", ev.ControlID, ev.Pass)
	}
}
