package kg

import (
	"context"
	"fmt"
	"sort"
	"strings"
)

// Embedder is the minimal embedding interface needed by the retriever.
type Embedder interface {
	Embed(ctx context.Context, texts []string) ([][]float32, error)
}

// Retriever implements HippoRAG-style knowledge graph retrieval.
type Retriever struct {
	repo     *Repository
	embedder Embedder
	logger   Logger
}

func NewRetriever(repo *Repository, embedder Embedder, logger Logger) *Retriever {
	return &Retriever{repo: repo, embedder: embedder, logger: logger}
}

const (
	defaultMaxHops     = 2
	defaultSeedCount   = 5
	maxReturnedTriples = 15
)

// Query performs a HippoRAG query and returns a KGQueryResult.
func (r *Retriever) Query(
	ctx context.Context,
	workspaceID string,
	queryText string,
	maxHops int,
) (*KGQueryResult, error) {
	if strings.TrimSpace(queryText) == "" {
		return nil, nil
	}
	if maxHops <= 0 {
		maxHops = defaultMaxHops
	}

	embeddings, err := r.embedder.Embed(ctx, []string{queryText})
	if err != nil || len(embeddings) == 0 {
		r.logger.Warn("kg_retrieval: embed failed", "error", err)
		return nil, nil
	}
	queryVec := embeddings[0]

	seeds, err := r.repo.FindSeedEntities(ctx, workspaceID, queryVec, defaultSeedCount)
	if err != nil {
		return nil, fmt.Errorf("kg_retrieval: seed discovery: %w", err)
	}
	if len(seeds) == 0 {
		return nil, nil
	}

	visited := make(map[string]bool)
	triplesSeen := make(map[string]bool)
	var allTriples []Triple

	queue := make([]string, len(seeds))
	copy(queue, seeds)
	for _, s := range seeds {
		visited[strings.ToLower(strings.TrimSpace(s))] = true
	}

	for hop := 0; hop <= maxHops && len(queue) > 0; hop++ {
		var nextQueue []string

		for _, entity := range queue {
			triples, err := r.repo.GetTriplesForEntity(ctx, workspaceID, entity)
			if err != nil {
				r.logger.Warn("kg_retrieval: get triples error",
					"entity", entity, "error", err)
				continue
			}

			for _, t := range triples {
				if triplesSeen[t.ID] {
					continue
				}
				triplesSeen[t.ID] = true

				t.HopDistance = hop
				t.TraversalScore = t.Confidence * (1.0 / float64(hop+1))

				allTriples = append(allTriples, t)

				subjectNorm := strings.ToLower(strings.TrimSpace(t.Subject))
				entityNorm := strings.ToLower(strings.TrimSpace(entity))

				var neighbor string
				if subjectNorm == entityNorm {
					neighbor = t.Object
				} else {
					neighbor = t.Subject
				}

				neighborNorm := strings.ToLower(strings.TrimSpace(neighbor))
				if !visited[neighborNorm] {
					visited[neighborNorm] = true
					nextQueue = append(nextQueue, neighbor)
				}
			}
		}
		queue = nextQueue
	}

	if len(allTriples) == 0 {
		return nil, nil
	}

	sort.Slice(allTriples, func(i, j int) bool {
		return allTriples[i].TraversalScore > allTriples[j].TraversalScore
	})
	if len(allTriples) > maxReturnedTriples {
		allTriples = allTriples[:maxReturnedTriples]
	}

	snippet := FormatKGSnippet(allTriples)

	return &KGQueryResult{
		SeedEntities:   seeds,
		Triples:        allTriples,
		ContextSnippet: snippet,
		TraversalHops:  maxHops,
	}, nil
}

// FormatKGSnippet converts triples to a readable context block.
func FormatKGSnippet(triples []Triple) string {
	if len(triples) == 0 {
		return ""
	}

	var directFacts, expandedFacts []string
	for _, t := range triples {
		line := fmt.Sprintf("%s %s %s",
			t.Subject,
			strings.ReplaceAll(t.Predicate, "_", " "),
			t.Object,
		)
		if t.HopDistance == 0 {
			directFacts = append(directFacts, "• "+line)
		} else {
			expandedFacts = append(expandedFacts, "  ↳ "+line)
		}
	}

	var b strings.Builder
	b.WriteString("[Knowledge Graph — Entity Relationships]\n")
	for _, f := range directFacts {
		b.WriteString(f + "\n")
	}
	if len(expandedFacts) > 0 {
		b.WriteString("[Related]\n")
		for _, f := range expandedFacts {
			b.WriteString(f + "\n")
		}
	}
	return b.String()
}
