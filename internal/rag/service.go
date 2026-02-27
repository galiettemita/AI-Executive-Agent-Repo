package rag

import (
	"fmt"
	"sort"
	"strings"
	"sync"
)

type Collection struct {
	ID          string `json:"id"`
	WorkspaceID string `json:"workspace_id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Status      string `json:"status"`
	ChunkCount  int    `json:"chunk_count"`
}

type RetrievalResult struct {
	ChunkID     string  `json:"chunk_id"`
	Collection  string  `json:"collection_id"`
	Snippet     string  `json:"snippet"`
	Score       float64 `json:"score"`
	Provenance  string  `json:"provenance"`
	DataClass   string  `json:"data_class"`
	Sensitivity string  `json:"sensitivity_label"`
}

type Retrieval struct {
	TurnID      string            `json:"turn_id"`
	WorkspaceID string            `json:"workspace_id"`
	QueryText   string            `json:"query_text"`
	Results     []RetrievalResult `json:"results"`
}

type EvalScore struct {
	CollectionID string  `json:"collection_id"`
	WorkspaceID  string  `json:"workspace_id"`
	Faithfulness float64 `json:"faithfulness"`
	Relevance    float64 `json:"relevance"`
	ComputedAt   string  `json:"computed_at"`
}

type Service struct {
	mu          sync.RWMutex
	nextID      int
	collections map[string]Collection
	chunks      map[string][]string
	retrievals  map[string]Retrieval
	evalScores  map[string]EvalScore
}

func NewService() *Service {
	return &Service{
		nextID:      1,
		collections: map[string]Collection{},
		chunks:      map[string][]string{},
		retrievals:  map[string]Retrieval{},
		evalScores:  map[string]EvalScore{},
	}
}

func (s *Service) UpsertCollection(collection Collection) Collection {
	s.mu.Lock()
	defer s.mu.Unlock()

	if collection.ID == "" {
		collection.ID = s.nextCollectionID()
	}
	if collection.WorkspaceID == "" {
		collection.WorkspaceID = "default"
	}
	if collection.Status == "" {
		collection.Status = "active"
	}
	existingChunks := s.chunks[collection.ID]
	collection.ChunkCount = len(existingChunks)

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

	out := make([]Collection, 0, len(s.collections))
	for _, collection := range s.collections {
		if workspaceID != "" && collection.WorkspaceID != workspaceID {
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
	for _, document := range documents {
		clean := strings.TrimSpace(document)
		if clean == "" {
			continue
		}
		parts := splitChunks(clean)
		s.chunks[collectionID] = append(s.chunks[collectionID], parts...)
		ingested += len(parts)
	}
	collection.ChunkCount = len(s.chunks[collectionID])
	s.collections[collectionID] = collection
	s.evalScores[collectionID] = evaluateCollection(collection)
	return collection, ingested, true
}

func (s *Service) Search(workspaceID, turnID, queryText string, collectionIDs []string, maxResults int) Retrieval {
	s.mu.Lock()
	defer s.mu.Unlock()

	if workspaceID == "" {
		workspaceID = "default"
	}
	if turnID == "" {
		turnID = fmt.Sprintf("turn_%06d", len(s.retrievals)+1)
	}
	if maxResults <= 0 {
		maxResults = 3
	}

	allowedCollections := s.collectionSelection(workspaceID, collectionIDs)
	results := make([]RetrievalResult, 0)
	queryLower := strings.ToLower(queryText)
	for _, collection := range allowedCollections {
		chunks := s.chunks[collection.ID]
		for idx, chunk := range chunks {
			score := overlapScore(queryLower, strings.ToLower(chunk))
			results = append(results, RetrievalResult{
				ChunkID:     fmt.Sprintf("%s_chunk_%04d", collection.ID, idx+1),
				Collection:  collection.ID,
				Snippet:     chunk,
				Score:       score,
				Provenance:  fmt.Sprintf("collection:%s", collection.ID),
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
		TurnID:      turnID,
		WorkspaceID: workspaceID,
		QueryText:   queryText,
		Results:     results,
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

	out := make([]EvalScore, 0, len(s.evalScores))
	for _, score := range s.evalScores {
		if workspaceID != "" && score.WorkspaceID != workspaceID {
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

func splitChunks(input string) []string {
	words := strings.Fields(input)
	if len(words) == 0 {
		return nil
	}
	const chunkSize = 24
	chunks := make([]string, 0, (len(words)/chunkSize)+1)
	for i := 0; i < len(words); i += chunkSize {
		end := i + chunkSize
		if end > len(words) {
			end = len(words)
		}
		chunks = append(chunks, strings.Join(words[i:end], " "))
	}
	return chunks
}

func overlapScore(query, content string) float64 {
	if query == "" || content == "" {
		return 0
	}
	score := 0.1
	queryParts := strings.Fields(query)
	for _, part := range queryParts {
		if strings.Contains(content, part) {
			score += 0.2
		}
	}
	if score > 1 {
		return 1
	}
	return score
}

func evaluateCollection(collection Collection) EvalScore {
	faithfulness := 0.75 + (float64(collection.ChunkCount) * 0.01)
	if faithfulness > 0.95 {
		faithfulness = 0.95
	}
	relevance := 0.70 + (float64(collection.ChunkCount) * 0.008)
	if relevance > 0.90 {
		relevance = 0.90
	}
	return EvalScore{
		CollectionID: collection.ID,
		WorkspaceID:  collection.WorkspaceID,
		Faithfulness: faithfulness,
		Relevance:    relevance,
		ComputedAt:   fmt.Sprintf("2026-01-01T00:%02d:00Z", collection.ChunkCount%60),
	}
}
