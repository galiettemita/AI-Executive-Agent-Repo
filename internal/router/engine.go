package router

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
	"time"
)

func generateID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// ---------------------------------------------------------------------------
// Model routing
// ---------------------------------------------------------------------------

// ModelConfig describes an available LLM model.
type ModelConfig struct {
	Provider        string
	Model           string
	CostPer1KTokens float64
	MaxTokens       int
	Latency         time.Duration
	Capabilities    []string
}

// RoutingRequest describes the requirements for a routing decision.
type RoutingRequest struct {
	TaskComplexity       float64
	RequiredCapabilities []string
	MaxLatencyMs         int
	BudgetConstraint     float64
}

// RoutingDecision is the engine's model selection.
type RoutingDecision struct {
	SelectedModel ModelConfig
	Reason        string
	EstimatedCost float64
	Fallback      *ModelConfig
}

// RoutingEngine selects the best model for a given request.
type RoutingEngine struct {
	mu     sync.RWMutex
	models []ModelConfig
}

// NewRoutingEngine creates a RoutingEngine with default model catalog.
func NewRoutingEngine() *RoutingEngine {
	return &RoutingEngine{
		models: []ModelConfig{
			{Provider: "anthropic", Model: "claude-opus-4-20250514", CostPer1KTokens: 0.015, MaxTokens: 200000, Latency: 2 * time.Second, Capabilities: []string{"reasoning", "coding", "analysis", "creative"}},
			{Provider: "anthropic", Model: "claude-sonnet-4-20250514", CostPer1KTokens: 0.003, MaxTokens: 200000, Latency: 800 * time.Millisecond, Capabilities: []string{"reasoning", "coding", "analysis"}},
			{Provider: "anthropic", Model: "claude-3-5-haiku-20241022", CostPer1KTokens: 0.001, MaxTokens: 200000, Latency: 400 * time.Millisecond, Capabilities: []string{"chat", "summarization"}},
			{Provider: "openai", Model: "gpt-4o", CostPer1KTokens: 0.005, MaxTokens: 128000, Latency: 1 * time.Second, Capabilities: []string{"reasoning", "coding", "analysis", "vision"}},
			{Provider: "openai", Model: "gpt-4o-mini", CostPer1KTokens: 0.00015, MaxTokens: 128000, Latency: 500 * time.Millisecond, Capabilities: []string{"chat", "summarization"}},
		},
	}
}

// ModelSelection picks the best model for the request.
func (re *RoutingEngine) ModelSelection(workspaceID string, req RoutingRequest) (*RoutingDecision, error) {
	if workspaceID == "" {
		return nil, errors.New("workspaceID is required")
	}

	re.mu.RLock()
	defer re.mu.RUnlock()

	type scored struct {
		model ModelConfig
		score float64
	}
	var candidates []scored

	for _, m := range re.models {
		// Check capability match.
		capSet := make(map[string]bool, len(m.Capabilities))
		for _, c := range m.Capabilities {
			capSet[c] = true
		}
		allMatch := true
		for _, req := range req.RequiredCapabilities {
			if !capSet[req] {
				allMatch = false
				break
			}
		}
		if !allMatch {
			continue
		}

		// Check latency constraint.
		if req.MaxLatencyMs > 0 && m.Latency.Milliseconds() > int64(req.MaxLatencyMs) {
			continue
		}

		// Check budget.
		if req.BudgetConstraint > 0 && m.CostPer1KTokens > req.BudgetConstraint {
			continue
		}

		// Score: balance cost efficiency with capability.
		// Higher complexity = prefer more capable (more expensive) models.
		costScore := 1.0 / (m.CostPer1KTokens + 0.0001)
		capScore := float64(len(m.Capabilities))
		score := capScore*req.TaskComplexity + costScore*(1-req.TaskComplexity)

		candidates = append(candidates, scored{m, score})
	}

	if len(candidates) == 0 {
		return nil, errors.New("no model matches the given requirements")
	}

	sort.Slice(candidates, func(i, j int) bool { return candidates[i].score > candidates[j].score })

	selected := candidates[0].model
	var fallback *ModelConfig
	if len(candidates) > 1 {
		fb := candidates[1].model
		fallback = &fb
	}

	// Estimate cost for 1000 tokens at the task complexity level.
	estimatedTokens := 500 + int(req.TaskComplexity*4500)
	estimatedCost := selected.CostPer1KTokens * float64(estimatedTokens) / 1000.0

	return &RoutingDecision{
		SelectedModel: selected,
		Reason:        fmt.Sprintf("best score %.2f for complexity %.2f", candidates[0].score, req.TaskComplexity),
		EstimatedCost: estimatedCost,
		Fallback:      fallback,
	}, nil
}

// ---------------------------------------------------------------------------
// Cost tracking
// ---------------------------------------------------------------------------

// ModelUsage tracks usage for a single model.
type ModelUsage struct {
	Tokens         int
	CostMicroCents int64
	Invocations    int
}

// UsageSummary aggregates usage across models.
type UsageSummary struct {
	TotalTokens         int
	TotalCostMicroCents int64
	ByModel             map[string]ModelUsage
}

// CostTracker records and reports token usage costs.
type CostTracker struct {
	mu    sync.RWMutex
	usage map[string]*UsageSummary // keyed by workspaceID
}

// NewCostTracker creates a CostTracker.
func NewCostTracker() *CostTracker {
	return &CostTracker{usage: make(map[string]*UsageSummary)}
}

// RecordUsage records a model invocation.
func (ct *CostTracker) RecordUsage(workspaceID, model string, tokens int, costMicroCents int64) {
	ct.mu.Lock()
	defer ct.mu.Unlock()

	summary, ok := ct.usage[workspaceID]
	if !ok {
		summary = &UsageSummary{ByModel: make(map[string]ModelUsage)}
		ct.usage[workspaceID] = summary
	}

	summary.TotalTokens += tokens
	summary.TotalCostMicroCents += costMicroCents
	mu := summary.ByModel[model]
	mu.Tokens += tokens
	mu.CostMicroCents += costMicroCents
	mu.Invocations++
	summary.ByModel[model] = mu
}

// GetUsageSummary returns usage stats for a workspace.
func (ct *CostTracker) GetUsageSummary(workspaceID string) *UsageSummary {
	ct.mu.RLock()
	defer ct.mu.RUnlock()
	s, ok := ct.usage[workspaceID]
	if !ok {
		return &UsageSummary{ByModel: make(map[string]ModelUsage)}
	}
	return s
}

// ---------------------------------------------------------------------------
// Provider failover
// ---------------------------------------------------------------------------

// ProviderHealth tracks the health of an LLM provider.
type ProviderHealth struct {
	Provider    string
	ErrorRate   float64
	AvgLatencyMs int
	Status      string // healthy, degraded, down
}

// ProviderFailoverService monitors provider health and fails over.
type ProviderFailoverService struct {
	mu        sync.RWMutex
	requests  map[string][]requestRecord
	providers map[string]*ProviderHealth
}

type requestRecord struct {
	Success   bool
	LatencyMs int
	Time      time.Time
}

// NewProviderFailoverService creates a ProviderFailoverService.
func NewProviderFailoverService() *ProviderFailoverService {
	return &ProviderFailoverService{
		requests: make(map[string][]requestRecord),
		providers: map[string]*ProviderHealth{
			"anthropic": {Provider: "anthropic", Status: "healthy"},
			"openai":    {Provider: "openai", Status: "healthy"},
		},
	}
}

// RecordRequest records a request outcome for a provider.
func (p *ProviderFailoverService) RecordRequest(provider string, success bool, latencyMs int) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.requests[provider] = append(p.requests[provider], requestRecord{
		Success:   success,
		LatencyMs: latencyMs,
		Time:      time.Now().UTC(),
	})

	// Recompute health from last 100 requests.
	records := p.requests[provider]
	if len(records) > 100 {
		records = records[len(records)-100:]
		p.requests[provider] = records
	}

	var failures int
	var totalLatency int
	for _, r := range records {
		if !r.Success {
			failures++
		}
		totalLatency += r.LatencyMs
	}

	health, ok := p.providers[provider]
	if !ok {
		health = &ProviderHealth{Provider: provider}
		p.providers[provider] = health
	}
	health.ErrorRate = float64(failures) / float64(len(records))
	health.AvgLatencyMs = totalLatency / len(records)

	switch {
	case health.ErrorRate >= 0.20:
		health.Status = "down"
	case health.ErrorRate >= 0.05:
		health.Status = "degraded"
	default:
		health.Status = "healthy"
	}
}

// GetHealth returns the current health of a provider.
func (p *ProviderFailoverService) GetHealth(provider string) *ProviderHealth {
	p.mu.RLock()
	defer p.mu.RUnlock()
	h, ok := p.providers[provider]
	if !ok {
		return &ProviderHealth{Provider: provider, Status: "unknown"}
	}
	return h
}

// ShouldFailover returns true if the provider error rate >= 5%.
func (p *ProviderFailoverService) ShouldFailover(provider string) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	h, ok := p.providers[provider]
	if !ok {
		return false
	}
	return h.ErrorRate >= 0.05
}

// SelectProvider returns the preferred provider if healthy, otherwise a
// failover.
func (p *ProviderFailoverService) SelectProvider(preferred string) string {
	p.mu.RLock()
	defer p.mu.RUnlock()

	h, ok := p.providers[preferred]
	if ok && h.ErrorRate < 0.05 {
		return preferred
	}

	// Find healthiest alternative.
	var best string
	bestRate := math.MaxFloat64
	for name, ph := range p.providers {
		if name == preferred {
			continue
		}
		if ph.ErrorRate < bestRate {
			bestRate = ph.ErrorRate
			best = name
		}
	}
	if best != "" {
		return best
	}
	return preferred // no alternatives
}

// ---------------------------------------------------------------------------
// Model cache
// ---------------------------------------------------------------------------

// CacheStats reports cache performance.
type CacheStats struct {
	Size           int
	HitRate        float64
	EvictionCount  int
}

type cacheEntry struct {
	Response  string
	ExpiresAt time.Time
}

// ModelCacheService caches model responses to avoid duplicate calls.
type ModelCacheService struct {
	mu       sync.RWMutex
	cache    map[string]*cacheEntry
	hits     int64
	misses   int64
	evictions int
}

// NewModelCacheService creates a ModelCacheService.
func NewModelCacheService() *ModelCacheService {
	return &ModelCacheService{cache: make(map[string]*cacheEntry)}
}

// CacheResponse stores a response with a TTL.
func (mc *ModelCacheService) CacheResponse(key, response string, ttl time.Duration) {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	mc.cache[key] = &cacheEntry{
		Response:  response,
		ExpiresAt: time.Now().UTC().Add(ttl),
	}
}

// GetCached retrieves a cached response. Returns ("", false) on miss.
func (mc *ModelCacheService) GetCached(key string) (string, bool) {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	entry, ok := mc.cache[key]
	if !ok {
		mc.misses++
		return "", false
	}
	if time.Now().UTC().After(entry.ExpiresAt) {
		delete(mc.cache, key)
		mc.evictions++
		mc.misses++
		return "", false
	}
	mc.hits++
	return entry.Response, true
}

// InvalidateCache removes entries matching the given key prefix pattern.
// Returns the number of entries removed.
func (mc *ModelCacheService) InvalidateCache(pattern string) int {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	count := 0
	for k := range mc.cache {
		if strings.HasPrefix(k, pattern) || pattern == "*" {
			delete(mc.cache, k)
			mc.evictions++
			count++
		}
	}
	return count
}

// GetCacheStats returns cache performance statistics.
func (mc *ModelCacheService) GetCacheStats() *CacheStats {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	total := mc.hits + mc.misses
	hitRate := 0.0
	if total > 0 {
		hitRate = float64(mc.hits) / float64(total)
	}

	// Purge expired entries from size count.
	size := 0
	now := time.Now().UTC()
	for _, e := range mc.cache {
		if now.Before(e.ExpiresAt) {
			size++
		}
	}

	return &CacheStats{
		Size:          size,
		HitRate:       hitRate,
		EvictionCount: mc.evictions,
	}
}

// compile-time check that generateID is used
var _ = generateID
