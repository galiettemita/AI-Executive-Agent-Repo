package brain

import (
	"strings"
	"sync"

	"github.com/google/uuid"
)

// KeywordRule defines a keyword-based classification rule.
type KeywordRule struct {
	ID         uuid.UUID
	Keywords   []string
	Intent     string
	Confidence float64
}

// KeywordClassifier provides fallback intent classification using keyword matching.
type KeywordClassifier struct {
	mu    sync.Mutex
	rules []KeywordRule
}

// NewKeywordClassifier creates a new KeywordClassifier.
func NewKeywordClassifier() *KeywordClassifier {
	return &KeywordClassifier{
		rules: []KeywordRule{},
	}
}

// AddRule adds a keyword classification rule.
func (kc *KeywordClassifier) AddRule(keywords []string, intent string, confidence float64) KeywordRule {
	kc.mu.Lock()
	defer kc.mu.Unlock()

	if confidence <= 0 {
		confidence = 0.5
	}
	if confidence > 1 {
		confidence = 1.0
	}

	lowerKeywords := make([]string, 0, len(keywords))
	for _, kw := range keywords {
		clean := strings.ToLower(strings.TrimSpace(kw))
		if clean != "" {
			lowerKeywords = append(lowerKeywords, clean)
		}
	}

	rule := KeywordRule{
		ID:         uuid.Must(uuid.NewV7()),
		Keywords:   lowerKeywords,
		Intent:     strings.TrimSpace(intent),
		Confidence: confidence,
	}
	kc.rules = append(kc.rules, rule)
	return rule
}

// Classify attempts to classify input using keyword rules.
// Returns the best matching intent and confidence.
func (kc *KeywordClassifier) Classify(input string) (string, float64) {
	kc.mu.Lock()
	defer kc.mu.Unlock()

	if strings.TrimSpace(input) == "" {
		return "unknown", 0
	}

	bestIntent := "unknown"
	bestConfidence := 0.0
	bestMatchCount := 0

	for _, rule := range kc.rules {
		matchCount := 0
		for _, kw := range rule.Keywords {
			if Match(input, []string{kw}) {
				matchCount++
			}
		}
		if matchCount > 0 {
			scaledConfidence := rule.Confidence * (float64(matchCount) / float64(len(rule.Keywords)))
			if scaledConfidence > bestConfidence || (scaledConfidence == bestConfidence && matchCount > bestMatchCount) {
				bestIntent = rule.Intent
				bestConfidence = scaledConfidence
				bestMatchCount = matchCount
			}
		}
	}

	return bestIntent, bestConfidence
}

// Match checks if the input contains any of the given keywords.
func Match(input string, keywords []string) bool {
	lower := strings.ToLower(input)
	for _, kw := range keywords {
		if strings.Contains(lower, strings.ToLower(kw)) {
			return true
		}
	}
	return false
}
