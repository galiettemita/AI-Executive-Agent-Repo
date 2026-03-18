package llm

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"
)

// ProviderCapability describes what a provider supports.
type ProviderCapability struct {
	SupportsImages  bool
	MaxContextK     int
	SpeedTier       int     // 0=fastest, 1=fast, 2=normal
	CostPer1KTokens float64 // USD
	EUDataResidency bool
	SupportsLocal   bool
}

// CircuitBreaker tracks failures and opens the circuit after 3 consecutive failures.
type CircuitBreaker struct {
	mu         sync.Mutex
	failures   int
	open       bool
	halfOpenAt time.Time
}

// RecordFailure increments the failure counter. Opens circuit after 3 failures.
func (cb *CircuitBreaker) RecordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.failures++
	if cb.failures >= 3 {
		cb.open = true
		cb.halfOpenAt = time.Now().Add(30 * time.Second)
	}
}

// RecordSuccess resets the failure counter and closes the circuit.
func (cb *CircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.failures = 0
	cb.open = false
}

// IsOpen returns true if the circuit is open (provider should be skipped).
// Transitions to half-open after 30s to allow a probe request.
func (cb *CircuitBreaker) IsOpen() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	if cb.open && time.Now().After(cb.halfOpenAt) {
		cb.open = false // half-open: allow one probe
	}
	return cb.open
}

// ProviderEntry holds a client, its capabilities, and circuit breaker state.
type ProviderEntry struct {
	Client       Client
	Name         string
	Capabilities ProviderCapability
	CB           *CircuitBreaker
}

// RoutingContext carries per-request routing signals.
type RoutingContext struct {
	HasImages        bool
	WorkspaceEU      bool
	WorkspaceLocal   bool
	LatencyBudgetMs  int
	LatencySensitive bool // T0/T1 fast path
	CostOptimize     bool
}

// MultiProviderRouter selects the best provider per request with circuit-broken failover.
type MultiProviderRouter struct {
	providers map[string]*ProviderEntry
	primary   string
	fallback  string
	emergency string
}

// NewMultiProviderRouter creates a router from a map of provider entries.
func NewMultiProviderRouter(providers map[string]*ProviderEntry) *MultiProviderRouter {
	return &MultiProviderRouter{
		providers: providers,
		primary:   "anthropic",
		fallback:  "gemini",
		emergency: "groq",
	}
}

func (r *MultiProviderRouter) buildCandidateList(rc RoutingContext) []string {
	if rc.WorkspaceLocal {
		return []string{"ollama", r.primary, r.fallback, r.emergency}
	}
	if rc.WorkspaceEU {
		return []string{"mistral", r.primary, r.fallback, r.emergency}
	}
	if rc.HasImages {
		return []string{"gemini", r.primary, r.fallback}
	}
	if rc.LatencySensitive || (rc.LatencyBudgetMs > 0 && rc.LatencyBudgetMs < 2000) {
		return []string{"groq", r.primary, r.fallback}
	}
	if rc.CostOptimize {
		return r.sortByCost()
	}
	return []string{r.primary, r.fallback, r.emergency}
}

func (r *MultiProviderRouter) sortByCost() []string {
	type ce struct {
		name string
		cost float64
	}
	entries := make([]ce, 0, len(r.providers))
	for name, p := range r.providers {
		entries = append(entries, ce{name, p.Capabilities.CostPer1KTokens})
	}
	// Insertion sort (small N).
	for i := 1; i < len(entries); i++ {
		for j := i; j > 0 && entries[j].cost < entries[j-1].cost; j-- {
			entries[j], entries[j-1] = entries[j-1], entries[j]
		}
	}
	names := make([]string, len(entries))
	for i, e := range entries {
		names[i] = e.name
	}
	return names
}

// Route selects the first available provider from the candidate list.
func (r *MultiProviderRouter) Route(ctx context.Context, req GenerateRequest, rc RoutingContext) (Client, string, error) {
	for _, name := range r.buildCandidateList(rc) {
		entry, ok := r.providers[name]
		if !ok || entry == nil || entry.CB.IsOpen() {
			continue
		}
		return entry.Client, name, nil
	}
	return nil, "", fmt.Errorf("no available LLM provider: all candidates exhausted or circuit open")
}

// GenerateWithFailover wraps Route with automatic failover on error.
func (r *MultiProviderRouter) GenerateWithFailover(
	ctx context.Context, req GenerateRequest, rc RoutingContext,
) (*GenerateResponse, *Usage, error) {
	for _, name := range r.buildCandidateList(rc) {
		entry, ok := r.providers[name]
		if !ok || entry == nil || entry.CB.IsOpen() {
			continue
		}
		resp, usage, err := entry.Client.Generate(ctx, req)
		if err != nil {
			entry.CB.RecordFailure()
			continue
		}
		entry.CB.RecordSuccess()
		if resp != nil {
			resp.ProviderID = name
		}
		return resp, usage, nil
	}
	return nil, nil, fmt.Errorf("all providers failed")
}

// ProviderCount returns the number of registered providers.
func (r *MultiProviderRouter) ProviderCount() int {
	return len(r.providers)
}

// HasProvider returns true if a provider is registered.
func (r *MultiProviderRouter) HasProvider(name string) bool {
	_, ok := r.providers[name]
	return ok
}

// BuildDefaultRouter constructs the router from environment variables.
// Anthropic is required; all other providers are optional.
func BuildDefaultRouter() (*MultiProviderRouter, error) {
	providers := make(map[string]*ProviderEntry)

	// Anthropic — required.
	anthropicKey := os.Getenv("ANTHROPIC_API_KEY")
	if anthropicKey == "" {
		return nil, fmt.Errorf("ANTHROPIC_API_KEY required for multi-provider router")
	}
	ac, err := NewAnthropicClient(AnthropicConfig{APIKey: anthropicKey})
	if err != nil {
		return nil, fmt.Errorf("anthropic client: %w", err)
	}
	providers["anthropic"] = &ProviderEntry{
		Client: ac, Name: "anthropic",
		Capabilities: ProviderCapability{SupportsImages: true, MaxContextK: 200, SpeedTier: 2, CostPer1KTokens: 0.003},
		CB:           &CircuitBreaker{},
	}

	// Gemini — optional.
	if os.Getenv("GEMINI_API_KEY") != "" {
		gc, err := NewGeminiClient()
		if err == nil {
			providers["gemini"] = &ProviderEntry{
				Client: gc, Name: "gemini",
				Capabilities: ProviderCapability{SupportsImages: true, MaxContextK: 1000, SpeedTier: 1, CostPer1KTokens: 0.00015},
				CB:           &CircuitBreaker{},
			}
		}
	}

	// Groq — optional.
	if os.Getenv("GROQ_API_KEY") != "" {
		gc, err := NewGroqClient()
		if err == nil {
			providers["groq"] = &ProviderEntry{
				Client: gc, Name: "groq",
				Capabilities: ProviderCapability{SupportsImages: false, MaxContextK: 128, SpeedTier: 0, CostPer1KTokens: 0.00059},
				CB:           &CircuitBreaker{},
			}
		}
	}

	// Mistral — optional, EU data residency.
	if os.Getenv("MISTRAL_API_KEY") != "" {
		mc, err := NewMistralClient()
		if err == nil {
			providers["mistral"] = &ProviderEntry{
				Client: mc, Name: "mistral",
				Capabilities: ProviderCapability{SupportsImages: false, MaxContextK: 128, SpeedTier: 2, CostPer1KTokens: 0.002, EUDataResidency: true},
				CB:           &CircuitBreaker{},
			}
		}
	}

	// Ollama — optional, local inference.
	if os.Getenv("OLLAMA_BASE_URL") != "" || strings.EqualFold(os.Getenv("PRIVACY_MODE"), "strict") {
		oc := NewOllamaClient("")
		providers["ollama"] = &ProviderEntry{
			Client: oc, Name: "ollama",
			Capabilities: ProviderCapability{SupportsImages: false, MaxContextK: 128, SpeedTier: 2, CostPer1KTokens: 0.0, SupportsLocal: true},
			CB:           &CircuitBreaker{},
		}
	}

	return NewMultiProviderRouter(providers), nil
}
