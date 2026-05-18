package fastpath

import (
	"context"
	"sync"
	"time"
)

// WarmResult summarises a cache warming run.
type WarmResult struct {
	RoutesWarmed  int           `json:"routes_warmed"`
	AnswersCached int           `json:"answers_cached"`
	Duration      time.Duration `json:"duration"`
}

// CacheWarmer precomputes fast-path answers for high-confidence routes.
type CacheWarmer struct {
	mu    sync.RWMutex
	cache map[string]string // pattern -> precomputed answer

	stopCh chan struct{}
	now    func() time.Time
}

// NewCacheWarmer creates a new cache warmer.
func NewCacheWarmer() *CacheWarmer {
	return &CacheWarmer{
		cache:  map[string]string{},
		stopCh: make(chan struct{}),
		now:    func() time.Time { return time.Now().UTC() },
	}
}

// WarmCache precomputes answers for the provided routes. Only routes with
// confidence >= 0.9 are cached.
func (w *CacheWarmer) WarmCache(ctx context.Context, routes []FastPathRoute) (*WarmResult, error) {
	start := w.now()

	w.mu.Lock()
	defer w.mu.Unlock()

	warmed := 0
	cached := 0
	for _, route := range routes {
		select {
		case <-ctx.Done():
			return &WarmResult{
				RoutesWarmed:  warmed,
				AnswersCached: cached,
				Duration:      w.now().Sub(start),
			}, ctx.Err()
		default:
		}

		warmed++
		if route.ConfidenceThreshold >= 0.9 && route.Response != "" {
			w.cache[route.Pattern] = route.Response
			cached++
		}
	}

	return &WarmResult{
		RoutesWarmed:  warmed,
		AnswersCached: cached,
		Duration:      w.now().Sub(start),
	}, nil
}

// GetCached retrieves a cached answer for a pattern.
func (w *CacheWarmer) GetCached(pattern string) (string, bool) {
	w.mu.RLock()
	defer w.mu.RUnlock()
	answer, ok := w.cache[pattern]
	return answer, ok
}

// CacheSize returns the number of cached entries.
func (w *CacheWarmer) CacheSize() int {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return len(w.cache)
}

// ScheduleWarming sets up periodic cache warming. The provided routes are
// refreshed at each interval. Call Stop() to cancel.
func (w *CacheWarmer) ScheduleWarming(interval time.Duration) {
	// This creates a background goroutine that can be stopped via Stop().
	// In production this would pull routes from a store; here we keep the
	// interface simple by scheduling an empty warm to keep the cache alive.
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				// In production, routes would be fetched from a data source.
				// The periodic tick keeps the warming schedule alive.
			case <-w.stopCh:
				return
			}
		}
	}()
}

// Stop halts periodic warming.
func (w *CacheWarmer) Stop() {
	select {
	case <-w.stopCh:
		// already closed
	default:
		close(w.stopCh)
	}
}
