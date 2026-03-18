package llm

import (
	"context"
	"fmt"
	"testing"
)

// mockProviderClient is a test double for Client.
type mockProviderClient struct {
	name    string
	failN   int
	callNum int
}

func (m *mockProviderClient) Generate(_ context.Context, _ GenerateRequest) (*GenerateResponse, *Usage, error) {
	m.callNum++
	if m.callNum <= m.failN {
		return nil, nil, fmt.Errorf("mock failure %d", m.callNum)
	}
	return &GenerateResponse{Content: "ok", ProviderID: m.name}, &Usage{}, nil
}

func (m *mockProviderClient) Stream(_ context.Context, _ GenerateRequest, out chan<- StreamChunk) {
	close(out)
}

func (m *mockProviderClient) ProviderName() string { return m.name }

func TestCircuitBreakerOpensAfter3Failures(t *testing.T) {
	t.Parallel()
	cb := &CircuitBreaker{}
	cb.RecordFailure()
	cb.RecordFailure()
	if cb.IsOpen() {
		t.Error("should not be open after 2 failures")
	}
	cb.RecordFailure()
	if !cb.IsOpen() {
		t.Error("should be open after 3 failures")
	}
}

func TestCircuitBreakerResetsOnSuccess(t *testing.T) {
	t.Parallel()
	cb := &CircuitBreaker{}
	cb.RecordFailure()
	cb.RecordFailure()
	cb.RecordSuccess()
	cb.RecordFailure()
	cb.RecordFailure()
	if cb.IsOpen() {
		t.Error("should not be open — success reset the counter")
	}
}

func TestRouterImageRoutesToGemini(t *testing.T) {
	t.Parallel()
	providers := map[string]*ProviderEntry{
		"anthropic": {Client: &mockProviderClient{name: "anthropic"}, Name: "anthropic", CB: &CircuitBreaker{}},
		"gemini":    {Client: &mockProviderClient{name: "gemini"}, Name: "gemini", CB: &CircuitBreaker{}},
	}
	router := NewMultiProviderRouter(providers)
	client, name, err := router.Route(context.Background(), GenerateRequest{}, RoutingContext{HasImages: true})
	if err != nil {
		t.Fatal(err)
	}
	if name != "gemini" {
		t.Errorf("expected gemini, got %s", name)
	}
	_ = client
}

func TestRouterLatencySensitiveRoutesToGroq(t *testing.T) {
	t.Parallel()
	providers := map[string]*ProviderEntry{
		"anthropic": {Client: &mockProviderClient{name: "anthropic"}, Name: "anthropic", CB: &CircuitBreaker{}},
		"groq":      {Client: &mockProviderClient{name: "groq"}, Name: "groq", CB: &CircuitBreaker{}},
	}
	router := NewMultiProviderRouter(providers)
	_, name, err := router.Route(context.Background(), GenerateRequest{}, RoutingContext{LatencySensitive: true})
	if err != nil {
		t.Fatal(err)
	}
	if name != "groq" {
		t.Errorf("expected groq, got %s", name)
	}
}

func TestRouterEURoutesToMistral(t *testing.T) {
	t.Parallel()
	providers := map[string]*ProviderEntry{
		"anthropic": {Client: &mockProviderClient{name: "anthropic"}, Name: "anthropic", CB: &CircuitBreaker{}},
		"mistral":   {Client: &mockProviderClient{name: "mistral"}, Name: "mistral", CB: &CircuitBreaker{}},
	}
	router := NewMultiProviderRouter(providers)
	_, name, err := router.Route(context.Background(), GenerateRequest{}, RoutingContext{WorkspaceEU: true})
	if err != nil {
		t.Fatal(err)
	}
	if name != "mistral" {
		t.Errorf("expected mistral, got %s", name)
	}
}

func TestRouterLocalRoutesToOllama(t *testing.T) {
	t.Parallel()
	providers := map[string]*ProviderEntry{
		"anthropic": {Client: &mockProviderClient{name: "anthropic"}, Name: "anthropic", CB: &CircuitBreaker{}},
		"ollama":    {Client: &mockProviderClient{name: "ollama"}, Name: "ollama", CB: &CircuitBreaker{}},
	}
	router := NewMultiProviderRouter(providers)
	_, name, err := router.Route(context.Background(), GenerateRequest{}, RoutingContext{WorkspaceLocal: true})
	if err != nil {
		t.Fatal(err)
	}
	if name != "ollama" {
		t.Errorf("expected ollama, got %s", name)
	}
}

func TestRouterFailoverOnCircuitOpen(t *testing.T) {
	t.Parallel()
	providers := map[string]*ProviderEntry{
		"anthropic": {Client: &mockProviderClient{name: "anthropic"}, Name: "anthropic", CB: &CircuitBreaker{}},
		"gemini":    {Client: &mockProviderClient{name: "gemini"}, Name: "gemini", CB: &CircuitBreaker{}},
		"groq":      {Client: &mockProviderClient{name: "groq"}, Name: "groq", CB: &CircuitBreaker{}},
	}
	// Open anthropic circuit.
	providers["anthropic"].CB.RecordFailure()
	providers["anthropic"].CB.RecordFailure()
	providers["anthropic"].CB.RecordFailure()

	router := NewMultiProviderRouter(providers)
	_, name, err := router.Route(context.Background(), GenerateRequest{}, RoutingContext{})
	if err != nil {
		t.Fatal(err)
	}
	if name != "gemini" {
		t.Errorf("expected gemini fallback, got %s", name)
	}
}

func TestGenerateWithFailover_RecoversFromFailure(t *testing.T) {
	t.Parallel()
	providers := map[string]*ProviderEntry{
		"anthropic": {Client: &mockProviderClient{name: "anthropic", failN: 1}, Name: "anthropic", CB: &CircuitBreaker{}},
		"gemini":    {Client: &mockProviderClient{name: "gemini"}, Name: "gemini", CB: &CircuitBreaker{}},
	}
	router := NewMultiProviderRouter(providers)
	resp, _, err := router.GenerateWithFailover(context.Background(), GenerateRequest{}, RoutingContext{})
	if err != nil {
		t.Fatal(err)
	}
	if resp.ProviderID != "gemini" {
		t.Errorf("expected gemini after anthropic failure, got %s", resp.ProviderID)
	}
}

func TestRouterDefaultFailoverChain(t *testing.T) {
	t.Parallel()
	providers := map[string]*ProviderEntry{
		"anthropic": {Client: &mockProviderClient{name: "anthropic"}, Name: "anthropic", CB: &CircuitBreaker{}},
		"gemini":    {Client: &mockProviderClient{name: "gemini"}, Name: "gemini", CB: &CircuitBreaker{}},
		"groq":      {Client: &mockProviderClient{name: "groq"}, Name: "groq", CB: &CircuitBreaker{}},
	}
	router := NewMultiProviderRouter(providers)
	candidates := router.buildCandidateList(RoutingContext{})
	if len(candidates) != 3 {
		t.Fatalf("expected 3 candidates, got %d", len(candidates))
	}
	if candidates[0] != "anthropic" || candidates[1] != "gemini" || candidates[2] != "groq" {
		t.Errorf("expected [anthropic, gemini, groq], got %v", candidates)
	}
}

func TestRouterCostOptimize(t *testing.T) {
	t.Parallel()
	providers := map[string]*ProviderEntry{
		"anthropic": {Client: &mockProviderClient{name: "anthropic"}, Name: "anthropic",
			Capabilities: ProviderCapability{CostPer1KTokens: 0.003}, CB: &CircuitBreaker{}},
		"gemini": {Client: &mockProviderClient{name: "gemini"}, Name: "gemini",
			Capabilities: ProviderCapability{CostPer1KTokens: 0.00015}, CB: &CircuitBreaker{}},
		"groq": {Client: &mockProviderClient{name: "groq"}, Name: "groq",
			Capabilities: ProviderCapability{CostPer1KTokens: 0.00059}, CB: &CircuitBreaker{}},
	}
	router := NewMultiProviderRouter(providers)
	candidates := router.buildCandidateList(RoutingContext{CostOptimize: true})
	if len(candidates) < 3 {
		t.Fatalf("expected 3 candidates, got %d", len(candidates))
	}
	// Cheapest should be first (gemini at 0.00015).
	if candidates[0] != "gemini" {
		t.Errorf("expected cheapest (gemini) first, got %s", candidates[0])
	}
}
