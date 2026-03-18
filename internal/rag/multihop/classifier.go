package multihop

import "strings"

// MultiHopClassifier determines whether a query should use multi-hop retrieval.
type MultiHopClassifier struct{}

// NewMultiHopClassifier creates a new classifier.
func NewMultiHopClassifier() *MultiHopClassifier { return &MultiHopClassifier{} }

// ShouldUseMultiHop returns true if the query is complex enough to benefit
// from multi-hop retrieval: word_count > 30 OR entity_count > 4.
func (c *MultiHopClassifier) ShouldUseMultiHop(query string) bool {
	words := strings.Fields(query)
	if len(words) > 30 {
		return true
	}
	return c.estimateEntityCount(query) > 4
}

func (c *MultiHopClassifier) estimateEntityCount(query string) int {
	count := 0
	words := strings.Fields(query)
	for i, w := range words {
		if i == 0 {
			continue // skip first word (often capitalized anyway)
		}
		cleaned := strings.Trim(w, `.,;:!?"'`)
		if len(cleaned) > 1 && cleaned[0] >= 'A' && cleaned[0] <= 'Z' {
			count++
		}
	}
	return count
}
