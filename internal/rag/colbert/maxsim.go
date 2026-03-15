package colbert

import (
	"context"
	"math"
	"sort"
)

// SubVector is one sub-sentence embedding for a chunk.
type SubVector struct {
	ChunkID      string
	SegmentIndex int
	SegmentText  string
	Embedding    []float32
}

// SubvectorRepository fetches stored sub-sentence vectors for candidate chunks.
type SubvectorRepository interface {
	GetSubvectors(ctx context.Context, chunkIDs []string) (map[string][]SubVector, error)
	StoreSubvectors(ctx context.Context, chunkID, workspaceID, collectionID string, segments []SubVector) error
	MarkSubvectorsGenerated(ctx context.Context, chunkID string, count int) error
}

// MaxSimEmbedder is the embedding interface needed by the MaxSim scorer.
type MaxSimEmbedder interface {
	Embed(ctx context.Context, texts []string) ([][]float32, error)
}

// Logger is the minimal logging interface.
type Logger interface {
	Warn(msg string, args ...any)
	Debug(msg string, args ...any)
	Error(msg string, args ...any)
}

// ScoredChunk wraps a chunk with retrieval scoring metadata.
type ScoredChunk struct {
	ChunkID     string
	Content     string
	HybridScore float64
	RerankScore float64
	MaxSimScore float64
	FinalScore  float64
}

// MaxSimScorer computes ColBERT-style MaxSim scores for candidate chunks.
type MaxSimScorer struct {
	repo     SubvectorRepository
	embedder MaxSimEmbedder
	logger   Logger
}

func NewMaxSimScorer(repo SubvectorRepository, embedder MaxSimEmbedder, logger Logger) *MaxSimScorer {
	return &MaxSimScorer{repo: repo, embedder: embedder, logger: logger}
}

// ScoreChunks reranks candidates using MaxSim scoring.
// Returns original order on any failure.
func (s *MaxSimScorer) ScoreChunks(
	ctx context.Context,
	query string,
	candidates []ScoredChunk,
) []ScoredChunk {
	if len(candidates) == 0 {
		return candidates
	}

	querySegments := Segment(query, 30, 5)
	for i, seg := range querySegments {
		querySegments[i] = NormalizeSegment(seg)
	}

	queryEmbeddings, err := s.embedder.Embed(ctx, querySegments)
	if err != nil {
		s.logger.Warn("colbert: query segment embed failed; returning original order",
			"error", err)
		return candidates
	}

	chunkIDs := make([]string, len(candidates))
	for i, c := range candidates {
		chunkIDs[i] = c.ChunkID
	}
	subvectors, err := s.repo.GetSubvectors(ctx, chunkIDs)
	if err != nil {
		s.logger.Warn("colbert: fetch subvectors failed; returning original order",
			"error", err)
		return candidates
	}

	for i := range candidates {
		docVecs, ok := subvectors[candidates[i].ChunkID]
		if !ok || len(docVecs) == 0 {
			candidates[i].FinalScore = candidates[i].HybridScore
			continue
		}

		docEmbeddings := make([][]float32, len(docVecs))
		for j, sv := range docVecs {
			docEmbeddings[j] = sv.Embedding
		}

		maxSimScore := ComputeMaxSim(queryEmbeddings, docEmbeddings)
		candidates[i].MaxSimScore = maxSimScore
		candidates[i].FinalScore = 0.60*maxSimScore + 0.40*candidates[i].HybridScore
	}

	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].FinalScore > candidates[j].FinalScore
	})

	return candidates
}

// ComputeMaxSim computes the MaxSim score between query vectors and document vectors.
func ComputeMaxSim(queryVecs, docVecs [][]float32) float64 {
	if len(queryVecs) == 0 || len(docVecs) == 0 {
		return 0
	}

	var totalScore float64
	for _, qVec := range queryVecs {
		maxSim := -1.0
		for _, dVec := range docVecs {
			sim := cosineSimilarity(qVec, dVec)
			if sim > maxSim {
				maxSim = sim
			}
		}
		if maxSim > -1.0 {
			totalScore += maxSim
		}
	}

	return totalScore / float64(len(queryVecs))
}

func cosineSimilarity(a, b []float32) float64 {
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
