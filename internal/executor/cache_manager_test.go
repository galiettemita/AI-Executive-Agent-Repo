package executor

import (
	"testing"
	"time"
)

func TestCacheManagerReadWriteAndInvalidate(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 3, 1, 21, 0, 0, 0, time.UTC)
	manager := NewCacheManager()
	manager.SetNow(func() time.Time { return now })

	manager.WriteThrough("ws_1", "connector", "google_calendar", "value_1")
	value, ok := manager.Read("ws_1", "connector", "google_calendar")
	if !ok || value != "value_1" {
		t.Fatalf("unexpected cache read: ok=%v value=%s", ok, value)
	}

	now = now.Add(2 * time.Minute) // L1 expired, L2 still valid.
	value, ok = manager.Read("ws_1", "connector", "google_calendar")
	if !ok || value != "value_1" {
		t.Fatalf("unexpected cache read from L2 promotion: ok=%v value=%s", ok, value)
	}

	now = now.Add(10 * time.Minute) // L1 and L2 expired, L3 should repopulate.
	value, ok = manager.Read("ws_1", "connector", "google_calendar")
	if !ok || value != "value_1" {
		t.Fatalf("unexpected cache read from L3: ok=%v value=%s", ok, value)
	}

	manager.Invalidate("ws_1", "connector", "google_calendar")
	if _, ok := manager.Read("ws_1", "connector", "google_calendar"); ok {
		t.Fatal("expected cache miss after invalidation")
	}
}
