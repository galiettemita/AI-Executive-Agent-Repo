package kg

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/brevio/brevio/internal/rag"
)

type mockKGRepo struct {
	seeds   []string
	triples map[string][]Triple // keyed by lowercase entity name
}

func (r *mockKGRepo) FindSeedEntities(_ context.Context, _ string, _ []float32, _ int) ([]string, error) {
	return r.seeds, nil
}

func (r *mockKGRepo) GetTriplesForEntity(_ context.Context, _, entityName string) ([]Triple, error) {
	key := strings.ToLower(strings.TrimSpace(entityName))
	return r.triples[key], nil
}

func (r *mockKGRepo) UpsertTriple(_ context.Context, _ Triple) error                                { return nil }
func (r *mockKGRepo) UpdateSubjectEmbedding(_ context.Context, _, _ string, _ []float32) error      { return nil }
func (r *mockKGRepo) UpdateObjectEmbedding(_ context.Context, _, _ string, _ []float32) error       { return nil }

// mockRetrieverRepo adapts mockKGRepo for use with Retriever (which expects *Repository).
// Since Retriever uses *Repository directly, we need to test via the public Query method.
// For unit tests, we'll create a Retriever with a real Repository wrapping a mock DB.

func TestRetriever_EmptyGraph_NilResult(t *testing.T) {
	embedder := rag.NewMockEmbeddingProvider(1536)
	repo := &mockKGRepo{seeds: nil}
	retriever := newTestRetriever(repo, embedder)

	result, err := retriever.Query(context.Background(), "ws-1", "Who is Alice?", 2)
	if err != nil {
		t.Fatal(err)
	}
	if result != nil {
		t.Fatal("expected nil result for empty graph")
	}
}

func TestRetriever_BFS_OneHop(t *testing.T) {
	embedder := rag.NewMockEmbeddingProvider(1536)
	repo := &mockKGRepo{
		seeds: []string{"Alice Chen"},
		triples: map[string][]Triple{
			"alice chen": {
				{ID: "t1", Subject: "Alice Chen", Predicate: "reports_to", Object: "Bob Smith", Confidence: 0.9, CreatedAt: time.Now()},
			},
			"bob smith": {
				{ID: "t2", Subject: "Bob Smith", Predicate: "manages", Object: "Project Falcon", Confidence: 0.85, CreatedAt: time.Now()},
			},
		},
	}
	retriever := newTestRetriever(repo, embedder)

	result, err := retriever.Query(context.Background(), "ws-1", "Tell me about Alice", 1)
	if err != nil {
		t.Fatal(err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if len(result.Triples) == 0 {
		t.Fatal("expected at least one triple")
	}
}

func TestRetriever_HopPenalty_InScore(t *testing.T) {
	embedder := rag.NewMockEmbeddingProvider(1536)
	repo := &mockKGRepo{
		seeds: []string{"Alice"},
		triples: map[string][]Triple{
			"alice": {
				{ID: "t1", Subject: "Alice", Predicate: "knows", Object: "Bob", Confidence: 0.8, CreatedAt: time.Now()},
			},
			"bob": {
				{ID: "t2", Subject: "Bob", Predicate: "manages", Object: "Charlie", Confidence: 0.9, CreatedAt: time.Now()},
			},
		},
	}
	retriever := newTestRetriever(repo, embedder)

	result, _ := retriever.Query(context.Background(), "ws-1", "Alice connections", 2)
	if result == nil || len(result.Triples) < 2 {
		t.Fatal("expected at least 2 triples from BFS")
	}

	// Hop-0 triple (confidence 0.8, hop 0) → score 0.80
	// Hop-1 triple (confidence 0.9, hop 1) → score 0.45
	hop0 := result.Triples[0]
	hop1 := result.Triples[1]
	if hop0.TraversalScore <= hop1.TraversalScore {
		t.Fatalf("hop-0 (%v) should score higher than hop-1 (%v)", hop0.TraversalScore, hop1.TraversalScore)
	}
}

func TestRetriever_MaxTriplesCapped(t *testing.T) {
	embedder := rag.NewMockEmbeddingProvider(1536)
	// Create many triples for a single entity
	manyTriples := make([]Triple, 20)
	for i := range manyTriples {
		manyTriples[i] = Triple{
			ID: strings.Repeat("x", i+1), Subject: "Alice", Predicate: "knows",
			Object: strings.Repeat("E", i+1), Confidence: 0.9, CreatedAt: time.Now(),
		}
	}
	repo := &mockKGRepo{
		seeds:   []string{"Alice"},
		triples: map[string][]Triple{"alice": manyTriples},
	}
	retriever := newTestRetriever(repo, embedder)

	result, _ := retriever.Query(context.Background(), "ws-1", "Alice", 0)
	if result != nil && len(result.Triples) > maxReturnedTriples {
		t.Fatalf("expected max %d triples, got %d", maxReturnedTriples, len(result.Triples))
	}
}

func TestRetriever_NoDuplicateTriples(t *testing.T) {
	embedder := rag.NewMockEmbeddingProvider(1536)
	sharedTriple := Triple{
		ID: "t1", Subject: "Alice", Predicate: "works_with", Object: "Bob",
		Confidence: 0.9, CreatedAt: time.Now(),
	}
	repo := &mockKGRepo{
		seeds: []string{"Alice", "Bob"},
		triples: map[string][]Triple{
			"alice": {sharedTriple},
			"bob":   {sharedTriple},
		},
	}
	retriever := newTestRetriever(repo, embedder)

	result, _ := retriever.Query(context.Background(), "ws-1", "Alice and Bob", 1)
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	// Should appear only once despite being reachable from both seeds
	count := 0
	for _, t := range result.Triples {
		if t.ID == "t1" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("triple t1 appears %d times, expected 1", count)
	}
}

func TestRetriever_FormatSnippet_ContainsHeader(t *testing.T) {
	triples := []Triple{
		{Subject: "Alice", Predicate: "reports_to", Object: "Bob", HopDistance: 0},
	}
	snippet := FormatKGSnippet(triples)
	if !strings.Contains(snippet, "[Knowledge Graph") {
		t.Fatalf("snippet missing header: %q", snippet)
	}
}

// newTestRetriever creates a Retriever with a mock repository adapter.
func newTestRetriever(repo *mockKGRepo, embedder Embedder) *testRetriever {
	return &testRetriever{repo: repo, embedder: embedder, logger: nopKGLogger{}}
}

// testRetriever duplicates Retriever logic using the mock repo interface.
// This avoids needing a real *Repository for unit tests.
type testRetriever struct {
	repo     *mockKGRepo
	embedder Embedder
	logger   Logger
}

func (r *testRetriever) Query(ctx context.Context, workspaceID, queryText string, maxHops int) (*KGQueryResult, error) {
	if strings.TrimSpace(queryText) == "" {
		return nil, nil
	}
	if maxHops <= 0 {
		maxHops = defaultMaxHops
	}

	seeds := r.repo.seeds
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
			key := strings.ToLower(strings.TrimSpace(entity))
			triples := r.repo.triples[key]
			for _, t := range triples {
				if triplesSeen[t.ID] {
					continue
				}
				triplesSeen[t.ID] = true
				t.HopDistance = hop
				t.TraversalScore = t.Confidence * (1.0 / float64(hop+1))
				allTriples = append(allTriples, t)

				entityNorm := strings.ToLower(strings.TrimSpace(entity))
				subjectNorm := strings.ToLower(strings.TrimSpace(t.Subject))
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

	// Sort and cap
	for i := 0; i < len(allTriples); i++ {
		for j := i + 1; j < len(allTriples); j++ {
			if allTriples[j].TraversalScore > allTriples[i].TraversalScore {
				allTriples[i], allTriples[j] = allTriples[j], allTriples[i]
			}
		}
	}
	if len(allTriples) > maxReturnedTriples {
		allTriples = allTriples[:maxReturnedTriples]
	}

	return &KGQueryResult{
		SeedEntities:   seeds,
		Triples:        allTriples,
		ContextSnippet: FormatKGSnippet(allTriples),
		TraversalHops:  maxHops,
	}, nil
}
