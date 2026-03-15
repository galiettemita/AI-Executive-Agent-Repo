package eval

import (
	"context"
	"fmt"
	"math"
	"strings"
)

// RAGEvalLLMClient is the minimal LLM interface for eval graders.
type RAGEvalLLMClient interface {
	Complete(ctx context.Context, system, user string) (string, error)
}

// RAGEvalEmbedder is the minimal embedding interface for relevance grading.
type RAGEvalEmbedder interface {
	Embed(ctx context.Context, texts []string) ([][]float32, error)
}

// RAGChunk represents a retrieved chunk for eval grading.
type RAGChunk struct {
	ID        string
	Content   string
	Embedding []float32
	Score     float64
}

// FaithfulnessGrader measures whether retrieved content supports the expected answer.
type FaithfulnessGrader struct {
	llm RAGEvalLLMClient
}

func NewFaithfulnessGrader(llm RAGEvalLLMClient) *FaithfulnessGrader {
	return &FaithfulnessGrader{llm: llm}
}

func (g *FaithfulnessGrader) Grade(
	ctx context.Context,
	query, expectedAnswer string,
	retrievedChunks []RAGChunk,
) (float64, error) {
	if len(retrievedChunks) == 0 {
		return 0, nil
	}

	var contextBuilder strings.Builder
	limit := 3
	if len(retrievedChunks) < limit {
		limit = len(retrievedChunks)
	}
	for i, chunk := range retrievedChunks[:limit] {
		contextBuilder.WriteString(fmt.Sprintf("[Chunk %d]: %s\n\n", i+1, chunk.Content))
	}

	systemPrompt := `You are an evaluation judge for a retrieval system.
Score whether the provided retrieved context FAITHFULLY supports the expected answer.
1.0 = context contains all needed information.
0.5 = context partially supports the answer.
0.0 = context does not support the answer at all.
Output ONLY a decimal number between 0.0 and 1.0. No explanation.`

	userPrompt := fmt.Sprintf(
		"Query: %s\n\nExpected answer: %s\n\nRetrieved context:\n%s\n\nFaithfulness score:",
		query, expectedAnswer, contextBuilder.String(),
	)

	raw, err := g.llm.Complete(ctx, systemPrompt, userPrompt)
	if err != nil {
		return 0, fmt.Errorf("faithfulness LLM call: %w", err)
	}

	var score float64
	if _, err := fmt.Sscanf(strings.TrimSpace(raw), "%f", &score); err != nil {
		return 0, fmt.Errorf("faithfulness parse score '%s': %w", raw, err)
	}
	return math.Max(0, math.Min(1, score)), nil
}

// RelevanceGrader measures average cosine similarity between query and retrieved chunks.
type RelevanceGrader struct {
	embedder RAGEvalEmbedder
}

func NewRelevanceGrader(embedder RAGEvalEmbedder) *RelevanceGrader {
	return &RelevanceGrader{embedder: embedder}
}

func (g *RelevanceGrader) Grade(
	ctx context.Context,
	query string,
	retrievedChunks []RAGChunk,
) (float64, error) {
	if len(retrievedChunks) == 0 {
		return 0, nil
	}

	queryEmbeddings, err := g.embedder.Embed(ctx, []string{query})
	if err != nil {
		return 0, fmt.Errorf("relevance embed query: %w", err)
	}
	queryVec := queryEmbeddings[0]

	var totalSim float64
	counted := 0
	for _, chunk := range retrievedChunks {
		if len(chunk.Embedding) == 0 {
			// Embed the chunk content on-the-fly if no stored embedding
			chunkEmbs, err := g.embedder.Embed(ctx, []string{chunk.Content})
			if err != nil || len(chunkEmbs) == 0 {
				continue
			}
			chunk.Embedding = chunkEmbs[0]
		}
		sim := cosineSimF32(queryVec, chunk.Embedding)
		totalSim += sim
		counted++
	}

	if counted == 0 {
		return 0, nil
	}
	return totalSim / float64(counted), nil
}

func cosineSimF32(a, b []float32) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	var dot, magA, magB float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
		magA += float64(a[i]) * float64(a[i])
		magB += float64(b[i]) * float64(b[i])
	}
	if magA == 0 || magB == 0 {
		return 0
	}
	return dot / (math.Sqrt(magA) * math.Sqrt(magB))
}
