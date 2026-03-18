// Package multihop implements IRCoT-style query decomposition and FLARE-style
// low-confidence retrieval for complex multi-entity queries.
package multihop

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/brevio/brevio/internal/llm"
	"github.com/brevio/brevio/internal/rag"
)

const maxHops = 3

// LowConfidenceSpan is a span in a partial answer that needs more retrieval evidence.
type LowConfidenceSpan struct {
	Text   string `json:"text"`
	Reason string `json:"reason"`
}

// HopResult captures the output of a single retrieval hop.
type HopResult struct {
	SubQuestion     string
	RetrievedChunks []rag.RetrievalResult
	PartialAnswer   string
}

// MultiHopPlan is the output of multi-hop retrieval.
type MultiHopPlan struct {
	OriginalQuery  string
	Hops           []HopResult
	FinalSynthesis string
	HopCount       int
}

// MultiHopPlanner orchestrates IRCoT-style decomposition and FLARE-style
// low-confidence retrieval over the existing RAG pipeline.
type MultiHopPlanner struct {
	ragService *rag.Service
	llmClient  llm.Client
	classifier *MultiHopClassifier
}

// NewMultiHopPlanner creates a multi-hop planner.
func NewMultiHopPlanner(ragService *rag.Service, llmClient llm.Client) *MultiHopPlanner {
	return &MultiHopPlanner{
		ragService: ragService,
		llmClient:  llmClient,
		classifier: NewMultiHopClassifier(),
	}
}

// Classifier returns the multi-hop classifier for external use.
func (p *MultiHopPlanner) Classifier() *MultiHopClassifier {
	return p.classifier
}

// Plan executes multi-hop retrieval.
// Uses IRCoT decomposition: decompose → retrieve per sub-question → synthesize.
// Uses FLARE for additional hops: detect low-confidence spans → retrieve → continue.
// Max 3 hops. Terminates early on empty delta or no low-confidence spans.
func (p *MultiHopPlanner) Plan(ctx context.Context, query, workspaceID string) (*MultiHopPlan, error) {
	subQuestions, err := p.decomposeQuery(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query decomposition failed: %w", err)
	}

	plan := &MultiHopPlan{OriginalQuery: query}
	accumulatedContext := ""

	for hop := 0; hop < maxHops; hop++ {
		var retrievalQuery string
		if hop < len(subQuestions) {
			retrievalQuery = subQuestions[hop]
		} else {
			// FLARE: detect low-confidence spans from last partial answer.
			if len(plan.Hops) == 0 {
				break
			}
			lastHop := plan.Hops[len(plan.Hops)-1]
			spans := p.detectLowConfidenceSpans(ctx, lastHop.PartialAnswer, accumulatedContext)
			if len(spans) == 0 {
				break // termination: no new low-confidence spans
			}
			retrievalQuery = spans[0].Text
		}

		// Use the existing RAG service Search method.
		retrieval := p.ragService.Search(workspaceID, fmt.Sprintf("hop_%d", hop+1), retrievalQuery, nil, 8)
		if len(retrieval.Results) == 0 || p.isDeltaEmpty(retrieval.Results, plan.Hops) {
			break // termination: empty delta
		}

		chunkTexts := make([]string, len(retrieval.Results))
		for i, c := range retrieval.Results {
			chunkTexts[i] = c.Snippet
		}
		accumulatedContext += "\n\n" + strings.Join(chunkTexts, "\n---\n")

		partialAnswer, err := p.generatePartialAnswer(ctx, query, retrievalQuery, accumulatedContext, hop)
		if err != nil {
			return nil, fmt.Errorf("hop %d partial answer failed: %w", hop+1, err)
		}
		plan.Hops = append(plan.Hops, HopResult{
			SubQuestion:     retrievalQuery,
			RetrievedChunks: retrieval.Results,
			PartialAnswer:   partialAnswer,
		})
	}

	finalSynthesis, err := p.synthesize(ctx, query, plan.Hops)
	if err != nil {
		return nil, fmt.Errorf("synthesis failed: %w", err)
	}
	plan.FinalSynthesis = finalSynthesis
	plan.HopCount = len(plan.Hops)
	return plan, nil
}

func (p *MultiHopPlanner) decomposeQuery(ctx context.Context, query string) ([]string, error) {
	if p.llmClient == nil {
		return []string{query}, nil
	}

	prompt := fmt.Sprintf(`Decompose this complex research query into at most 3 atomic sub-questions that can each be answered by a document search.
Return ONLY a JSON array of strings. No preamble.

Query: %s

Example output: ["What is X?", "How does X affect Y?", "What are the outcomes?"]`, query)

	resp, _, err := p.llmClient.Generate(ctx, llm.GenerateRequest{
		MaxTokens: 256,
		System:    "You are a query decomposition engine. Output only valid JSON arrays.",
		Messages:  []llm.ChatMsg{{Role: "user", Content: prompt}},
	})
	if err != nil {
		return []string{query}, nil // fallback to original query
	}
	content := strings.TrimSpace(resp.Content)
	if !strings.HasPrefix(content, "[") {
		return []string{query}, nil
	}
	var subQuestions []string
	if err := json.Unmarshal([]byte(content), &subQuestions); err != nil || len(subQuestions) == 0 {
		return []string{query}, nil
	}
	return subQuestions, nil
}

func (p *MultiHopPlanner) detectLowConfidenceSpans(ctx context.Context, partialAnswer, ctxText string) []LowConfidenceSpan {
	if p.llmClient == nil {
		return nil
	}
	sample := ctxText
	if len(sample) > 1000 {
		sample = sample[:1000] + "..."
	}
	prompt := fmt.Sprintf(`Identify spans in this partial answer that are low-confidence or need more retrieval evidence.
Return a JSON array of objects with "text" and "reason" fields. Return empty array [] if none.

Context used: %s
Partial answer: %s`, sample, partialAnswer)

	resp, _, err := p.llmClient.Generate(ctx, llm.GenerateRequest{
		MaxTokens: 256,
		System:    "You are a confidence evaluator. Output only valid JSON.",
		Messages:  []llm.ChatMsg{{Role: "user", Content: prompt}},
	})
	if err != nil || resp == nil {
		return nil
	}
	var spans []LowConfidenceSpan
	_ = json.Unmarshal([]byte(strings.TrimSpace(resp.Content)), &spans)
	return spans
}

func (p *MultiHopPlanner) generatePartialAnswer(ctx context.Context, originalQuery, subQuestion, ctxText string, hop int) (string, error) {
	if p.llmClient == nil {
		return "", fmt.Errorf("llm client not configured")
	}
	sample := ctxText
	if len(sample) > 4000 {
		sample = sample[:4000] + "..."
	}
	prompt := fmt.Sprintf(`Based on the retrieved context below, answer this sub-question as part of solving the original query.
Be specific and cite evidence from the context.

Original query: %s
Current sub-question (hop %d): %s

Retrieved context:
%s

Partial answer:`, originalQuery, hop+1, subQuestion, sample)

	resp, _, err := p.llmClient.Generate(ctx, llm.GenerateRequest{
		MaxTokens: 512,
		Messages:  []llm.ChatMsg{{Role: "user", Content: prompt}},
	})
	if err != nil {
		return "", err
	}
	return resp.Content, nil
}

func (p *MultiHopPlanner) synthesize(ctx context.Context, originalQuery string, hops []HopResult) (string, error) {
	if len(hops) == 0 {
		return "", fmt.Errorf("no hops to synthesize")
	}
	if p.llmClient == nil {
		return hops[len(hops)-1].PartialAnswer, nil
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Original query: %s\n\n", originalQuery))
	for i, h := range hops {
		sb.WriteString(fmt.Sprintf("Step %d — Sub-question: %s\nPartial answer: %s\n\n", i+1, h.SubQuestion, h.PartialAnswer))
	}
	sb.WriteString("Using the step-by-step reasoning above, provide a final comprehensive answer to the original query:")

	resp, _, err := p.llmClient.Generate(ctx, llm.GenerateRequest{
		MaxTokens: 1024,
		System:    "You are a synthesis engine. Produce a coherent final answer from the reasoning chain.",
		Messages:  []llm.ChatMsg{{Role: "user", Content: sb.String()}},
	})
	if err != nil {
		return "", err
	}
	return resp.Content, nil
}

func (p *MultiHopPlanner) isDeltaEmpty(newResults []rag.RetrievalResult, priorHops []HopResult) bool {
	seen := make(map[string]bool)
	for _, h := range priorHops {
		for _, c := range h.RetrievedChunks {
			seen[c.ChunkID] = true
		}
	}
	for _, c := range newResults {
		if !seen[c.ChunkID] {
			return false
		}
	}
	return true
}
