package brain

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/brevio/brevio/internal/llm"
)

// ErrInsufficientProposals is returned when fewer than 2 proposals are generated.
var ErrInsufficientProposals = errors.New("moa: fewer than 2 proposals generated")

// MoAProposer is a named LLM client used as a proposer in Mixture of Agents.
type MoAProposer struct {
	Client     llm.Client
	ProviderID string
	Model      string
}

// MoAFullConfig configures the multi-provider Mixture of Agents.
type MoAFullConfig struct {
	Proposers         []MoAProposer
	SynthesizerClient llm.Client
	SynthesizerModel  string
	Enabled           bool
	MaxProposalTokens int // default 1024
}

// MoAResult holds the output of a MoA invocation.
type MoAResult struct {
	Proposals   []string
	Synthesis   string
	ProposerIDs []string
}

// MixtureOfAgents runs N proposer models in parallel and fuses their outputs
// via a synthesizer model. Implements Wang et al. 2024 MoA pattern.
type MixtureOfAgents struct {
	cfg MoAFullConfig
}

// NewMixtureOfAgents creates a MoA instance.
func NewMixtureOfAgents(cfg MoAFullConfig) *MixtureOfAgents {
	if cfg.MaxProposalTokens == 0 {
		cfg.MaxProposalTokens = 1024
	}
	return &MixtureOfAgents{cfg: cfg}
}

// Propose runs all proposer models in parallel and synthesizes their outputs.
// Partial failures are non-fatal — fewer proposals are acceptable if >= 2 succeed.
func (m *MixtureOfAgents) Propose(ctx context.Context, systemPrompt, userPrompt string) (*MoAResult, error) {
	proposals := make([]string, 0, len(m.cfg.Proposers))
	proposerIDs := make([]string, 0, len(m.cfg.Proposers))
	var mu sync.Mutex

	var wg sync.WaitGroup
	for _, p := range m.cfg.Proposers {
		p := p // capture
		wg.Add(1)
		go func() {
			defer wg.Done()
			resp, _, err := p.Client.Generate(ctx, llm.GenerateRequest{
				Model:     p.Model,
				MaxTokens: m.cfg.MaxProposalTokens,
				System:    systemPrompt,
				Messages:  []llm.ChatMsg{{Role: "user", Content: userPrompt}},
			})
			if err != nil || resp == nil {
				return // non-fatal
			}
			mu.Lock()
			proposals = append(proposals, resp.Content)
			proposerIDs = append(proposerIDs, p.ProviderID)
			mu.Unlock()
		}()
	}
	wg.Wait()

	if len(proposals) < 2 {
		return nil, ErrInsufficientProposals
	}

	result, err := m.synthesize(ctx, userPrompt, proposals, proposerIDs)
	if err != nil {
		return nil, fmt.Errorf("moa synthesis: %w", err)
	}
	return result, nil
}

func (m *MixtureOfAgents) synthesize(
	ctx context.Context,
	originalPrompt string,
	proposals []string,
	proposerIDs []string,
) (*MoAResult, error) {
	if m.cfg.SynthesizerClient == nil {
		return nil, fmt.Errorf("moa: synthesizer client not configured")
	}

	var sb strings.Builder
	sb.WriteString("You are a synthesis judge. Select and merge the best elements from each proposal below to produce a single superior response.\n\n")
	sb.WriteString(fmt.Sprintf("Original user request:\n%s\n\n", originalPrompt))
	for i, p := range proposals {
		id := "unknown"
		if i < len(proposerIDs) {
			id = proposerIDs[i]
		}
		sb.WriteString(fmt.Sprintf("--- Proposal from %s ---\n%s\n\n", id, p))
	}
	sb.WriteString("Synthesized best response:")

	resp, _, err := m.cfg.SynthesizerClient.Generate(ctx, llm.GenerateRequest{
		Model:     m.cfg.SynthesizerModel,
		MaxTokens: 2048,
		Messages:  []llm.ChatMsg{{Role: "user", Content: sb.String()}},
	})
	if err != nil {
		return nil, err
	}
	return &MoAResult{
		Proposals:   proposals,
		Synthesis:   resp.Content,
		ProposerIDs: proposerIDs,
	}, nil
}
