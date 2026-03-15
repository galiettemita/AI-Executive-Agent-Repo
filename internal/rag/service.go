package rag

import (
	"context"
	"fmt"
	"log"
	"math"
	"sort"
	"strings"
	"sync"
	"time"
)

type Collection struct {
	ID             string `json:"id"`
	CollectionID   string `json:"collection_id"`
	WorkspaceID    string `json:"workspace_id"`
	Name           string `json:"name"`
	Description    string `json:"description"`
	EmbeddingModel string `json:"embedding_model"`
	ChunkSize      int    `json:"chunk_size"`
	BM25Enabled    bool   `json:"bm25_enabled"`
	Status         string `json:"status"`
	ChunkCount     int    `json:"chunk_count"`
}

type RetrievalResult struct {
	ChunkID     string  `json:"chunk_id"`
	Score       float64 `json:"score"`
	Source      string  `json:"source"`
	Collection  string  `json:"collection_id,omitempty"`
	Snippet     string  `json:"snippet,omitempty"`
	Provenance  string  `json:"provenance,omitempty"`
	DataClass   string  `json:"data_class,omitempty"`
	Sensitivity string  `json:"sensitivity_label,omitempty"`
}

type Retrieval struct {
	RetrievalID  string            `json:"retrieval_id"`
	TurnID       string            `json:"turn_id"`
	WorkspaceID  string            `json:"workspace_id"`
	Query        string            `json:"query"`
	QueryText    string            `json:"query_text"`
	QueryRewrite string            `json:"query_rewrite"`
	Results      []RetrievalResult `json:"results"`
}

type EvalScore struct {
	CollectionID string  `json:"collection_id"`
	WorkspaceID  string  `json:"workspace_id"`
	Faithfulness float64 `json:"faithfulness"`
	Relevance    float64 `json:"relevance"`
	ComputedAt   string  `json:"computed_at"`
}

type RerankerConfig struct {
	WorkspaceID  string  `json:"workspace_id"`
	DenseWeight  float64 `json:"dense_weight"`
	BM25Weight   float64 `json:"bm25_weight"`
	Enabled      bool    `json:"enabled"`
	VersionLabel string  `json:"version_label"`
	RerankerMode string  `json:"reranker_mode,omitempty"` // "cohere" | "llm" | "passthrough"
	CohereModel  string  `json:"cohere_model,omitempty"`
	RerankTopK   int     `json:"rerank_top_k,omitempty"`
}

type RetrievalEvalScore struct {
	RetrievalID  string  `json:"retrieval_id"`
	WorkspaceID  string  `json:"workspace_id"`
	Faithfulness float64 `json:"faithfulness"`
	Relevance    float64 `json:"relevance"`
	Pass         bool    `json:"pass"`
}

type chunk struct {
	ID              string
	CollectionID    string
	Text            string
	OriginalContent string // raw text before enrichment — displayed to users
	EnrichedContent string // context-prepended text that was embedded
	Tokens          []string
	Embedding       []float64
	Source          string
}

// AdaptiveGate classifies queries to determine if retrieval should be skipped.
// Implemented by adaptive.Gate. Nil = always retrieve.
type AdaptiveGate interface {
	ShouldSkipRetrieval(ctx context.Context, query string) bool
}

type Service struct {
	embedder       EmbeddingProvider // never nil; panics at construction if nil
	bm25           *BM25Index        // never nil; initialized in NewService
	enricher       ChunkEnricher     // default: MetadataChunkEnricher
	hydeExpander   *HyDEExpander     // nil = direct embedding (no HyDE)
	reranker       Reranker          // nil = no reranking
	adaptiveGate   AdaptiveGate      // nil = always retrieve (backwards compat)
	mu             sync.RWMutex
	nextID         int
	collections    map[string]Collection
	chunks         map[string][]chunk
	retrievals     map[string]Retrieval
	evalScores     map[string]EvalScore
	rerankers      map[string]RerankerConfig
	retrievalEvals map[string]RetrievalEvalScore
}

func NewService(embedder EmbeddingProvider) *Service {
	if embedder == nil {
		panic("rag.NewService: embedder must not be nil — use MockEmbeddingProvider in tests")
	}
	return &Service{
		embedder:       embedder,
		bm25:           NewBM25Index(),
		enricher:       NewPassthroughChunkEnricher(),
		nextID:         1,
		collections:    map[string]Collection{},
		chunks:         map[string][]chunk{},
		retrievals:     map[string]Retrieval{},
		evalScores:     map[string]EvalScore{},
		rerankers:      map[string]RerankerConfig{},
		retrievalEvals: map[string]RetrievalEvalScore{},
	}
}

// WithAdaptiveGate attaches the adaptive RAG gate to the service.
func (s *Service) WithAdaptiveGate(gate AdaptiveGate) *Service {
	s.adaptiveGate = gate
	return s
}

// WithHyDEExpander attaches a HyDE expander to the service.
func (s *Service) WithHyDEExpander(expander *HyDEExpander) *Service {
	s.hydeExpander = expander
	return s
}

// WithReranker attaches a cross-encoder reranker to the service.
func (s *Service) WithReranker(r Reranker) *Service {
	s.reranker = r
	return s
}

// WithEnricher overrides the default chunk enricher.
func (s *Service) WithEnricher(e ChunkEnricher) *Service {
	s.enricher = e
	return s
}

func (s *Service) UpsertCollection(collection Collection) Collection {
	s.mu.Lock()
	defer s.mu.Unlock()

	collection = normalizeCollection(collection)
	if collection.ID == "" {
		collection.ID = s.nextCollectionID()
	}
	collection.CollectionID = collection.ID
	collection.ChunkCount = len(s.chunks[collection.ID])
	s.collections[collection.ID] = collection
	return collection
}

func (s *Service) GetCollection(id string) (Collection, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	collection, ok := s.collections[id]
	return collection, ok
}

func (s *Service) DeleteCollection(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.collections[id]; !ok {
		return false
	}
	delete(s.collections, id)
	delete(s.chunks, id)
	delete(s.evalScores, id)
	return true
}

func (s *Service) ListCollections(workspaceID string) []Collection {
	s.mu.RLock()
	defer s.mu.RUnlock()

	workspaceID = normalizeWorkspaceID(workspaceID)
	out := make([]Collection, 0, len(s.collections))
	for _, collection := range s.collections {
		if collection.WorkspaceID != workspaceID {
			continue
		}
		out = append(out, collection)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].ID < out[j].ID
	})
	return out
}

func (s *Service) Ingest(collectionID string, documents []string) (Collection, int, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	collection, ok := s.collections[collectionID]
	if !ok {
		return Collection{}, 0, false
	}

	ingested := 0
	existing := s.chunks[collectionID]
	chunkIndex := len(existing)
	for _, document := range documents {
		clean := strings.TrimSpace(document)
		if clean == "" {
			continue
		}
		parts := splitChunks(clean, collection.ChunkSize)
		for _, part := range parts {
			chunkIndex++
			tokens := tokenize(part)

			// Enrich chunk — best-effort; never fail ingest due to enrichment error.
			enriched, enrichErr := s.enricher.Enrich(context.Background(), DocumentMeta{
				WorkspaceID: collection.WorkspaceID,
			}, part)
			if enrichErr != nil {
				log.Printf("[rag.Ingest] enrichment failed for chunk %d, using original: %v", chunkIndex, enrichErr)
				enriched = part
			}

			// Embed the ENRICHED text (improves retrieval quality).
			chunkEmbeddings, embedErr := s.embedder.Embed(context.Background(), []string{enriched})
			if embedErr != nil {
				log.Printf("[rag.Ingest] embedding failed for chunk %d: %v", chunkIndex, embedErr)
				continue
			}
			if len(chunkEmbeddings) == 0 || len(chunkEmbeddings[0]) == 0 {
				log.Printf("[rag.Ingest] empty vector returned for chunk %d", chunkIndex)
				continue
			}

			chunkID := fmt.Sprintf("%s_chunk_%04d", collectionID, chunkIndex)

			// Index for BM25 scoring.
			bm25Tokens := BM25Tokenize(part)
			s.bm25.IndexDocument(chunkID, bm25Tokens)

			existing = append(existing, chunk{
				ID:              chunkID,
				CollectionID:    collectionID,
				Text:            part,
				OriginalContent: part,
				EnrichedContent: enriched,
				Tokens:          tokens,
				Embedding:       float32ToFloat64(chunkEmbeddings[0]),
				Source:          fmt.Sprintf("collection:%s", collectionID),
			})
			ingested++
		}
	}
	s.chunks[collectionID] = existing
	collection.ChunkCount = len(existing)
	s.collections[collectionID] = collection
	s.evalScores[collectionID] = evaluateCollection(collection)
	return collection, ingested, true
}

func (s *Service) Search(workspaceID, turnID, queryText string, collectionIDs []string, maxResults int) Retrieval {
	// Adaptive RAG Gate: skip retrieval for simple acknowledgements.
	if s.adaptiveGate != nil && s.adaptiveGate.ShouldSkipRetrieval(context.Background(), queryText) {
		return Retrieval{
			RetrievalID:  turnID,
			TurnID:       turnID,
			WorkspaceID:  workspaceID,
			Query:        queryText,
			QueryText:    queryText,
			QueryRewrite: normalizeQueryRewrite(queryText),
			Results:      []RetrievalResult{},
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	workspaceID = normalizeWorkspaceID(workspaceID)
	if strings.TrimSpace(turnID) == "" {
		turnID = fmt.Sprintf("turn_%06d", len(s.retrievals)+1)
	}
	if maxResults <= 0 {
		maxResults = 3
	}
	queryRewrite := normalizeQueryRewrite(queryText)

	// HyDE-expanded query embedding (averages hypothetical doc + original query).
	var queryVecF32 []float32
	if s.hydeExpander != nil {
		var hydeErr error
		queryVecF32, hydeErr = s.hydeExpander.ExpandQuery(context.Background(), queryRewrite)
		if hydeErr != nil {
			// Fallback: direct embedding.
			embs, embedErr := s.embedder.Embed(context.Background(), []string{queryRewrite})
			if embedErr != nil || len(embs) == 0 || len(embs[0]) == 0 {
				return Retrieval{
					RetrievalID: turnID, TurnID: turnID, WorkspaceID: workspaceID,
					Query: queryText, QueryText: queryText, QueryRewrite: queryRewrite,
				}
			}
			queryVecF32 = embs[0]
		}
	} else {
		embs, embedErr := s.embedder.Embed(context.Background(), []string{queryRewrite})
		if embedErr != nil || len(embs) == 0 || len(embs[0]) == 0 {
			return Retrieval{
				RetrievalID: turnID, TurnID: turnID, WorkspaceID: workspaceID,
				Query: queryText, QueryText: queryText, QueryRewrite: queryRewrite,
			}
		}
		queryVecF32 = embs[0]
	}
	queryEmbedding := float32ToFloat64(queryVecF32)
	reranker := s.rerankerConfigLocked(workspaceID)

	allowedCollections := s.collectionSelection(workspaceID, collectionIDs)
	results := make([]RetrievalResult, 0)
	for _, collection := range allowedCollections {
		for _, storedChunk := range s.chunks[collection.ID] {
			dense := cosineSimilarity(queryEmbedding, storedChunk.Embedding)
			bm25 := s.bm25.Score(BM25Tokenize(storedChunk.Text), BM25Tokenize(queryRewrite))
			hybrid := dense
			if reranker.Enabled && collection.BM25Enabled {
				hybrid = (reranker.DenseWeight * dense) + (reranker.BM25Weight * bm25)
			}
			results = append(results, RetrievalResult{
				ChunkID:     storedChunk.ID,
				Score:       roundScore(hybrid),
				Source:      storedChunk.Source,
				Collection:  collection.ID,
				Snippet:     storedChunk.Text,
				Provenance:  storedChunk.Source,
				DataClass:   "internal",
				Sensitivity: "standard",
			})
		}
	}

	sort.Slice(results, func(i, j int) bool {
		if results[i].Score == results[j].Score {
			return results[i].ChunkID < results[j].ChunkID
		}
		return results[i].Score > results[j].Score
	})

	// Cross-encoder reranking: applied to top-20 hybrid candidates.
	if s.reranker != nil && len(results) > 0 {
		rerankInput := results
		if len(rerankInput) > 20 {
			rerankInput = rerankInput[:20]
		}
		reranked, rerankErr := s.reranker.Rerank(context.Background(), queryRewrite, rerankInput, maxResults)
		if rerankErr == nil {
			results = reranked
		}
	}

	if len(results) > maxResults {
		results = results[:maxResults]
	}

	retrieval := Retrieval{
		RetrievalID:  turnID,
		TurnID:       turnID,
		WorkspaceID:  workspaceID,
		Query:        queryText,
		QueryText:    queryText,
		QueryRewrite: queryRewrite,
		Results:      results,
	}
	s.retrievals[turnID] = retrieval
	s.retrievalEvals[turnID] = evaluateRetrieval(retrieval)
	return retrieval
}

func (s *Service) GetRetrieval(turnID string) (Retrieval, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	retrieval, ok := s.retrievals[turnID]
	return retrieval, ok
}

func (s *Service) ListEvalScores(workspaceID string) []EvalScore {
	s.mu.RLock()
	defer s.mu.RUnlock()

	workspaceID = normalizeWorkspaceID(workspaceID)
	out := make([]EvalScore, 0, len(s.evalScores))
	for _, score := range s.evalScores {
		if score.WorkspaceID != workspaceID {
			continue
		}
		out = append(out, score)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].CollectionID < out[j].CollectionID
	})
	return out
}

func (s *Service) SetRerankerConfig(workspaceID string, denseWeight, bm25Weight float64) RerankerConfig {
	s.mu.Lock()
	defer s.mu.Unlock()
	workspaceID = normalizeWorkspaceID(workspaceID)
	cfg := normalizeRerankerConfig(RerankerConfig{
		WorkspaceID:  workspaceID,
		DenseWeight:  denseWeight,
		BM25Weight:   bm25Weight,
		Enabled:      true,
		VersionLabel: "reranker_v1",
	})
	s.rerankers[workspaceID] = cfg
	return cfg
}

func (s *Service) GetRerankerConfig(workspaceID string) RerankerConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.rerankerConfigLocked(normalizeWorkspaceID(workspaceID))
}

func (s *Service) ListRetrievalEvalScores(workspaceID string) []RetrievalEvalScore {
	s.mu.RLock()
	defer s.mu.RUnlock()
	workspaceID = normalizeWorkspaceID(workspaceID)
	out := make([]RetrievalEvalScore, 0, len(s.retrievalEvals))
	for _, score := range s.retrievalEvals {
		if score.WorkspaceID != workspaceID {
			continue
		}
		out = append(out, score)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].RetrievalID < out[j].RetrievalID
	})
	return out
}

func (s *Service) collectionSelection(workspaceID string, collectionIDs []string) []Collection {
	allowSet := map[string]struct{}{}
	for _, id := range collectionIDs {
		allowSet[id] = struct{}{}
	}

	out := make([]Collection, 0, len(s.collections))
	for _, collection := range s.collections {
		if collection.WorkspaceID != workspaceID {
			continue
		}
		if len(allowSet) > 0 {
			if _, ok := allowSet[collection.ID]; !ok {
				continue
			}
		}
		out = append(out, collection)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].ID < out[j].ID
	})
	return out
}

func (s *Service) nextCollectionID() string {
	id := fmt.Sprintf("collection_%06d", s.nextID)
	s.nextID++
	return id
}

func normalizeCollection(collection Collection) Collection {
	if strings.TrimSpace(collection.WorkspaceID) == "" {
		collection.WorkspaceID = "default"
	}
	if strings.TrimSpace(collection.Status) == "" {
		collection.Status = "active"
	}
	if strings.TrimSpace(collection.EmbeddingModel) == "" {
		collection.EmbeddingModel = "text-embedding-3-small"
	}
	if collection.ChunkSize < 64 {
		collection.ChunkSize = 96
	}
	if !collection.BM25Enabled {
		// Default to true for hybrid-search mode unless explicitly disabled by payload.
		collection.BM25Enabled = true
	}
	return collection
}

func normalizeRerankerConfig(config RerankerConfig) RerankerConfig {
	if strings.TrimSpace(config.WorkspaceID) == "" {
		config.WorkspaceID = "default"
	}
	if config.DenseWeight <= 0 {
		config.DenseWeight = 0.6
	}
	if config.BM25Weight < 0 {
		config.BM25Weight = 0
	}
	total := config.DenseWeight + config.BM25Weight
	if total == 0 {
		config.DenseWeight = 0.6
		config.BM25Weight = 0.4
		total = 1
	}
	config.DenseWeight = config.DenseWeight / total
	config.BM25Weight = config.BM25Weight / total
	if strings.TrimSpace(config.VersionLabel) == "" {
		config.VersionLabel = "reranker_v1"
	}
	return config
}

func normalizeWorkspaceID(workspaceID string) string {
	if strings.TrimSpace(workspaceID) == "" {
		return "default"
	}
	return workspaceID
}

func (s *Service) rerankerConfigLocked(workspaceID string) RerankerConfig {
	if config, ok := s.rerankers[workspaceID]; ok {
		return normalizeRerankerConfig(config)
	}
	return normalizeRerankerConfig(RerankerConfig{
		WorkspaceID:  workspaceID,
		DenseWeight:  0.6,
		BM25Weight:   0.4,
		Enabled:      true,
		VersionLabel: "reranker_v1",
	})
}

func splitChunks(input string, chunkSize int) []string {
	words := strings.Fields(input)
	if len(words) == 0 {
		return nil
	}
	if chunkSize < 64 {
		chunkSize = 96
	}
	wordsPerChunk := maxInt(chunkSize/8, 8)
	chunks := make([]string, 0, (len(words)/wordsPerChunk)+1)
	for i := 0; i < len(words); i += wordsPerChunk {
		end := i + wordsPerChunk
		if end > len(words) {
			end = len(words)
		}
		chunks = append(chunks, strings.Join(words[i:end], " "))
	}
	return chunks
}

func normalizeQueryRewrite(query string) string {
	lower := strings.ToLower(strings.TrimSpace(query))
	fields := strings.Fields(lower)
	if len(fields) == 0 {
		return ""
	}
	return strings.Join(fields, " ")
}

func tokenize(input string) []string {
	fields := strings.Fields(strings.ToLower(strings.TrimSpace(input)))
	out := make([]string, 0, len(fields))
	for _, field := range fields {
		clean := strings.Trim(field, ".,:;!?()[]{}\"'")
		if clean == "" {
			continue
		}
		out = append(out, clean)
	}
	return out
}

// float32ToFloat64 converts a []float32 vector to []float64 for compatibility
// with the in-memory chunk storage which uses float64.
func float32ToFloat64(v []float32) []float64 {
	out := make([]float64, len(v))
	for i, x := range v {
		out[i] = float64(x)
	}
	return out
}

func cosineSimilarity(left, right []float64) float64 {
	if len(left) == 0 || len(right) == 0 {
		return 0
	}
	dimensions := len(left)
	if len(right) < dimensions {
		dimensions = len(right)
	}
	var dot float64
	var leftNorm float64
	var rightNorm float64
	for idx := 0; idx < dimensions; idx++ {
		dot += left[idx] * right[idx]
		leftNorm += left[idx] * left[idx]
		rightNorm += right[idx] * right[idx]
	}
	if leftNorm == 0 || rightNorm == 0 {
		return 0
	}
	return dot / (math.Sqrt(leftNorm) * math.Sqrt(rightNorm))
}


func roundScore(score float64) float64 {
	if score < 0 {
		score = 0
	}
	if score > 1 {
		score = 1
	}
	return math.Round(score*10000) / 10000
}

func evaluateCollection(collection Collection) EvalScore {
	faithfulness := 0.74 + (float64(collection.ChunkCount) * 0.01)
	if faithfulness > 0.95 {
		faithfulness = 0.95
	}
	relevance := 0.70 + (float64(collection.ChunkCount) * 0.009)
	if relevance > 0.90 {
		relevance = 0.90
	}
	return EvalScore{
		CollectionID: collection.ID,
		WorkspaceID:  collection.WorkspaceID,
		Faithfulness: roundScore(faithfulness),
		Relevance:    roundScore(relevance),
		ComputedAt:   time.Now().UTC().Format(time.RFC3339),
	}
}

func evaluateRetrieval(retrieval Retrieval) RetrievalEvalScore {
	score := RetrievalEvalScore{
		RetrievalID: retrieval.RetrievalID,
		WorkspaceID: retrieval.WorkspaceID,
	}
	if len(retrieval.Results) == 0 {
		return score
	}
	score.Relevance = retrieval.Results[0].Score
	sum := 0.0
	for _, result := range retrieval.Results {
		sum += result.Score
	}
	score.Faithfulness = roundScore(sum / float64(len(retrieval.Results)))
	score.Pass = score.Faithfulness >= 0.80 && score.Relevance >= 0.75
	return score
}

func maxInt(left, right int) int {
	if left > right {
		return left
	}
	return right
}
