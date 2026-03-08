package brain

import (
	"strings"
	"sync"
)

// IntentResult represents a single classified intent.
type IntentResult struct {
	Intent         string
	Confidence     float64
	Skills         []string
	IsPrimary      bool
	DependsOnIndex *int
}

// MultiIntentOutput represents the output of multi-intent classification.
type MultiIntentOutput struct {
	Intents               []IntentResult
	CompoundRequest       bool
	OverallConfidence     float64
	RequiresDecomposition bool
}

// MultiIntentClassifier classifies compound requests into multiple intents.
type MultiIntentClassifier struct {
	mu sync.Mutex
}

// NewMultiIntentClassifier creates a new MultiIntentClassifier.
func NewMultiIntentClassifier() *MultiIntentClassifier {
	return &MultiIntentClassifier{}
}

// conjunctions used to split compound requests.
var conjunctions = []string{" and ", " also ", " plus ", " then "}

// Classify splits compound requests into multiple intents.
func (mic *MultiIntentClassifier) Classify(input string) MultiIntentOutput {
	mic.mu.Lock()
	defer mic.mu.Unlock()

	input = strings.TrimSpace(input)
	if input == "" {
		return MultiIntentOutput{
			Intents:           nil,
			CompoundRequest:   false,
			OverallConfidence: 0,
		}
	}

	segments := splitByConjunctions(input)
	if len(segments) <= 1 {
		intent := classifySingleIntent(input)
		intent.IsPrimary = true
		return MultiIntentOutput{
			Intents:               []IntentResult{intent},
			CompoundRequest:       false,
			OverallConfidence:     intent.Confidence,
			RequiresDecomposition: false,
		}
	}

	intents := make([]IntentResult, 0, len(segments))
	totalConfidence := 0.0
	for i, segment := range segments {
		intent := classifySingleIntent(segment)
		intent.IsPrimary = i == 0
		if i > 0 {
			prev := i - 1
			intent.DependsOnIndex = &prev
		}
		intents = append(intents, intent)
		totalConfidence += intent.Confidence
	}

	overallConfidence := totalConfidence / float64(len(intents))

	return MultiIntentOutput{
		Intents:               intents,
		CompoundRequest:       true,
		OverallConfidence:     overallConfidence,
		RequiresDecomposition: len(intents) > 1,
	}
}

func splitByConjunctions(input string) []string {
	lower := strings.ToLower(input)
	segments := []string{input}

	for _, conj := range conjunctions {
		newSegments := []string{}
		for _, seg := range segments {
			lowerSeg := strings.ToLower(seg)
			idx := strings.Index(lowerSeg, conj)
			if idx >= 0 {
				left := strings.TrimSpace(seg[:idx])
				right := strings.TrimSpace(seg[idx+len(conj):])
				if left != "" {
					newSegments = append(newSegments, left)
				}
				if right != "" {
					newSegments = append(newSegments, right)
				}
			} else {
				newSegments = append(newSegments, seg)
			}
		}
		segments = newSegments
	}

	_ = lower
	return segments
}

// classifySingleIntent classifies a single intent segment using keyword heuristics.
func classifySingleIntent(segment string) IntentResult {
	lower := strings.ToLower(segment)

	type intentDef struct {
		keywords   []string
		intent     string
		skills     []string
		confidence float64
	}

	definitions := []intentDef{
		{keywords: []string{"send", "email", "mail"}, intent: "send_email", skills: []string{"email"}, confidence: 0.85},
		{keywords: []string{"schedule", "meeting", "calendar", "event"}, intent: "schedule_event", skills: []string{"calendar"}, confidence: 0.85},
		{keywords: []string{"search", "find", "look up", "lookup"}, intent: "search", skills: []string{"search"}, confidence: 0.80},
		{keywords: []string{"create", "write", "draft"}, intent: "create_content", skills: []string{"content"}, confidence: 0.80},
		{keywords: []string{"summarize", "summary"}, intent: "summarize", skills: []string{"analysis"}, confidence: 0.82},
		{keywords: []string{"remind", "reminder"}, intent: "set_reminder", skills: []string{"reminders"}, confidence: 0.85},
	}

	for _, def := range definitions {
		for _, kw := range def.keywords {
			if strings.Contains(lower, kw) {
				return IntentResult{
					Intent:     def.intent,
					Confidence: def.confidence,
					Skills:     def.skills,
				}
			}
		}
	}

	return IntentResult{
		Intent:     "general_query",
		Confidence: 0.6,
		Skills:     []string{"general"},
	}
}
