package rag

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"
)

// Reranker reorders a list of candidate results by scoring (query, chunk) pairs jointly.
type Reranker interface {
	Rerank(ctx context.Context, query string, candidates []RetrievalResult, limit int) ([]RetrievalResult, error)
}

// CohereReranker calls the Cohere /v1/rerank endpoint.
type CohereReranker struct {
	apiKey     string
	model      string
	httpClient *http.Client
	fallback   Reranker
	logger     Logger
}

func NewCohereReranker(apiKey, model string, fallback Reranker, logger Logger) *CohereReranker {
	if model == "" {
		model = "rerank-english-v3.0"
	}
	return &CohereReranker{
		apiKey:     apiKey,
		model:      model,
		httpClient: &http.Client{Timeout: 10 * time.Second},
		fallback:   fallback,
		logger:     logger,
	}
}

type cohereRerankRequest struct {
	Model     string   `json:"model"`
	Query     string   `json:"query"`
	Documents []string `json:"documents"`
	TopN      int      `json:"top_n,omitempty"`
}

type cohereRerankResponse struct {
	Results []struct {
		Index          int     `json:"index"`
		RelevanceScore float64 `json:"relevance_score"`
	} `json:"results"`
}

func (r *CohereReranker) Rerank(ctx context.Context, query string, candidates []RetrievalResult, limit int) ([]RetrievalResult, error) {
	if len(candidates) == 0 {
		return candidates, nil
	}
	if limit <= 0 || limit > len(candidates) {
		limit = len(candidates)
	}

	docs := make([]string, len(candidates))
	for i, c := range candidates {
		docs[i] = c.Snippet
	}

	reqBody := cohereRerankRequest{
		Model:     r.model,
		Query:     query,
		Documents: docs,
		TopN:      limit,
	}
	bodyBytes, _ := json.Marshal(reqBody)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://api.cohere.com/v1/rerank",
		strings.NewReader(string(bodyBytes)))
	if err != nil {
		return r.fallbackRerank(ctx, query, candidates, limit)
	}
	httpReq.Header.Set("Authorization", "Bearer "+r.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := r.httpClient.Do(httpReq)
	if err != nil {
		r.logger.Warn("cohere_rerank: HTTP request failed; using fallback", "error", err)
		return r.fallbackRerank(ctx, query, candidates, limit)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		r.logger.Warn("cohere_rerank: non-200 response; using fallback", "status", resp.StatusCode)
		return r.fallbackRerank(ctx, query, candidates, limit)
	}

	var rerankResp cohereRerankResponse
	if err := json.NewDecoder(resp.Body).Decode(&rerankResp); err != nil {
		r.logger.Warn("cohere_rerank: decode failed; using fallback", "error", err)
		return r.fallbackRerank(ctx, query, candidates, limit)
	}

	result := make([]RetrievalResult, 0, len(rerankResp.Results))
	for _, res := range rerankResp.Results {
		if res.Index < len(candidates) {
			c := candidates[res.Index]
			c.Score = res.RelevanceScore
			result = append(result, c)
		}
	}

	return result, nil
}

func (r *CohereReranker) fallbackRerank(ctx context.Context, query string, candidates []RetrievalResult, limit int) ([]RetrievalResult, error) {
	if r.fallback != nil {
		return r.fallback.Rerank(ctx, query, candidates, limit)
	}
	if limit < len(candidates) {
		return candidates[:limit], nil
	}
	return candidates, nil
}

// LLMReranker uses Claude Haiku to score (query, chunk) relevance.
type LLMReranker struct {
	llm      RerankLLMClient
	fallback Reranker
	logger   Logger
}

// RerankLLMClient is the interface for reranking LLM calls.
type RerankLLMClient interface {
	Complete(ctx context.Context, system, user string) (string, error)
}

func NewLLMReranker(llm RerankLLMClient, fallback Reranker, logger Logger) *LLMReranker {
	return &LLMReranker{llm: llm, fallback: fallback, logger: logger}
}

const rerankSystemPrompt = `You are a relevance scoring assistant.
Rate the relevance of a document chunk to a search query on a scale of 0 to 10.
Output ONLY a single integer (0-10). No explanation, no punctuation.
10 = perfectly relevant, directly answers the query.
5  = partially relevant, mentions related topics.
0  = completely irrelevant.`

func (r *LLMReranker) Rerank(ctx context.Context, query string, candidates []RetrievalResult, limit int) ([]RetrievalResult, error) {
	if len(candidates) == 0 {
		return candidates, nil
	}

	scoringSet := candidates
	if len(scoringSet) > 10 {
		scoringSet = scoringSet[:10]
	}

	type scoredResult struct {
		result RetrievalResult
		score  float64
	}
	scored := make([]scoredResult, len(scoringSet))

	for i, c := range scoringSet {
		snippet := c.Snippet
		if len(snippet) > 500 {
			snippet = snippet[:500] + "..."
		}
		userPrompt := fmt.Sprintf("Query: %s\n\nDocument chunk:\n%s\n\nRelevance score (0-10):",
			query, snippet)

		raw, err := r.llm.Complete(ctx, rerankSystemPrompt, userPrompt)
		if err != nil {
			r.logger.Warn("llm_rerank: LLM call failed; using original score", "error", err)
			scored[i] = scoredResult{result: c, score: c.Score}
			continue
		}

		raw = strings.TrimSpace(raw)
		score := parseRerankScore(raw)
		scored[i] = scoredResult{result: c, score: score}
	}

	for _, c := range candidates[len(scoringSet):] {
		scored = append(scored, scoredResult{result: c, score: c.Score * 10})
	}

	sort.Slice(scored, func(i, j int) bool {
		return scored[i].score > scored[j].score
	})

	if limit <= 0 || limit > len(scored) {
		limit = len(scored)
	}
	result := make([]RetrievalResult, limit)
	for i := 0; i < limit; i++ {
		r := scored[i].result
		r.Score = scored[i].score / 10.0
		result[i] = r
	}
	return result, nil
}

func parseRerankScore(s string) float64 {
	var score float64
	if _, err := fmt.Sscanf(s, "%f", &score); err != nil {
		return 5.0
	}
	if score < 0 {
		score = 0
	}
	if score > 10 {
		score = 10
	}
	return score
}

// PassthroughReranker returns candidates unchanged.
type PassthroughReranker struct{}

func (r *PassthroughReranker) Rerank(_ context.Context, _ string, candidates []RetrievalResult, limit int) ([]RetrievalResult, error) {
	if limit > 0 && limit < len(candidates) {
		return candidates[:limit], nil
	}
	return candidates, nil
}
