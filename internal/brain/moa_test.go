package brain

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"

	"github.com/brevio/brevio/internal/llm"
)

// moaMockClient is a test double for llm.Client that returns a fixed response.
type moaMockClient struct {
	response string
	err      error
	provider string
}

func (m *moaMockClient) Generate(_ context.Context, _ llm.GenerateRequest) (*llm.GenerateResponse, *llm.Usage, error) {
	if m.err != nil {
		return nil, nil, m.err
	}
	return &llm.GenerateResponse{Content: m.response, ProviderID: m.provider}, &llm.Usage{}, nil
}

func (m *moaMockClient) Stream(_ context.Context, _ llm.GenerateRequest, out chan<- llm.StreamChunk) {
	close(out)
}

func TestMoA_ThreeProposals(t *testing.T) {
	t.Parallel()
	proposers := []MoAProposer{
		{Client: &moaMockClient{response: "proposal A from haiku", provider: "anthropic"}, ProviderID: "anthropic", Model: "haiku"},
		{Client: &moaMockClient{response: "proposal B from gemini", provider: "gemini"}, ProviderID: "gemini", Model: "gemini-flash"},
		{Client: &moaMockClient{response: "proposal C from groq", provider: "groq"}, ProviderID: "groq", Model: "llama"},
	}
	synthesizer := &moaMockClient{response: "synthesized best response", provider: "sonnet"}
	moa := NewMixtureOfAgents(MoAFullConfig{
		Proposers:         proposers,
		SynthesizerClient: synthesizer,
		SynthesizerModel:  "sonnet",
		Enabled:           true,
	})
	result, err := moa.Propose(context.Background(), "system", "user prompt")
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Proposals) < 2 {
		t.Errorf("expected >= 2 proposals, got %d", len(result.Proposals))
	}
	if result.Synthesis == "" {
		t.Error("expected non-empty synthesis")
	}
	if result.Synthesis != "synthesized best response" {
		t.Errorf("unexpected synthesis: %s", result.Synthesis)
	}
}

func TestMoA_InsufficientProposals(t *testing.T) {
	t.Parallel()
	proposers := []MoAProposer{
		{Client: &moaMockClient{err: fmt.Errorf("timeout")}, ProviderID: "p1"},
		{Client: &moaMockClient{err: fmt.Errorf("timeout")}, ProviderID: "p2"},
		{Client: &moaMockClient{response: "one proposal"}, ProviderID: "p3"},
	}
	moa := NewMixtureOfAgents(MoAFullConfig{
		Proposers:         proposers,
		SynthesizerClient: &moaMockClient{response: "synthesis"},
	})
	_, err := moa.Propose(context.Background(), "sys", "user")
	if !errors.Is(err, ErrInsufficientProposals) {
		t.Errorf("expected ErrInsufficientProposals, got %v", err)
	}
}

func TestMoA_TwoProposalsSuffice(t *testing.T) {
	t.Parallel()
	proposers := []MoAProposer{
		{Client: &moaMockClient{response: "A"}, ProviderID: "p1"},
		{Client: &moaMockClient{err: fmt.Errorf("fail")}, ProviderID: "p2"},
		{Client: &moaMockClient{response: "C"}, ProviderID: "p3"},
	}
	moa := NewMixtureOfAgents(MoAFullConfig{
		Proposers:         proposers,
		SynthesizerClient: &moaMockClient{response: "merged"},
	})
	result, err := moa.Propose(context.Background(), "sys", "user")
	if err != nil {
		t.Fatalf("2 proposals should suffice, got error: %v", err)
	}
	if len(result.Proposals) != 2 {
		t.Errorf("expected 2 proposals, got %d", len(result.Proposals))
	}
}

func TestMoATrigger_T0DoesNotFire(t *testing.T) {
	t.Setenv("FEATURE_MOA_ENABLED", "true")
	trigger := NewMoATrigger()
	if trigger.ShouldInvoke(0.5, 0.9, 1000, false) {
		t.Error("MoA should not fire on T0 latency budget")
	}
}

func TestMoATrigger_T2WithHighCouncilScore(t *testing.T) {
	t.Setenv("FEATURE_MOA_ENABLED", "true")
	trigger := NewMoATrigger()
	if !trigger.ShouldInvoke(0.7, 0.85, 5000, false) {
		t.Error("MoA should fire when council score > threshold on T2")
	}
}

func TestMoATrigger_RetryWithLowReActConfidence(t *testing.T) {
	t.Setenv("FEATURE_MOA_ENABLED", "true")
	trigger := NewMoATrigger()
	if !trigger.ShouldInvoke(0.5, 0.3, 5000, true) {
		t.Error("MoA should fire on retry with low ReAct confidence")
	}
}

func TestMoATrigger_DisabledByDefault(t *testing.T) {
	os.Unsetenv("FEATURE_MOA_ENABLED")
	trigger := NewMoATrigger()
	if trigger.ShouldInvoke(0.1, 0.99, 0, true) {
		t.Error("MoA should not fire when FEATURE_MOA_ENABLED is not set")
	}
}

func TestMoATrigger_UnconstrainedLatency(t *testing.T) {
	t.Setenv("FEATURE_MOA_ENABLED", "true")
	trigger := NewMoATrigger()
	if !trigger.ShouldInvoke(0.7, 0.85, 0, false) {
		t.Error("MoA should fire with unconstrained latency (0)")
	}
}
