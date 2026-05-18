package executor

import (
	"sync"
)

// RateLimitConfig defines rate limiting parameters for a provider.
type RateLimitConfig struct {
	Provider      string
	MaxConcurrent int
	WindowMs      int
}

// providerSemaphore tracks concurrent usage for a provider.
type providerSemaphore struct {
	config  RateLimitConfig
	current int
}

// RateCoordinator coordinates rate limiting across providers using semaphores.
type RateCoordinator struct {
	mu        sync.Mutex
	providers map[string]*providerSemaphore
}

// NewRateCoordinator creates a new RateCoordinator.
func NewRateCoordinator() *RateCoordinator {
	return &RateCoordinator{
		providers: map[string]*providerSemaphore{},
	}
}

// SetLimit sets the rate limit configuration for a provider.
func (rc *RateCoordinator) SetLimit(provider string, config RateLimitConfig) {
	rc.mu.Lock()
	defer rc.mu.Unlock()

	if config.MaxConcurrent <= 0 {
		config.MaxConcurrent = 10
	}
	if config.WindowMs <= 0 {
		config.WindowMs = 1000
	}

	existing, ok := rc.providers[provider]
	if ok {
		existing.config = config
	} else {
		rc.providers[provider] = &providerSemaphore{
			config:  config,
			current: 0,
		}
	}
}

// Acquire attempts to acquire a slot for the given provider.
// Returns true if the slot was acquired, false if the provider is at capacity.
func (rc *RateCoordinator) Acquire(provider string) bool {
	rc.mu.Lock()
	defer rc.mu.Unlock()

	sem, ok := rc.providers[provider]
	if !ok {
		// No limit configured; create a default one.
		sem = &providerSemaphore{
			config: RateLimitConfig{
				Provider:      provider,
				MaxConcurrent: 10,
				WindowMs:      1000,
			},
			current: 0,
		}
		rc.providers[provider] = sem
	}

	if sem.current >= sem.config.MaxConcurrent {
		return false
	}

	sem.current++
	return true
}

// Release releases a slot for the given provider.
func (rc *RateCoordinator) Release(provider string) {
	rc.mu.Lock()
	defer rc.mu.Unlock()

	sem, ok := rc.providers[provider]
	if !ok {
		return
	}
	if sem.current > 0 {
		sem.current--
	}
}

// CurrentUsage returns the current concurrent usage for a provider.
func (rc *RateCoordinator) CurrentUsage(provider string) int {
	rc.mu.Lock()
	defer rc.mu.Unlock()

	sem, ok := rc.providers[provider]
	if !ok {
		return 0
	}
	return sem.current
}
