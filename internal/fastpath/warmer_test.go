package fastpath

import (
	"context"
	"testing"
	"time"
)

func TestWarmCache(t *testing.T) {
	w := NewCacheWarmer()
	routes := []FastPathRoute{
		{Pattern: "hello", Response: "Hi there!", ConfidenceThreshold: 0.95},
		{Pattern: "weather", Response: "Check forecast", ConfidenceThreshold: 0.90},
	}
	result, err := w.WarmCache(context.Background(), routes)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.AnswersCached != 2 {
		t.Fatalf("expected 2 cached, got %d", result.AnswersCached)
	}
	if w.CacheSize() != 2 {
		t.Fatalf("expected cache size 2, got %d", w.CacheSize())
	}
}

func TestWarmCacheSkipsLowConfidence(t *testing.T) {
	w := NewCacheWarmer()
	routes := []FastPathRoute{
		{Pattern: "low", Response: "skip", ConfidenceThreshold: 0.5},
		{Pattern: "valid", Response: "ok", ConfidenceThreshold: 0.95},
	}
	result, _ := w.WarmCache(context.Background(), routes)
	if result.AnswersCached != 1 {
		t.Fatalf("expected 1 cached, got %d", result.AnswersCached)
	}
}

func TestGetCached(t *testing.T) {
	w := NewCacheWarmer()
	_, _ = w.WarmCache(context.Background(), []FastPathRoute{
		{Pattern: "hello", Response: "Hi!", ConfidenceThreshold: 0.95},
	})

	answer, ok := w.GetCached("hello")
	if !ok {
		t.Fatal("expected cache hit")
	}
	if answer != "Hi!" {
		t.Fatalf("expected 'Hi!', got %q", answer)
	}

	_, ok = w.GetCached("missing")
	if ok {
		t.Fatal("expected cache miss")
	}
}

func TestCacheSize(t *testing.T) {
	w := NewCacheWarmer()
	_, _ = w.WarmCache(context.Background(), []FastPathRoute{
		{Pattern: "a", Response: "b", ConfidenceThreshold: 0.95},
	})
	if w.CacheSize() != 1 {
		t.Fatalf("expected cache size 1, got %d", w.CacheSize())
	}
}

func TestScheduleAndStop(t *testing.T) {
	w := NewCacheWarmer()
	w.ScheduleWarming(50 * time.Millisecond)
	time.Sleep(100 * time.Millisecond)
	w.Stop()
	// Calling Stop again should not panic.
	w.Stop()
}

func TestWarmCacheContextCancellation(t *testing.T) {
	w := NewCacheWarmer()
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	routes := []FastPathRoute{
		{Pattern: "a", Response: "A", ConfidenceThreshold: 0.95},
		{Pattern: "b", Response: "B", ConfidenceThreshold: 0.95},
	}
	_, err := w.WarmCache(ctx, routes)
	if err == nil {
		t.Fatal("expected context cancellation error")
	}
}
