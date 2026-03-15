package rag

import (
	"math"
	"strings"
	"sync"
	"unicode"
)

// BM25Index maintains Okapi BM25 corpus statistics.
// Formula: score(q,d) = Σ IDF(t) · TF(t,d)·(k1+1) / (TF(t,d) + k1·(1-b+b·|d|/avgDL))
// Parameters: k1=1.5 (TF saturation), b=0.75 (length normalization)
type BM25Index struct {
	mu        sync.RWMutex
	docFreq   map[string]int // df(t): number of documents containing term t
	docLens   map[string]int // |d| in tokens, keyed by chunk ID
	totalDocs int
	totalLen  int // sum of all document lengths (for avgDL)
	k1        float64
	b         float64
}

func NewBM25Index() *BM25Index {
	return &BM25Index{
		docFreq: make(map[string]int),
		docLens: make(map[string]int),
		k1:      1.5,
		b:       0.75,
	}
}

// IndexDocument registers a document for IDF tracking.
func (idx *BM25Index) IndexDocument(chunkID string, tokens []string) {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	if _, exists := idx.docLens[chunkID]; exists {
		return
	}

	seen := make(map[string]bool, len(tokens))
	for _, tok := range tokens {
		t := strings.ToLower(tok)
		if t == "" {
			continue
		}
		if !seen[t] {
			idx.docFreq[t]++
			seen[t] = true
		}
	}

	idx.docLens[chunkID] = len(tokens)
	idx.totalLen += len(tokens)
	idx.totalDocs++
}

// Score returns the BM25 score for a document against a query.
func (idx *BM25Index) Score(docTokens, queryTokens []string) float64 {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	if idx.totalDocs == 0 {
		return 0
	}

	avgDL := float64(idx.totalLen) / float64(idx.totalDocs)
	docLen := float64(len(docTokens))

	tf := make(map[string]int, len(docTokens))
	for _, tok := range docTokens {
		tf[strings.ToLower(tok)]++
	}

	var score float64
	seen := make(map[string]bool)

	for _, qt := range queryTokens {
		term := strings.ToLower(qt)
		if term == "" || seen[term] {
			continue
		}
		seen[term] = true

		termTF := float64(tf[term])
		if termTF == 0 {
			continue
		}

		df := float64(idx.docFreq[term])
		n := float64(idx.totalDocs)

		idf := math.Log((n-df+0.5)/(df+0.5) + 1)
		tfComp := termTF * (idx.k1 + 1) /
			(termTF + idx.k1*(1-idx.b+idx.b*docLen/avgDL))

		score += idf * tfComp
	}

	return score
}

// Stats returns corpus statistics for observability and testing.
func (idx *BM25Index) Stats() (docs int, vocabSize int, avgDocLen float64) {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	if idx.totalDocs > 0 {
		avgDocLen = float64(idx.totalLen) / float64(idx.totalDocs)
	}
	return idx.totalDocs, len(idx.docFreq), avgDocLen
}

// BM25Tokenize converts text to lowercase tokens, strips punctuation, removes stopwords.
func BM25Tokenize(text string) []string {
	var tokens []string
	var cur strings.Builder

	for _, r := range strings.ToLower(text) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' || r == '_' {
			cur.WriteRune(r)
		} else {
			if tok := strings.Trim(cur.String(), "-_"); len(tok) > 1 && !bm25Stopword(tok) {
				tokens = append(tokens, tok)
			}
			cur.Reset()
		}
	}
	if tok := strings.Trim(cur.String(), "-_"); len(tok) > 1 && !bm25Stopword(tok) {
		tokens = append(tokens, tok)
	}
	return tokens
}

func bm25Stopword(t string) bool {
	switch t {
	case "a", "an", "the", "and", "or", "but", "in", "on", "at", "to",
		"for", "of", "with", "is", "are", "was", "were", "be", "been",
		"by", "from", "as", "it", "its", "this", "that", "will", "can",
		"do", "did", "not", "have", "has", "had", "so", "if", "we", "you":
		return true
	}
	return false
}
