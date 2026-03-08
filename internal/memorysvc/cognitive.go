package memorysvc

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
	"time"
)

func generateID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// ---------------------------------------------------------------------------
// Cognitive memory
// ---------------------------------------------------------------------------

// CognitiveMemory represents a single memory unit.
type CognitiveMemory struct {
	ID             string
	WorkspaceID    string
	Type           string // episodic, semantic, procedural
	Content        string
	Importance     float64
	Tags           []string
	CreatedAt      time.Time
	AccessCount    int
	LastAccessedAt time.Time
	Embedding      []float32
}

// ConsolidationResult reports the outcome of memory consolidation.
type ConsolidationResult struct {
	MergedCount   int
	NewSemanticID string
	Summary       string
}

// CognitiveMemoryService stores, recalls, and consolidates memories.
type CognitiveMemoryService struct {
	mu       sync.RWMutex
	memories map[string]*CognitiveMemory
	// byWorkspace indexes memory IDs by workspace.
	byWorkspace map[string][]string
}

// NewCognitiveMemoryService creates a CognitiveMemoryService.
func NewCognitiveMemoryService() *CognitiveMemoryService {
	return &CognitiveMemoryService{
		memories:    make(map[string]*CognitiveMemory),
		byWorkspace: make(map[string][]string),
	}
}

var validMemoryTypes = map[string]bool{
	"episodic": true, "semantic": true, "procedural": true,
}

// Store persists a new memory.
func (c *CognitiveMemoryService) Store(workspaceID string, mem CognitiveMemory) (*CognitiveMemory, error) {
	if workspaceID == "" {
		return nil, errors.New("workspaceID is required")
	}
	if mem.Content == "" {
		return nil, errors.New("content is required")
	}
	if !validMemoryTypes[mem.Type] {
		return nil, fmt.Errorf("invalid memory type %q", mem.Type)
	}

	mem.ID = generateID()
	mem.WorkspaceID = workspaceID
	mem.CreatedAt = time.Now().UTC()
	mem.AccessCount = 0
	if mem.Importance < 0 || mem.Importance > 1 {
		mem.Importance = 0.5
	}

	// Generate a simple pseudo-embedding from content words.
	mem.Embedding = simpleEmbed(mem.Content)

	c.mu.Lock()
	c.memories[mem.ID] = &mem
	c.byWorkspace[workspaceID] = append(c.byWorkspace[workspaceID], mem.ID)
	c.mu.Unlock()
	return &mem, nil
}

// Recall retrieves the most relevant memories for a query, ranked by keyword
// overlap and importance.
func (c *CognitiveMemoryService) Recall(workspaceID, query string, limit int) []CognitiveMemory {
	if limit <= 0 {
		limit = 10
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	ids := c.byWorkspace[workspaceID]
	queryWords := strings.Fields(strings.ToLower(query))

	type scored struct {
		mem   *CognitiveMemory
		score float64
	}
	var results []scored

	for _, id := range ids {
		m := c.memories[id]
		contentLower := strings.ToLower(m.Content)

		// Keyword match score.
		matchCount := 0
		for _, w := range queryWords {
			if strings.Contains(contentLower, w) {
				matchCount++
			}
		}
		if matchCount == 0 {
			continue
		}
		keywordScore := float64(matchCount) / float64(len(queryWords))
		totalScore := keywordScore*0.6 + m.Importance*0.4

		results = append(results, scored{m, totalScore})
	}

	sort.Slice(results, func(i, j int) bool { return results[i].score > results[j].score })

	if len(results) > limit {
		results = results[:limit]
	}

	out := make([]CognitiveMemory, len(results))
	now := time.Now().UTC()
	for i, r := range results {
		r.mem.AccessCount++
		r.mem.LastAccessedAt = now
		out[i] = *r.mem
	}
	return out
}

// Forget deletes a memory by ID.
func (c *CognitiveMemoryService) Forget(memoryID string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	m, ok := c.memories[memoryID]
	if !ok {
		return fmt.Errorf("memory %s not found", memoryID)
	}

	// Remove from workspace index.
	wsIDs := c.byWorkspace[m.WorkspaceID]
	for i, id := range wsIDs {
		if id == memoryID {
			c.byWorkspace[m.WorkspaceID] = append(wsIDs[:i], wsIDs[i+1:]...)
			break
		}
	}
	delete(c.memories, memoryID)
	return nil
}

// Consolidate merges related episodic memories into a single semantic memory.
func (c *CognitiveMemoryService) Consolidate(workspaceID string) (*ConsolidationResult, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	ids := c.byWorkspace[workspaceID]
	var episodic []*CognitiveMemory
	for _, id := range ids {
		m := c.memories[id]
		if m.Type == "episodic" {
			episodic = append(episodic, m)
		}
	}
	if len(episodic) < 2 {
		return nil, errors.New("need at least 2 episodic memories to consolidate")
	}

	// Merge content into a summary.
	var parts []string
	var allTags []string
	var maxImportance float64
	for _, m := range episodic {
		parts = append(parts, m.Content)
		allTags = append(allTags, m.Tags...)
		if m.Importance > maxImportance {
			maxImportance = m.Importance
		}
	}
	summary := "Consolidated: " + strings.Join(parts, " | ")

	// Create semantic memory.
	sem := &CognitiveMemory{
		ID:          generateID(),
		WorkspaceID: workspaceID,
		Type:        "semantic",
		Content:     summary,
		Importance:  maxImportance,
		Tags:        dedupStrings(allTags),
		CreatedAt:   time.Now().UTC(),
		Embedding:   simpleEmbed(summary),
	}
	c.memories[sem.ID] = sem
	c.byWorkspace[workspaceID] = append(c.byWorkspace[workspaceID], sem.ID)

	// Remove original episodic memories.
	for _, m := range episodic {
		delete(c.memories, m.ID)
	}
	// Rebuild workspace index.
	var remaining []string
	for _, id := range c.byWorkspace[workspaceID] {
		if _, ok := c.memories[id]; ok {
			remaining = append(remaining, id)
		}
	}
	c.byWorkspace[workspaceID] = remaining

	return &ConsolidationResult{
		MergedCount:   len(episodic),
		NewSemanticID: sem.ID,
		Summary:       summary,
	}, nil
}

// ---------------------------------------------------------------------------
// Vector index (simplified)
// ---------------------------------------------------------------------------

// Document represents an indexable document.
type Document struct {
	ID       string
	Content  string
	Metadata map[string]any
}

// SearchResult represents a vector search hit.
type SearchResult struct {
	DocumentID string
	Content    string
	Score      float64
}

// VectorIndexService provides document indexing and similarity search using
// a simplified cosine-similarity approach over term-frequency vectors.
type VectorIndexService struct {
	mu   sync.RWMutex
	docs map[string]*indexedDoc
	// byWorkspace indexes doc IDs per workspace.
	byWorkspace map[string][]string
}

type indexedDoc struct {
	doc       Document
	embedding []float32
}

// NewVectorIndexService creates a VectorIndexService.
func NewVectorIndexService() *VectorIndexService {
	return &VectorIndexService{
		docs:        make(map[string]*indexedDoc),
		byWorkspace: make(map[string][]string),
	}
}

// IndexDocument indexes a document under the given workspace.
func (v *VectorIndexService) IndexDocument(workspaceID string, doc Document) error {
	if workspaceID == "" {
		return errors.New("workspaceID is required")
	}
	if doc.Content == "" {
		return errors.New("document content is required")
	}
	if doc.ID == "" {
		doc.ID = generateID()
	}

	v.mu.Lock()
	v.docs[doc.ID] = &indexedDoc{doc: doc, embedding: simpleEmbed(doc.Content)}
	v.byWorkspace[workspaceID] = append(v.byWorkspace[workspaceID], doc.ID)
	v.mu.Unlock()
	return nil
}

// SearchSimilar returns the top-limit documents most similar to query.
func (v *VectorIndexService) SearchSimilar(workspaceID, query string, limit int) []SearchResult {
	if limit <= 0 {
		limit = 10
	}
	queryEmb := simpleEmbed(query)

	v.mu.RLock()
	defer v.mu.RUnlock()

	ids := v.byWorkspace[workspaceID]
	type scored struct {
		id      string
		content string
		score   float64
	}
	var results []scored
	for _, id := range ids {
		d := v.docs[id]
		sim := cosineSimilarity(queryEmb, d.embedding)
		if sim > 0 {
			results = append(results, scored{id, d.doc.Content, sim})
		}
	}

	sort.Slice(results, func(i, j int) bool { return results[i].score > results[j].score })
	if len(results) > limit {
		results = results[:limit]
	}

	out := make([]SearchResult, len(results))
	for i, r := range results {
		out[i] = SearchResult{DocumentID: r.id, Content: r.content, Score: r.score}
	}
	return out
}

// ---------------------------------------------------------------------------
// Context compaction
// ---------------------------------------------------------------------------

// Message represents a conversation message.
type Message struct {
	Role    string
	Content string
}

// CompactedContext is the result of compacting a conversation.
type CompactedContext struct {
	Summary          string
	KeyFacts         []string
	TokenCount       int
	OriginalCount    int
	CompressionRatio float64
}

// ContextCompactionService compresses long conversations.
type ContextCompactionService struct{}

// NewContextCompactionService creates a ContextCompactionService.
func NewContextCompactionService() *ContextCompactionService {
	return &ContextCompactionService{}
}

// ShouldCompact returns true when the conversation exceeds 40 turns.
func (c *ContextCompactionService) ShouldCompact(messageCount int) bool {
	return messageCount > 40
}

// Compact summarizes messages down to maxTokens (approximate).
func (c *ContextCompactionService) Compact(workspaceID string, messages []Message, maxTokens int) (*CompactedContext, error) {
	if workspaceID == "" {
		return nil, errors.New("workspaceID is required")
	}
	if len(messages) == 0 {
		return nil, errors.New("no messages to compact")
	}
	if maxTokens <= 0 {
		maxTokens = 2000
	}

	// Approximate token count (words / 0.75).
	var totalWords int
	var allContent []string
	for _, m := range messages {
		words := strings.Fields(m.Content)
		totalWords += len(words)
		allContent = append(allContent, m.Content)
	}
	originalTokens := int(float64(totalWords) / 0.75)

	// Extract key facts: take the first sentence from each message (up to 10).
	var keyFacts []string
	for _, m := range messages {
		if len(keyFacts) >= 10 {
			break
		}
		sentence := firstSentence(m.Content)
		if sentence != "" {
			keyFacts = append(keyFacts, sentence)
		}
	}

	// Build summary: truncate combined content to fit token budget.
	combined := strings.Join(allContent, " ")
	maxChars := maxTokens * 4 // rough token-to-char ratio
	if len(combined) > maxChars {
		combined = combined[:maxChars]
	}

	summaryTokens := len(strings.Fields(combined))
	ratio := 0.0
	if originalTokens > 0 {
		ratio = float64(summaryTokens) / float64(originalTokens)
	}

	return &CompactedContext{
		Summary:          combined,
		KeyFacts:         keyFacts,
		TokenCount:       summaryTokens,
		OriginalCount:    originalTokens,
		CompressionRatio: ratio,
	}, nil
}

// ---------------------------------------------------------------------------
// Preference learning
// ---------------------------------------------------------------------------

// PreferenceLearningService learns and infers user preferences.
type PreferenceLearningService struct {
	mu sync.RWMutex
	// prefs[workspaceID][userID][category] = []observation
	prefs map[string]map[string]map[string][]observation
}

type observation struct {
	Value      string
	Confidence float64
	Time       time.Time
}

// NewPreferenceLearningService creates a PreferenceLearningService.
func NewPreferenceLearningService() *PreferenceLearningService {
	return &PreferenceLearningService{
		prefs: make(map[string]map[string]map[string][]observation),
	}
}

// LearnPreference records a preference observation.
func (p *PreferenceLearningService) LearnPreference(workspaceID, userID, category, value string, confidence float64) {
	if confidence <= 0 || confidence > 1 {
		confidence = 0.5
	}
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.prefs[workspaceID] == nil {
		p.prefs[workspaceID] = make(map[string]map[string][]observation)
	}
	if p.prefs[workspaceID][userID] == nil {
		p.prefs[workspaceID][userID] = make(map[string][]observation)
	}
	p.prefs[workspaceID][userID][category] = append(
		p.prefs[workspaceID][userID][category],
		observation{Value: value, Confidence: confidence, Time: time.Now().UTC()},
	)
}

// GetPreferences returns the best-known preference per category.
func (p *PreferenceLearningService) GetPreferences(workspaceID, userID string) map[string]string {
	p.mu.RLock()
	defer p.mu.RUnlock()

	result := make(map[string]string)
	userPrefs := p.prefs[workspaceID][userID]
	for cat, obs := range userPrefs {
		best := ""
		bestConf := 0.0
		for _, o := range obs {
			if o.Confidence > bestConf {
				best = o.Value
				bestConf = o.Confidence
			}
		}
		result[cat] = best
	}
	return result
}

// InferPreference returns the inferred value and confidence for a category.
func (p *PreferenceLearningService) InferPreference(workspaceID, userID, category string) (string, float64) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	obs := p.prefs[workspaceID][userID][category]
	if len(obs) == 0 {
		return "", 0
	}

	// Weighted vote: more recent + higher confidence observations win.
	valueCounts := make(map[string]float64)
	for _, o := range obs {
		// Weight by confidence and recency (decay over 30 days).
		age := time.Since(o.Time).Hours() / (24 * 30)
		recency := math.Exp(-age)
		valueCounts[o.Value] += o.Confidence * recency
	}

	best := ""
	bestScore := 0.0
	totalScore := 0.0
	for v, s := range valueCounts {
		totalScore += s
		if s > bestScore {
			best = v
			bestScore = s
		}
	}

	conf := 0.0
	if totalScore > 0 {
		conf = bestScore / totalScore
	}
	return best, conf
}

// ---------------------------------------------------------------------------
// Episode index
// ---------------------------------------------------------------------------

// Episode represents an episodic memory entry.
type Episode struct {
	ID           string
	Content      string
	Participants []string
	Timestamp    time.Time
	Tags         []string
	Outcome      string
}

// EpisodeIndexService indexes and searches episodes.
type EpisodeIndexService struct {
	mu          sync.RWMutex
	episodes    map[string]*Episode
	byWorkspace map[string][]string
}

// NewEpisodeIndexService creates an EpisodeIndexService.
func NewEpisodeIndexService() *EpisodeIndexService {
	return &EpisodeIndexService{
		episodes:    make(map[string]*Episode),
		byWorkspace: make(map[string][]string),
	}
}

// IndexEpisode stores an episode.
func (e *EpisodeIndexService) IndexEpisode(workspaceID string, episode Episode) error {
	if workspaceID == "" {
		return errors.New("workspaceID is required")
	}
	if episode.Content == "" {
		return errors.New("episode content is required")
	}
	if episode.ID == "" {
		episode.ID = generateID()
	}
	if episode.Timestamp.IsZero() {
		episode.Timestamp = time.Now().UTC()
	}

	e.mu.Lock()
	e.episodes[episode.ID] = &episode
	e.byWorkspace[workspaceID] = append(e.byWorkspace[workspaceID], episode.ID)
	e.mu.Unlock()
	return nil
}

// SearchEpisodes returns episodes matching the query by keyword.
func (e *EpisodeIndexService) SearchEpisodes(workspaceID, query string, limit int) []Episode {
	if limit <= 0 {
		limit = 10
	}
	e.mu.RLock()
	defer e.mu.RUnlock()

	queryLower := strings.ToLower(query)
	queryWords := strings.Fields(queryLower)

	ids := e.byWorkspace[workspaceID]
	type scored struct {
		ep    *Episode
		score int
	}
	var results []scored

	for _, id := range ids {
		ep := e.episodes[id]
		contentLower := strings.ToLower(ep.Content)
		s := 0
		for _, w := range queryWords {
			if strings.Contains(contentLower, w) {
				s++
			}
		}
		// Also check tags.
		for _, tag := range ep.Tags {
			for _, w := range queryWords {
				if strings.Contains(strings.ToLower(tag), w) {
					s++
				}
			}
		}
		if s > 0 {
			results = append(results, scored{ep, s})
		}
	}

	sort.Slice(results, func(i, j int) bool { return results[i].score > results[j].score })
	if len(results) > limit {
		results = results[:limit]
	}

	out := make([]Episode, len(results))
	for i, r := range results {
		out[i] = *r.ep
	}
	return out
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// simpleEmbed creates a simple term-frequency vector from text.  This is a
// lightweight placeholder for a real embedding model.
func simpleEmbed(text string) []float32 {
	words := strings.Fields(strings.ToLower(text))
	freq := make(map[string]int)
	for _, w := range words {
		freq[w]++
	}
	// Create a fixed-size vector by hashing words into 64 buckets.
	vec := make([]float32, 64)
	for w, count := range freq {
		bucket := 0
		for _, ch := range w {
			bucket = (bucket*31 + int(ch)) % 64
		}
		if bucket < 0 {
			bucket = -bucket
		}
		vec[bucket] += float32(count)
	}
	// Normalize.
	var norm float32
	for _, v := range vec {
		norm += v * v
	}
	if norm > 0 {
		norm = float32(math.Sqrt(float64(norm)))
		for i := range vec {
			vec[i] /= norm
		}
	}
	return vec
}

func cosineSimilarity(a, b []float32) float64 {
	if len(a) != len(b) {
		return 0
	}
	var dot, normA, normB float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
		normA += float64(a[i]) * float64(a[i])
		normB += float64(b[i]) * float64(b[i])
	}
	denom := math.Sqrt(normA) * math.Sqrt(normB)
	if denom == 0 {
		return 0
	}
	return dot / denom
}

func firstSentence(s string) string {
	for i, ch := range s {
		if ch == '.' || ch == '!' || ch == '?' {
			return s[:i+1]
		}
	}
	if len(s) > 120 {
		return s[:120]
	}
	return s
}

func dedupStrings(ss []string) []string {
	seen := make(map[string]bool, len(ss))
	var out []string
	for _, s := range ss {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}
