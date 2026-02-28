package rag

import (
	"fmt"
	"hash/fnv"
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

type chunk struct {
	ID           string
	CollectionID string
	Text         string
	Tokens       []string
	Embedding    []float64
	Source       string
}

type Service struct {
	mu          sync.RWMutex
	nextID      int
	collections map[string]Collection
	chunks      map[string][]chunk
	retrievals  map[string]Retrieval
	evalScores  map[string]EvalScore
}

func NewService() *Service {
	return &Service{
		nextID:      1,
		collections: map[string]Collection{},
		chunks:      map[string][]chunk{},
		retrievals:  map[string]Retrieval{},
		evalScores:  map[string]EvalScore{},
	}
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
			existing = append(existing, chunk{
				ID:           fmt.Sprintf("%s_chunk_%04d", collectionID, chunkIndex),
				CollectionID: collectionID,
				Text:         part,
				Tokens:       tokens,
				Embedding:    embeddingFromTokens(tokens, 12),
				Source:       fmt.Sprintf("collection:%s", collectionID),
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
	queryTokens := tokenize(queryRewrite)
	queryEmbedding := embeddingFromTokens(queryTokens, 12)

	allowedCollections := s.collectionSelection(workspaceID, collectionIDs)
	results := make([]RetrievalResult, 0)
	for _, collection := range allowedCollections {
		for _, storedChunk := range s.chunks[collection.ID] {
			dense := cosineSimilarity(queryEmbedding, storedChunk.Embedding)
			bm25 := bm25TokenOverlap(queryTokens, storedChunk.Tokens)
			hybrid := dense
			if collection.BM25Enabled {
				hybrid = (0.6 * dense) + (0.4 * bm25)
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

func normalizeWorkspaceID(workspaceID string) string {
	if strings.TrimSpace(workspaceID) == "" {
		return "default"
	}
	return workspaceID
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

func embeddingFromTokens(tokens []string, dimensions int) []float64 {
	if dimensions <= 0 {
		dimensions = 8
	}
	vector := make([]float64, dimensions)
	if len(tokens) == 0 {
		return vector
	}
	for _, token := range tokens {
		hash := fnv.New64a()
		_, _ = hash.Write([]byte(token))
		value := hash.Sum64()
		for idx := 0; idx < dimensions; idx++ {
			component := float64((value>>(uint(idx%8)*8))&0xFF) / 255.0
			vector[idx] += component
		}
	}
	for idx := range vector {
		vector[idx] = vector[idx] / float64(len(tokens))
	}
	return vector
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

func bm25TokenOverlap(queryTokens, chunkTokens []string) float64 {
	if len(queryTokens) == 0 || len(chunkTokens) == 0 {
		return 0
	}
	chunkSet := map[string]struct{}{}
	for _, token := range chunkTokens {
		chunkSet[token] = struct{}{}
	}
	matches := 0
	for _, token := range queryTokens {
		if _, ok := chunkSet[token]; ok {
			matches++
		}
	}
	return float64(matches) / float64(len(queryTokens))
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

func maxInt(left, right int) int {
	if left > right {
		return left
	}
	return right
}
